#!/usr/bin/env python3
"""matrix_loop.py - Matrix bot that routes messages through DevAgent.

This bot:
- Uses config from serve_config with stored access tokens
- Listens to messages and routes them through DevAgent (like loop.py)
- Supports E2E encryption
- One session per Matrix room (shared conversation context)

Prerequisites:
- Run matrix_login.py first to set up credentials and verify device
- Add matrix config to data/config.json:
  {
    "matrix": {
      "allowed_users": ["@user1:matrix.org", "@user2:example.com"],
      "max_room_size": 5
    }
  }

Usage:
    python matrix_loop.py
"""

import asyncio
import json
import sys
import traceback
from typing import Any, Dict, Optional

from nio import (
    AsyncClient,
    AsyncClientConfig,
    MatrixRoom,
    RoomMessageText,
)

from dev_agent import DevAgent
from serve_config import (
    get_config,
    get_matrix_config,
    get_matrix_credentials_path,
    get_matrix_room_sessions_path,
    get_matrix_store_path,
)
from session_manager import SessionManager


class MatrixBot:
    """Matrix bot that routes messages through DevAgent."""

    def __init__(self, config: Dict[str, Any]):
        self.config = config
        self.matrix_config = get_matrix_config(config)
        self.client: Optional[AsyncClient] = None

        # Session management
        self.session_manager = SessionManager(config)
        self.room_sessions: Dict[str, str] = {}  # room_id -> session_name
        self._load_room_sessions()

        # DevAgent instance (shared, uses session_manager for context)
        self.dev_agent = DevAgent(config)

    def _load_room_sessions(self) -> None:
        """Load room-to-session mapping from disk."""
        path = get_matrix_room_sessions_path()
        if path.exists():
            with open(path) as f:
                self.room_sessions = json.load(f)

    def _save_room_sessions(self) -> None:
        """Save room-to-session mapping to disk."""
        path = get_matrix_room_sessions_path()
        with open(path, "w") as f:
            json.dump(self.room_sessions, f, indent=2)

    def _get_or_create_room_session(self, room_id: str) -> str:
        """Get or create a session name for a Matrix room."""
        if room_id in self.room_sessions:
            session_name = self.room_sessions[room_id]
        else:
            # Create session name from room_id (use last part, sanitized)
            room_short = room_id.split(":")[0].lstrip("!").replace(".", "_")[:16]
            session_name = f"#matrix-{room_short}"
            self.room_sessions[room_id] = session_name
            self._save_room_sessions()

        # Ensure session exists in session_manager
        sessions = self.session_manager.list_sessions()
        if session_name not in sessions:
            self.session_manager.create_session(session_name.lstrip("#"))

        return session_name

    async def start(self) -> None:
        """Start the Matrix bot."""
        creds_path = get_matrix_credentials_path()
        store_path = get_matrix_store_path()

        if not creds_path.exists():
            print("Error: No Matrix credentials found.")
            print("Please run matrix_login.py first to set up credentials.")
            sys.exit(1)

        with open(creds_path) as f:
            creds = json.load(f)

        client_config = AsyncClientConfig(
            max_limit_exceeded=0,
            max_timeouts=0,
            store_sync_tokens=True,
            encryption_enabled=True,
        )

        self.client = AsyncClient(
            creds["homeserver"],
            creds["user_id"],
            device_id=creds["device_id"],
            store_path=str(store_path),
            config=client_config,
        )

        self.client.restore_login(
            user_id=creds["user_id"],
            device_id=creds["device_id"],
            access_token=creds["access_token"],
        )

        # Register message callback
        self.client.add_event_callback(self._message_callback, RoomMessageText)

        print(f"Matrix bot started as {creds['user_id']}")
        print(f"Allowed users: {self.matrix_config.get('allowed_users', ['(none - all allowed)'])}")
        print(f"Max room size: {self.matrix_config.get('max_room_size', '(unlimited)')}")
        print("Listening for messages... (Ctrl-C to stop)")

        # Sync forever
        await self.client.sync_forever(timeout=30000, full_state=True)

    async def _ensure_room_devices_trusted(self, room_id: str) -> None:
        """Trust devices in a room, preferring cross-signing verification."""
        room = self.client.rooms.get(room_id)
        if not room:
            return

        for user_id in room.users:
            for device_id, device in self.client.device_store.active_user_devices(user_id):
                if self.client.olm.is_device_verified(device):
                    continue  # Already verified

                # Check if device is cross-signed by a trusted user
                if hasattr(self.client.olm, 'is_device_cross_signed') and \
                   self.client.olm.is_device_cross_signed(device):
                    # Device is signed by user's self-signing key
                    self.client.verify_device(device)
                    print(f"Auto-trusted cross-signed device {device_id} for {user_id}")
                else:
                    # Fall back to trusting unverified devices (TOFU for bots)
                    self.client.verify_device(device)
                    print(f"TOFU-trusted device {device_id} for {user_id}")

    async def _message_callback(
        self, room: MatrixRoom, event: RoomMessageText
    ) -> None:
        """Handle incoming room messages."""
        # Skip own messages
        if event.sender == self.client.user_id:
            return

        # Authorization check
        allowed_users = self.matrix_config.get("allowed_users", [])
        if allowed_users and event.sender not in allowed_users:
            print(f"Ignoring message from unauthorized user: {event.sender}")
            return

        # Room size check
        max_room_size = self.matrix_config.get("max_room_size", 0)
        if max_room_size > 0 and len(room.users) > max_room_size:
            print(
                f"Ignoring message from room {room.display_name}: "
                f"{len(room.users)} users exceeds max {max_room_size}"
            )
            return

        print(
            f"\n[{room.display_name}] {room.user_name(event.sender)}: {event.body}"
        )

        # Get or create session for this room
        session_name = self._get_or_create_room_session(room.room_id)
        self.session_manager.switch_session(session_name)

        # Also update DevAgent's session_manager reference to use same session
        self.dev_agent.session_manager = self.session_manager

        try:
            # Process through DevAgent
            response = await self.dev_agent.process_input(event.body)

            # Trust all devices in room before sending encrypted message
            await self._ensure_room_devices_trusted(room.room_id)

            # Send response back to room
            await self.client.room_send(
                room.room_id,
                message_type="m.room.message",
                content={"msgtype": "m.text", "body": response},
            )
            print(f"[{room.display_name}] Bot: {response[:200]}{'...' if len(response) > 200 else ''}")

        except Exception as e:
            error_msg = f"Error processing message: {e}"
            print(error_msg)
            traceback.print_exc()

            # Optionally send error to room
            await self.client.room_send(
                room.room_id,
                message_type="m.room.message",
                content={"msgtype": "m.text", "body": f"Sorry, I encountered an error: {e}"},
            )


async def main() -> None:
    """Main entry point."""
    config = get_config()
    bot = MatrixBot(config)
    await bot.start()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nShutting down Matrix bot.")
        sys.exit(0)
    except Exception:
        print(traceback.format_exc())
        sys.exit(1)
