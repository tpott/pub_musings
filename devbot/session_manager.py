# session_manager.py

import fcntl
import json
import time
import uuid
from pathlib import Path
from typing import Any, Dict, List, Optional

from serve_config import get_data_dir, get_default_host


class SessionManager:
    """Manages session state and message history persistence."""

    def __init__(self, config: Dict[str, Any]):
        self.config = config
        self.data_dir = get_data_dir()
        self.sessions_file = self.data_dir / "sessions.json"
        self.history_dir = self.data_dir / "history"
        self.archive_dir = self.history_dir / "archive"

        # Ensure directories exist
        self.history_dir.mkdir(parents=True, exist_ok=True)
        self.archive_dir.mkdir(parents=True, exist_ok=True)

        # Load or initialize sessions
        self._sessions_data = self._load_sessions()

    def _load_sessions(self) -> Dict[str, Any]:
        """Load sessions from file with file locking."""
        if not self.sessions_file.exists():
            return {"sessions": {}, "current_session": "#default"}

        with open(self.sessions_file, "r") as f:
            fcntl.flock(f.fileno(), fcntl.LOCK_SH)
            try:
                return json.load(f)
            finally:
                fcntl.flock(f.fileno(), fcntl.LOCK_UN)

    def _save_sessions(self) -> None:
        """Save sessions to file with file locking."""
        with open(self.sessions_file, "w") as f:
            fcntl.flock(f.fileno(), fcntl.LOCK_EX)
            try:
                json.dump(self._sessions_data, f, indent=2)
            finally:
                fcntl.flock(f.fileno(), fcntl.LOCK_UN)

    def _get_history_file(self, session_name: str) -> Path:
        """Get the JSONL history file path for a session."""
        # Remove leading # if present
        name = session_name.lstrip("#")
        return self.history_dir / f"{name}.jsonl"

    def create_session(
        self, name: str, hostname: Optional[str] = None
    ) -> Dict[str, Any]:
        """Create a new session."""
        if not name.startswith("#"):
            name = f"#{name}"

        if hostname is None:
            hostname = get_default_host(self.config)

        session_id = str(uuid.uuid4())
        now = time.time()

        session = {
            "hostname": hostname,
            "last_message_id": None,
            "claude_session_id": None,  # Claude CLI session ID for --resume
            "session_id": session_id,  # Local UUID (not Claude CLI ID)
            "created_at": now,
            "updated_at": now,
            "message_count": 0,
        }

        self._sessions_data["sessions"][name] = session
        self._sessions_data["current_session"] = name
        self._save_sessions()

        return session

    def switch_session(self, name: str) -> Optional[Dict[str, Any]]:
        """Switch to an existing session."""
        if not name.startswith("#"):
            name = f"#{name}"

        if name not in self._sessions_data["sessions"]:
            return None

        self._sessions_data["current_session"] = name
        self._save_sessions()
        return self._sessions_data["sessions"][name]

    def get_current_session(self) -> Dict[str, Any]:
        """Get the current session, creating #default if needed."""
        current_name = self._sessions_data.get("current_session", "#default")

        if current_name not in self._sessions_data["sessions"]:
            return self.create_session("default")

        return self._sessions_data["sessions"][current_name]

    def get_current_session_name(self) -> str:
        """Get the current session name."""
        return self._sessions_data.get("current_session", "#default")

    def update_session(self, name: str, **kwargs) -> None:
        """Update session fields."""
        if not name.startswith("#"):
            name = f"#{name}"

        if name in self._sessions_data["sessions"]:
            self._sessions_data["sessions"][name].update(kwargs)
            self._sessions_data["sessions"][name]["updated_at"] = time.time()
            self._save_sessions()

    def add_message(
        self,
        role: str,
        content: str,
        proxied: bool = False,
        remote_message_id: Optional[str] = None,
        compacted_count: Optional[int] = None,
    ) -> None:
        """Add a message to the current session's history."""
        session_name = self.get_current_session_name()
        history_file = self._get_history_file(session_name)

        message: Dict[str, Any] = {
            "role": role,
            "content": content,
            "timestamp": time.time(),
        }

        if proxied:
            message["proxied"] = True
        if remote_message_id:
            message["remote_message_id"] = remote_message_id
        if compacted_count:
            message["compacted_count"] = compacted_count

        with open(history_file, "a") as f:
            f.write(json.dumps(message) + "\n")

        # Update session message count
        session = self._sessions_data["sessions"].get(session_name, {})
        session["message_count"] = session.get("message_count", 0) + 1
        if remote_message_id:
            session["last_message_id"] = remote_message_id
        self._sessions_data["sessions"][session_name] = session
        self._save_sessions()

    def list_sessions(self) -> Dict[str, Dict[str, Any]]:
        """List all sessions with their metadata."""
        return self._sessions_data["sessions"]

    def get_session_info(self, name: Optional[str] = None) -> Optional[Dict[str, Any]]:
        """Get detailed info for a session."""
        if name is None:
            name = self.get_current_session_name()
        elif not name.startswith("#"):
            name = f"#{name}"

        session = self._sessions_data["sessions"].get(name)
        if session is None:
            return None

        # Add additional computed fields
        info = session.copy()
        info["name"] = name

        # Get actual message count from file
        history_file = self._get_history_file(name)
        if history_file.exists():
            with open(history_file, "r") as f:
                info["actual_message_count"] = sum(1 for _ in f)

        return info

    def get_recent_messages(
        self, n: int = 10, session_name: Optional[str] = None
    ) -> List[Dict[str, Any]]:
        """Get the last N messages from a session's history."""
        if session_name is None:
            session_name = self.get_current_session_name()

        history_file = self._get_history_file(session_name)
        if not history_file.exists():
            return []

        messages = []
        with open(history_file, "r") as f:
            for line in f:
                if line.strip():
                    messages.append(json.loads(line))

        return messages[-n:]

    def get_all_messages(
        self, session_name: Optional[str] = None
    ) -> List[Dict[str, Any]]:
        """Get all messages from a session's history."""
        if session_name is None:
            session_name = self.get_current_session_name()

        history_file = self._get_history_file(session_name)
        if not history_file.exists():
            return []

        messages = []
        with open(history_file, "r") as f:
            for line in f:
                if line.strip():
                    messages.append(json.loads(line))

        return messages

    def compact_messages(
        self, n: int, summary: str, session_name: Optional[str] = None
    ) -> int:
        """
        Compact the last N messages into a summary.
        Archives the original messages and replaces them with a summary.
        Returns the number of messages compacted.
        """
        if session_name is None:
            session_name = self.get_current_session_name()

        history_file = self._get_history_file(session_name)
        if not history_file.exists():
            return 0

        # Read all messages
        messages = []
        with open(history_file, "r") as f:
            for line in f:
                if line.strip():
                    messages.append(json.loads(line))

        if len(messages) < n:
            n = len(messages)

        if n == 0:
            return 0

        # Messages to archive (last n)
        to_archive = messages[-n:]
        to_keep = messages[:-n]

        # Archive the messages
        archive_name = session_name.lstrip("#")
        archive_file = self.archive_dir / f"{archive_name}_{int(time.time())}.jsonl"
        with open(archive_file, "w") as f:
            for msg in to_archive:
                f.write(json.dumps(msg) + "\n")

        # Create summary message
        summary_message = {
            "role": "system",
            "content": f"[Compacted context summary]: {summary}",
            "timestamp": time.time(),
            "compacted_count": n,
        }

        # Write back: kept messages + summary
        with open(history_file, "w") as f:
            for msg in to_keep:
                f.write(json.dumps(msg) + "\n")
            f.write(json.dumps(summary_message) + "\n")

        # Update session
        session = self._sessions_data["sessions"].get(session_name, {})
        session["message_count"] = len(to_keep) + 1
        self._sessions_data["sessions"][session_name] = session
        self._save_sessions()

        return n
