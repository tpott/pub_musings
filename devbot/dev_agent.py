# dev_agent.py

import json
from typing import Any, Dict, List, Optional

from anthropic_client import anthropic_completion, anthropic_tool_result
from session_manager import SessionManager
from ssh_proxy import SSHProxy


SYSTEM_PROMPT = """You are a developer assistant managing remote Claude sessions.
By default, proxy all development messages to the current session using the proxy_message tool.
Use session tools when user explicitly mentions sessions, switching, hosts, or session management.

Available tools:
- proxy_message: Forward a message to the current session's remote Claude CLI
- get_sessions: List all active sessions with their stats
- switch_session: Switch to a different session by hashtag name
- new_session: Create a new session (optionally specify hostname)
- info: Get detailed info for current or specified session
- get_hosts: List configured remote hosts
- compact_session: Summarize the last N messages into a single context summary

When the user asks development questions, writes code, or needs coding assistance, always use proxy_message to forward to the remote Claude."""


TOOLS = [
    {
        "name": "proxy_message",
        "description": "Forward a message to the current session's remote Claude CLI. Use this for all development questions, coding tasks, and technical queries.",
        "input_schema": {
            "type": "object",
            "properties": {
                "message": {
                    "type": "string",
                    "description": "The message to send to the remote Claude CLI",
                }
            },
            "required": ["message"],
        },
    },
    {
        "name": "get_sessions",
        "description": "List all active sessions with their metadata (hostname, message count, last updated).",
        "input_schema": {
            "type": "object",
            "properties": {},
        },
    },
    {
        "name": "switch_session",
        "description": "Switch to a different session by its hashtag name (e.g., '#refactor-auth').",
        "input_schema": {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "The session name/hashtag to switch to (with or without leading #)",
                }
            },
            "required": ["name"],
        },
    },
    {
        "name": "new_session",
        "description": "Create a new session. Optionally specify which remote host to use.",
        "input_schema": {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Name for the new session (will be prefixed with # if not already)",
                },
                "hostname": {
                    "type": "string",
                    "description": "Optional: which configured host to use for this session",
                },
            },
            "required": ["name"],
        },
    },
    {
        "name": "info",
        "description": "Get detailed information about a session including message count, hostname, and timestamps.",
        "input_schema": {
            "type": "object",
            "properties": {
                "session_name": {
                    "type": "string",
                    "description": "Optional: session name to get info for (defaults to current session)",
                }
            },
        },
    },
    {
        "name": "get_hosts",
        "description": "List all configured remote hosts that can be used for sessions.",
        "input_schema": {
            "type": "object",
            "properties": {},
        },
    },
    {
        "name": "compact_session",
        "description": "Compact the last N messages in the session history into a summary to reduce context size.",
        "input_schema": {
            "type": "object",
            "properties": {
                "n": {
                    "type": "integer",
                    "description": "Number of recent messages to compact into a summary",
                },
                "session_name": {
                    "type": "string",
                    "description": "Optional: session to compact (defaults to current session)",
                },
            },
            "required": ["n"],
        },
    },
]


