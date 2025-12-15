# ssh_proxy.py

import asyncio
import json
import shlex
from dataclasses import dataclass
from typing import Any, Dict, List, Optional

from serve_config import get_hosts


@dataclass
class ProxyResult:
    """Result from SSH proxy execution."""

    success: bool
    content: str
    session_id: Optional[str] = None
    message_id: Optional[str] = None
    error: Optional[str] = None


class SSHProxy:
    """Secure SSH execution for remote Claude CLI."""

    def __init__(self, config: Dict[str, Any]):
        self.config = config
        self.hosts = get_hosts(config)

    def _validate_host(self, hostname: str) -> bool:
        """Validate that hostname is in the configured whitelist."""
        return hostname in self.hosts

    def _get_host_info(self, hostname: str) -> Optional[Dict[str, str]]:
        """Get host configuration (hostname, user)."""
        return self.hosts.get(hostname)

    async def execute(
        self,
        message: str,
        hostname: str,
        resume_id: Optional[str] = None,
        timeout: int = 300,
    ) -> ProxyResult:
        """
        Execute a message on remote Claude CLI via SSH.

        Args:
            message: The message to send to Claude CLI
            hostname: The configured host name (must be in whitelist)
            resume_id: Optional message ID to resume a conversation
            timeout: Timeout in seconds (default 5 minutes)

        Returns:
            ProxyResult with content and metadata
        """
        # Validate hostname against whitelist
        if not self._validate_host(hostname):
            return ProxyResult(
                success=False,
                content="",
                error=f"Host '{hostname}' not in configured whitelist. "
                f"Available hosts: {list(self.hosts.keys())}",
            )

        host_info = self._get_host_info(hostname)
        if host_info is None:
            return ProxyResult(
                success=False,
                content="",
                error=f"Host configuration not found for '{hostname}'",
            )

        # Build the remote command with proper escaping
        remote_cmd_parts = ["claude", "--print", "--output-format", "stream-json", "--verbose"]

        if resume_id:
            remote_cmd_parts.extend(["--resume", resume_id])

        # Add the message as the final argument
        remote_cmd_parts.append(message)

        # Build the full remote command string (properly quoted for remote shell)
        remote_cmd = " ".join(shlex.quote(part) for part in remote_cmd_parts)

        # Build SSH command (shell=False for security)
        ssh_target = f"{host_info['user']}@{host_info['hostname']}"
        # ssh -t forces psuedo-terminal creation
        cmd = ["ssh", "-t", ssh_target, remote_cmd]
        print(f"executing: {cmd}")

        try:
            process = await asyncio.create_subprocess_exec(
                *cmd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )

            stdout, stderr = await asyncio.wait_for(
                process.communicate(), timeout=timeout
            )

            if process.returncode != 0:
                error_msg = stderr.decode("utf-8").strip()
                return ProxyResult(
                    success=False,
                    content="",
                    error=f"SSH command failed (exit {process.returncode}): {error_msg}",
                )

            # Parse streaming JSON output
            return self._parse_stream_output(stdout.decode("utf-8"))

        except asyncio.TimeoutError:
            return ProxyResult(
                success=False,
                content="",
                error=f"SSH command timed out after {timeout} seconds",
            )
        except Exception as e:
            return ProxyResult(
                success=False,
                content="",
                error=f"SSH execution error: {str(e)}",
            )

    def _parse_stream_output(self, output: str) -> ProxyResult:
        """Parse Claude CLI stream-json output."""
        content_parts: List[str] = []
        session_id: Optional[str] = None
        message_id: Optional[str] = None

        for line in output.strip().split("\n"):
            if not line.strip():
                continue

            try:
                data = json.loads(line)

                # Handle different message types
                msg_type = data.get("type")

                if msg_type == "init":
                    session_id = data.get("session_id")

                elif msg_type == "content_block_delta":
                    delta = data.get("delta", {})
                    if "text" in delta:
                        content_parts.append(delta["text"])

                elif msg_type == "result":
                    message_id = data.get("message_id")

                elif msg_type == "message":
                    # Alternative format - full message
                    if "content" in data:
                        for block in data.get("content", []):
                            if isinstance(block, dict) and "text" in block:
                                content_parts.append(block["text"])

                elif msg_type == "assistant":
                    # Direct assistant message format
                    if "message" in data:
                        message_id = data.get("message", {}).get("id")
                    if "content" in data:
                        content_parts.append(data["content"])

            except json.JSONDecodeError:
                # Not JSON - might be plain text output
                content_parts.append(line)

        content = "".join(content_parts)

        if not content:
            return ProxyResult(
                success=False,
                content="",
                error="No content received from remote Claude CLI",
            )

        return ProxyResult(
            success=True,
            content=content,
            session_id=session_id,
            message_id=message_id,
        )

    def list_hosts(self) -> Dict[str, Dict[str, str]]:
        """List all configured hosts."""
        return self.hosts
