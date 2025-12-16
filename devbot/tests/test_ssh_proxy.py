# test_ssh_proxy.py

import asyncio
import json
import os
import sys
import unittest
from unittest.mock import AsyncMock, MagicMock, patch

# Add parent directory to path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from ssh_proxy import SSHProxy, ProxyResult


def create_test_proxy(hosts=None):
    """Helper to create SSHProxy with test configuration."""
    if hosts is None:
        hosts = {
            "testhost": {"hostname": "test.example.com", "user": "testuser"},
            "prodhost": {"hostname": "prod.example.com", "user": "produser"},
        }
    config = {"hosts": hosts}
    return SSHProxy(config)


class TestParseStreamOutput(unittest.TestCase):
    """Test the _parse_stream_output method."""

    def setUp(self):
        self.proxy = create_test_proxy()

    def test_parse_init_message(self):
        """Test parsing init message extracts session_id."""
        output = '{"type":"init","session_id":"sess_abc123"}\n{"type":"result","result":"Hello","message_id":"msg_1"}'
        result = self.proxy._parse_stream_output(output)

        self.assertTrue(result.success)
        self.assertEqual(result.session_id, "sess_abc123")

    def test_parse_content_block_delta(self):
        """Test parsing content_block_delta extracts text."""
        output = '{"type":"content_block_delta","delta":{"text":"Hello world"}}'
        result = self.proxy._parse_stream_output(output)

        self.assertTrue(result.success)
        self.assertEqual(result.content, "Hello world")

    def test_parse_result_message(self):
        """Test parsing result message extracts message_id and result."""
        output = '{"type":"result","message_id":"msg_xyz789","result":"Final response"}'
        result = self.proxy._parse_stream_output(output)

        self.assertTrue(result.success)
        self.assertEqual(result.message_id, "msg_xyz789")
        self.assertIn("Final response", result.content)

    def test_parse_message_format(self):
        """Test parsing message format with content array."""
        output = '{"type":"message","content":[{"text":"Block 1"},{"text":"Block 2"}]}'
        result = self.proxy._parse_stream_output(output)

        self.assertTrue(result.success)
        self.assertIn("Block 1", result.content)
        self.assertIn("Block 2", result.content)

    def test_parse_assistant_format(self):
        """Test parsing assistant format with message.id and content."""
        output = '{"type":"assistant","message":{"id":"msg_asst123"},"content":"Assistant response"}'
        result = self.proxy._parse_stream_output(output)

        self.assertTrue(result.success)
        self.assertEqual(result.message_id, "msg_asst123")
        self.assertIn("Assistant response", result.content)

    def test_parse_multiple_deltas(self):
        """Test that multiple content_block_delta messages are concatenated."""
        output = (
            '{"type":"content_block_delta","delta":{"text":"Hello"}}\n'
            '{"type":"content_block_delta","delta":{"text":" "}}\n'
            '{"type":"content_block_delta","delta":{"text":"world"}}'
        )
        result = self.proxy._parse_stream_output(output)

        self.assertTrue(result.success)
        self.assertEqual(result.content, "Hello world")

    def test_parse_non_json_lines(self):
        """Test that non-JSON lines are appended to content."""
        output = "Plain text output\n" '{"type":"result","result":"JSON result","message_id":"msg_1"}'
        result = self.proxy._parse_stream_output(output)

        self.assertTrue(result.success)
        # Non-JSON line should be included in content
        self.assertIn("Plain text output", result.content)

    def test_parse_empty_output(self):
        """Test that empty output returns error."""
        output = ""
        result = self.proxy._parse_stream_output(output)

        self.assertFalse(result.success)
        self.assertIn("No content", result.error)

    def test_parse_mixed_format(self):
        """Test parsing realistic output with multiple message types."""
        output = (
            '{"type":"init","session_id":"sess_abc123"}\n'
            '{"type":"content_block_delta","delta":{"text":"Hello"}}\n'
            '{"type":"content_block_delta","delta":{"text":" world"}}\n'
            '{"type":"result","message_id":"msg_xyz789","result":"!"}'
        )
        result = self.proxy._parse_stream_output(output)

        self.assertTrue(result.success)
        self.assertEqual(result.session_id, "sess_abc123")
        self.assertEqual(result.message_id, "msg_xyz789")
        # Content should include all parts
        self.assertIn("Hello", result.content)
        self.assertIn("world", result.content)

    def test_parse_output_with_trailing_garbage_bytes(self):
        """Test parsing output that has random trailing bytes (e.g., from SSH terminal)."""
        # Simulate SSH output with valid JSON followed by random terminal garbage
        valid_json = '{"type":"result","message_id":"msg_1","result":"Valid response"}'
        # Random trailing bytes that might come from SSH pseudo-terminal
        garbage_suffix = "\x1b[0m\x81\xfd\xfe\xff"
        output = valid_json + "\n" + garbage_suffix

        result = self.proxy._parse_stream_output(output)

        # Should still extract the valid content
        self.assertTrue(result.success)
        self.assertIn("Valid response", result.content)
        self.assertEqual(result.message_id, "msg_1")