class DevAgent:
    """Main agent class with tool use for managing remote Claude sessions."""

    def __init__(self, config: Dict[str, Any]):
        self.config = config
        self.session_manager = SessionManager(config)
        self.ssh_proxy = SSHProxy(config)
        self.context = {
            "verbose": config.get("verbose", 0),
            "model": config.get("model", "claude-sonnet-4-5-20250929"),
        }

    async def process_input(self, user_input: str) -> str:
        """
        Process user input through the agent.
        Returns the final response to display.
        """
        # Check for slash commands (bypass LLM)
        if user_input.startswith("/"):
            return await self._handle_slash_command(user_input)

        # Build messages for LLM
        messages = self._build_messages(user_input)

        # Agent loop with tool use
        max_iterations = 10
        for _ in range(max_iterations):
            result = anthropic_completion(
                messages=messages,
                context=self.context,
                tools=TOOLS,
                system_prompt=SYSTEM_PROMPT,
            )

            # Check if we got a tool call
            tool_use = result.get("tool_use")
            if not tool_use or result.get("stop_reason") != "tool_use":
                # No tool call, we have the final response
                return result["content"]

            # Execute the tool
            tool_result = await self._execute_tool(
                tool_use["name"], tool_use["input"]
            )

            if self.context["verbose"] > 0:
                print(f"Tool {tool_use}, result={tool_result}")

            # Add assistant message with tool_use to messages
            messages.append(
                {
                    "role": "assistant",
                    "content": [
                        {
                            "type": "tool_use",
                            "id": tool_use["id"],
                            "name": tool_use["name"],
                            "input": tool_use["input"],
                        }
                    ],
                }
            )

            # Add tool result
            messages.append(
                {
                    "role": "user",
                    "content": [
                        {
                            "type": "tool_result",
                            "tool_use_id": tool_use["id"],
                            "content": tool_result,
                        }
                    ],
                }
            )

            # Continue the loop to get next response
            continue


        return "Error: Agent loop exceeded maximum iterations"

    def _build_messages(self, user_input: str) -> List[Dict[str, Any]]:
        """Build the messages list for the API call."""
        # For now, just the current user message
        # Later could include recent history from session
        return [{"role": "user", "content": user_input}]

    async def _handle_slash_command(self, command: str) -> str:
        """Handle slash commands directly without LLM."""
        parts = command.strip().split(maxsplit=1)
        cmd = parts[0].lower()
        args = parts[1] if len(parts) > 1 else ""

        if cmd == "/sessions":
            sessions = self.session_manager.list_sessions()
            current = self.session_manager.get_current_session_name()
            if not sessions:
                return "No sessions found."
            lines = ["Sessions:"]
            for name, info in sessions.items():
                marker = " (current)" if name == current else ""
                hostname = info.get("hostname", "unknown")
                count = info.get("message_count", 0)
                lines.append(f"  {name}{marker} - {hostname} ({count} messages)")
            return "\n".join(lines)

        elif cmd == "/switch":
            if not args:
                return "Usage: /switch <session_name>"
            result = self.session_manager.switch_session(args)
            if result:
                return f"Switched to session #{args.lstrip('#')}"
            return f"Session '{args}' not found"

        elif cmd == "/new":
            if not args:
                return "Usage: /new <session_name> [hostname]"
            name_parts = args.split(maxsplit=1)
            name = name_parts[0]
            hostname = name_parts[1] if len(name_parts) > 1 else None
            self.session_manager.create_session(name, hostname)
            return f"Created and switched to session #{name.lstrip('#')}"

        elif cmd == "/hosts":
            hosts = self.ssh_proxy.list_hosts()
            if not hosts:
                return "No hosts configured."
            lines = ["Configured hosts:"]
            for name, info in hosts.items():
                lines.append(f"  {name}: {info['user']}@{info['hostname']}")
            return "\n".join(lines)

        elif cmd == "/info":
            session_name = args if args else None
            info = self.session_manager.get_session_info(session_name)
            if not info:
                return f"Session '{args}' not found" if args else "No current session"
            lines = [f"Session: {info['name']}"]
            lines.append(f"  Host: {info.get('hostname', 'unknown')}")
            lines.append(f"  Messages: {info.get('message_count', 0)}")
            lines.append(f"  Session ID: {info.get('session_id', 'N/A')}")
            lines.append(f"  Last message ID: {info.get('last_message_id', 'N/A')}")
            return "\n".join(lines)

        elif cmd in ["/help", "/?"]:
            return """Available commands:
  /sessions - List all sessions
  /switch <name> - Switch to a session
  /new <name> [host] - Create a new session
  /hosts - List configured hosts
  /info [session] - Show session details
  /compact <n> - Compact last n messages
  /help - Show this help"""

        elif cmd == "/compact":
            if not args:
                return "Usage: /compact <number_of_messages>"
            try:
                n = int(args)
            except ValueError:
                return "Error: argument must be a number"
            # Get messages to summarize
            messages = self.session_manager.get_recent_messages(n)
            if not messages:
                return "No messages to compact"
            # Generate summary using Claude
            summary = await self._generate_summary(messages)
            count = self.session_manager.compact_messages(n, summary)
            return f"Compacted {count} messages into summary"

        return f"Unknown command: {cmd}. Type /help for available commands."

    async def _execute_tool(self, tool_name: str, tool_input: Dict[str, Any]) -> str:
        """Execute a tool and return the result as a string."""
        if tool_name == "proxy_message":
            return await self._tool_proxy_message(tool_input)
        elif tool_name == "get_sessions":
            return self._tool_get_sessions()
        elif tool_name == "switch_session":
            return self._tool_switch_session(tool_input)
        elif tool_name == "new_session":
            return self._tool_new_session(tool_input)
        elif tool_name == "info":
            return self._tool_info(tool_input)
        elif tool_name == "get_hosts":
            return self._tool_get_hosts()
        elif tool_name == "compact_session":
            return await self._tool_compact_session(tool_input)
        else:
            return f"Unknown tool: {tool_name}"

    async def _tool_proxy_message(self, tool_input: Dict[str, Any]) -> str:
        """Forward message to remote Claude CLI."""
        message = tool_input.get("message", "")
        if not message:
            return "Error: No message provided"

        # Get current session info
        session = self.session_manager.get_current_session()
        hostname = session.get("hostname")

        if not hostname:
            return "Error: No hostname configured for current session"

        resume_id = session.get("last_message_id")

        # Execute SSH proxy
        result = await self.ssh_proxy.execute(
            message=message,
            hostname=hostname,
            resume_id=resume_id,
        )

        if not result.success:
            return f"Error: {result.error}"

        # Record messages in history
        session_name = self.session_manager.get_current_session_name()
        self.session_manager.add_message("user", message, proxied=True)
        self.session_manager.add_message(
            "assistant", result.content, remote_message_id=result.message_id
        )

        # Update session with new message_id for resume
        if result.message_id:
            self.session_manager.update_session(
                session_name, last_message_id=result.message_id
            )

        return result.content

    def _tool_get_sessions(self) -> str:
        """List all sessions."""
        sessions = self.session_manager.list_sessions()
        current = self.session_manager.get_current_session_name()
        if not sessions:
            return json.dumps({"sessions": [], "current": None})
        return json.dumps(
            {
                "sessions": [
                    {
                        "name": name,
                        "hostname": info.get("hostname"),
                        "message_count": info.get("message_count", 0),
                        "is_current": name == current,
                    }
                    for name, info in sessions.items()
                ],
                "current": current,
            }
        )

    def _tool_switch_session(self, tool_input: Dict[str, Any]) -> str:
        """Switch to a different session."""
        name = tool_input.get("name", "")
        if not name:
            return "Error: No session name provided"
        result = self.session_manager.switch_session(name)
        if result:
            return f"Successfully switched to session #{name.lstrip('#')}"
        return f"Session '{name}' not found. Use new_session to create it."

    def _tool_new_session(self, tool_input: Dict[str, Any]) -> str:
        """Create a new session."""
        name = tool_input.get("name", "")
        hostname = tool_input.get("hostname")
        if not name:
            return "Error: No session name provided"
        self.session_manager.create_session(name, hostname)
        return f"Created and switched to new session #{name.lstrip('#')}"

    def _tool_info(self, tool_input: Dict[str, Any]) -> str:
        """Get session info."""
        session_name = tool_input.get("session_name")
        info = self.session_manager.get_session_info(session_name)
        if not info:
            return f"Session '{session_name}' not found" if session_name else "No current session"
        return json.dumps(info, default=str)

    def _tool_get_hosts(self) -> str:
        """List configured hosts."""
        hosts = self.ssh_proxy.list_hosts()
        return json.dumps(hosts)

    async def _tool_compact_session(self, tool_input: Dict[str, Any]) -> str:
        """Compact session messages."""
        n = tool_input.get("n", 10)
        session_name = tool_input.get("session_name")

        if session_name is None:
            session_name = self.session_manager.get_current_session_name()

        messages = self.session_manager.get_recent_messages(n, session_name)
        if not messages:
            return "No messages to compact"

        summary = await self._generate_summary(messages)
        count = self.session_manager.compact_messages(n, summary, session_name)
        return f"Compacted {count} messages. Summary: {summary}"

    async def _generate_summary(self, messages: List[Dict[str, Any]]) -> str:
        """Generate a summary of messages using local Claude."""
        # Build a prompt for summarization
        conversation = "\n".join(
            f"{msg['role']}: {msg['content']}" for msg in messages
        )

        result = anthropic_completion(
            messages=[
                {
                    "role": "user",
                    "content": f"Summarize this conversation in 2-3 sentences, focusing on key topics and decisions:\n\n{conversation}",
                }
            ],
            context={"verbose": 0, "model": "claude-3-5-haiku-20241022"},
            system_prompt="You are a conversation summarizer. Provide concise summaries.",
        )

        return result["content"]