class TestSSHProxyValidation(unittest.TestCase):
    """Test host validation methods."""

    def setUp(self):
        self.proxy = create_test_proxy()

    def test_validate_host_success(self):
        """Test that configured host validates successfully."""
        self.assertTrue(self.proxy._validate_host("testhost"))
        self.assertTrue(self.proxy._validate_host("prodhost"))

    def test_validate_host_failure(self):
        """Test that unknown host fails validation."""
        self.assertFalse(self.proxy._validate_host("unknownhost"))
        self.assertFalse(self.proxy._validate_host(""))

    def test_get_host_info_success(self):
        """Test getting host info for configured host."""
        info = self.proxy._get_host_info("testhost")

        self.assertIsNotNone(info)
        self.assertEqual(info["hostname"], "test.example.com")
        self.assertEqual(info["user"], "testuser")

    def test_get_host_info_not_found(self):
        """Test getting host info for unknown host."""
        info = self.proxy._get_host_info("unknownhost")
        self.assertIsNone(info)

    def test_list_hosts(self):
        """Test listing all configured hosts."""
        hosts = self.proxy.list_hosts()

        self.assertIn("testhost", hosts)
        self.assertIn("prodhost", hosts)
        self.assertEqual(len(hosts), 2)


class TestSSHProxyExecute(unittest.TestCase):
    """Test the execute method with mocked subprocess."""

    def setUp(self):
        self.proxy = create_test_proxy()

    def test_execute_invalid_host(self):
        """Test execute returns error for non-whitelisted host."""
        result = asyncio.run(
            self.proxy.execute(message="test", hostname="invalidhost")
        )

        self.assertFalse(result.success)
        self.assertIn("not in configured whitelist", result.error)
        self.assertIn("invalidhost", result.error)

    def test_execute_host_not_found(self):
        """Test execute returns error when host config is missing."""
        # Create proxy with host that validates but has no config
        # This shouldn't happen in practice, but tests the code path
        proxy = create_test_proxy()
        with patch.object(proxy, "_validate_host", return_value=True):
            with patch.object(proxy, "_get_host_info", return_value=None):
                result = asyncio.run(
                    proxy.execute(message="test", hostname="ghosthost")
                )

        self.assertFalse(result.success)
        self.assertIn("not found", result.error)

    @patch("asyncio.create_subprocess_exec")
    def test_execute_success(self, mock_subprocess):
        """Test successful SSH execution with valid stream-json output."""
        # Mock subprocess
        mock_process = AsyncMock()
        mock_process.returncode = 0
        mock_process.communicate = AsyncMock(
            return_value=(
                b'{"type":"init","session_id":"sess_123"}\n'
                b'{"type":"result","message_id":"msg_456","result":"Success"}',
                b"",
            )
        )
        mock_subprocess.return_value = mock_process

        result = asyncio.run(
            self.proxy.execute(message="Hello Claude", hostname="testhost")
        )

        self.assertTrue(result.success)
        self.assertEqual(result.session_id, "sess_123")
        self.assertEqual(result.message_id, "msg_456")
        self.assertIn("Success", result.content)

    @patch("asyncio.create_subprocess_exec")
    def test_execute_with_resume_id(self, mock_subprocess):
        """Test that resume_id is included in SSH command."""
        mock_process = AsyncMock()
        mock_process.returncode = 0
        mock_process.communicate = AsyncMock(
            return_value=(
                b'{"type":"result","message_id":"msg_1","result":"Resumed"}',
                b"",
            )
        )
        mock_subprocess.return_value = mock_process

        result = asyncio.run(
            self.proxy.execute(
                message="Continue",
                hostname="testhost",
                resume_id="prev_msg_123",
            )
        )

        self.assertTrue(result.success)
        # Verify --resume was in the command
        call_args = mock_subprocess.call_args
        cmd = call_args[0]  # Positional args to create_subprocess_exec
        cmd_str = " ".join(cmd)
        self.assertIn("--resume", cmd_str)
        self.assertIn("prev_msg_123", cmd_str)

    @patch("asyncio.create_subprocess_exec")
    def test_execute_ssh_failure(self, mock_subprocess):
        """Test handling of SSH command failure (non-zero exit)."""
        mock_process = AsyncMock()
        mock_process.returncode = 255
        mock_process.communicate = AsyncMock(
            return_value=(b"", b"Connection refused")
        )
        mock_subprocess.return_value = mock_process

        result = asyncio.run(
            self.proxy.execute(message="test", hostname="testhost")
        )

        self.assertFalse(result.success)
        self.assertIn("SSH command failed", result.error)
        self.assertIn("255", result.error)
        self.assertIn("Connection refused", result.error)

    @patch("asyncio.create_subprocess_exec")
    def test_execute_timeout(self, mock_subprocess):
        """Test handling of timeout."""
        mock_process = AsyncMock()
        mock_process.communicate = AsyncMock(
            side_effect=asyncio.TimeoutError()
        )
        mock_subprocess.return_value = mock_process

        result = asyncio.run(
            self.proxy.execute(message="test", hostname="testhost", timeout=10)
        )

        self.assertFalse(result.success)
        self.assertIn("timed out", result.error)

    @patch("asyncio.create_subprocess_exec")
    def test_execute_exception(self, mock_subprocess):
        """Test handling of generic exceptions."""
        mock_subprocess.side_effect = OSError("No such file")

        result = asyncio.run(
            self.proxy.execute(message="test", hostname="testhost")
        )

        self.assertFalse(result.success)
        self.assertIn("SSH execution error", result.error)
        self.assertIn("No such file", result.error)

    @patch("asyncio.create_subprocess_exec")
    def test_execute_builds_correct_command(self, mock_subprocess):
        """Test that SSH command is built correctly with proper escaping."""
        mock_process = AsyncMock()
        mock_process.returncode = 0
        mock_process.communicate = AsyncMock(
            return_value=(
                b'{"type":"result","message_id":"msg_1","result":"OK"}',
                b"",
            )
        )
        mock_subprocess.return_value = mock_process

        asyncio.run(
            self.proxy.execute(
                message="Hello with 'quotes' and $pecial",
                hostname="testhost",
            )
        )

        # Verify command structure
        call_args = mock_subprocess.call_args
        cmd = call_args[0]

        # Should start with ssh -t user@host
        self.assertEqual(cmd[0], "ssh")
        self.assertEqual(cmd[1], "-t")
        self.assertEqual(cmd[2], "testuser@test.example.com")

        # Remote command should include claude with proper flags
        remote_cmd = cmd[3]
        self.assertIn("claude", remote_cmd)
        self.assertIn("--print", remote_cmd)
        self.assertIn("--output-format", remote_cmd)
        self.assertIn("stream-json", remote_cmd)
        self.assertIn("--verbose", remote_cmd)

    @patch("asyncio.create_subprocess_exec")
    def test_execute_with_trailing_garbage_bytes(self, mock_subprocess):
        """Test execution handles stdout with random trailing bytes from SSH."""
        # SSH with pseudo-terminal (-t) can sometimes emit terminal control
        # sequences or garbled bytes at the end of output
        mock_process = AsyncMock()
        mock_process.returncode = 0
        # Valid JSON followed by random binary garbage (terminal escape codes, etc.)
        stdout_with_garbage = (
            b'{"type":"init","session_id":"sess_123"}\n'
            b'{"type":"result","message_id":"msg_456","result":"Response"}\n'
            b'\x1b[0m\x81\xfd\xfe\xff\r\n'
        )
        mock_process.communicate = AsyncMock(
            return_value=(stdout_with_garbage, b"")
        )
        mock_subprocess.return_value = mock_process

        result = asyncio.run(
            self.proxy.execute(message="test", hostname="testhost")
        )

        # Should still successfully parse the valid JSON content
        self.assertTrue(result.success)
        self.assertEqual(result.session_id, "sess_123")
        self.assertEqual(result.message_id, "msg_456")
        self.assertIn("Response", result.content)


class TestProxyResult(unittest.TestCase):
    """Test ProxyResult dataclass."""

    def test_proxy_result_defaults(self):
        """Test ProxyResult default values."""
        result = ProxyResult(success=True, content="Test")

        self.assertTrue(result.success)
        self.assertEqual(result.content, "Test")
        self.assertIsNone(result.session_id)
        self.assertIsNone(result.message_id)
        self.assertIsNone(result.error)

    def test_proxy_result_all_fields(self):
        """Test ProxyResult with all fields set."""
        result = ProxyResult(
            success=True,
            content="Response",
            session_id="sess_1",
            message_id="msg_1",
            error=None,
        )

        self.assertTrue(result.success)
        self.assertEqual(result.content, "Response")
        self.assertEqual(result.session_id, "sess_1")
        self.assertEqual(result.message_id, "msg_1")

    def test_proxy_result_error_case(self):
        """Test ProxyResult for error case."""
        result = ProxyResult(
            success=False,
            content="",
            error="Something went wrong",
        )

        self.assertFalse(result.success)
        self.assertEqual(result.content, "")
        self.assertEqual(result.error, "Something went wrong")


if __name__ == "__main__":
    unittest.main()
