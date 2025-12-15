# test_dev_agent.py

import json
import os
from pathlib import Path
import shutil
import sys
import tempfile
import unittest
from unittest.mock import AsyncMock, MagicMock, patch

import asyncio

# Mock anthropic module before any imports
sys.modules["anthropic"] = MagicMock()

# Add parent directory to path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from dev_agent import DevAgent, TOOLS, SYSTEM_PROMPT
from ssh_proxy import ProxyResult


def create_test_agent(temp_dir, config=None):
    """Helper to create a DevAgent with proper mocking."""
    if config is None:
        config = {
            "anthropic_api_key": "test-key",
            "default_host": "testhost",
            "hosts": {
                "testhost": {"hostname": "test.example.com", "user": "testuser"},
                "prodhost": {"hostname": "prod.example.com", "user": "produser"},
            },
            "verbose": 0,
        }

    with patch("session_manager.get_data_dir", return_value=Path(temp_dir)):
        with patch("session_manager.get_default_host", return_value="testhost"):
            return DevAgent(config)


class TestDevAgentSlashCommands(unittest.TestCase):
    """Test slash command handling (no LLM calls)."""

    def setUp(self):
        """Set up test fixtures."""
        self.temp_dir = tempfile.mkdtemp()
        self.config = {
            "anthropic_api_key": "test-key",
            "default_host": "testhost",
            "hosts": {
                "testhost": {"hostname": "test.example.com", "user": "testuser"},
                "prodhost": {"hostname": "prod.example.com", "user": "produser"},
            },
            "verbose": 0,
        }
        self.agent = create_test_agent(self.temp_dir, self.config)

    def tearDown(self):
        """Clean up temporary directory."""
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_help_command(self):
        """Test /help command."""
        result = asyncio.run(self.agent._handle_slash_command("/help"))
        self.assertIn("Available commands", result)
        self.assertIn("/sessions", result)
        self.assertIn("/switch", result)

    def test_help_alias(self):
        """Test /? alias for help."""
        result = asyncio.run(self.agent._handle_slash_command("/?"))
        self.assertIn("Available commands", result)

    def test_sessions_command_empty(self):
        """Test /sessions with no sessions."""
        result = asyncio.run(self.agent._handle_slash_command("/sessions"))
        self.assertIsInstance(result, str)

    def test_sessions_command_with_sessions(self):
        """Test /sessions with multiple sessions."""
        self.agent.session_manager.create_session("session1")
        self.agent.session_manager.create_session("session2")

        result = asyncio.run(self.agent._handle_slash_command("/sessions"))
        self.assertIn("Sessions:", result)
        self.assertIn("#session1", result)
        self.assertIn("#session2", result)
        self.assertIn("(current)", result)

    def test_switch_command(self):
        """Test /switch command."""
        self.agent.session_manager.create_session("target")
        self.agent.session_manager.create_session("other")

        result = asyncio.run(self.agent._handle_slash_command("/switch target"))
        self.assertIn("Switched to session", result)
        self.assertEqual(
            self.agent.session_manager.get_current_session_name(), "#target"
        )

    def test_switch_command_no_args(self):
        """Test /switch without arguments."""
        result = asyncio.run(self.agent._handle_slash_command("/switch"))
        self.assertIn("Usage:", result)

    def test_switch_command_nonexistent(self):
        """Test /switch to non-existent session."""
        result = asyncio.run(self.agent._handle_slash_command("/switch nonexistent"))
        self.assertIn("not found", result)

    def test_new_command(self):
        """Test /new command."""
        result = asyncio.run(self.agent._handle_slash_command("/new mysession"))
        self.assertIn("Created", result)
        self.assertIn("mysession", result)

        sessions = self.agent.session_manager.list_sessions()
        self.assertIn("#mysession", sessions)

    def test_new_command_with_hostname(self):
        """Test /new command with hostname."""
        result = asyncio.run(
            self.agent._handle_slash_command("/new mysession prodhost")
        )
        self.assertIn("Created", result)

        info = self.agent.session_manager.get_session_info("mysession")
        self.assertEqual(info["hostname"], "prodhost")

    def test_new_command_no_args(self):
        """Test /new without arguments."""
        result = asyncio.run(self.agent._handle_slash_command("/new"))
        self.assertIn("Usage:", result)

    def test_hosts_command(self):
        """Test /hosts command."""
        result = asyncio.run(self.agent._handle_slash_command("/hosts"))
        self.assertIn("Configured hosts:", result)
        self.assertIn("testhost", result)
        self.assertIn("prodhost", result)
        self.assertIn("testuser@test.example.com", result)

    def test_info_command_current(self):
        """Test /info for current session."""
        self.agent.session_manager.create_session("infotest")

        result = asyncio.run(self.agent._handle_slash_command("/info"))
        self.assertIn("Session:", result)
        self.assertIn("#infotest", result)
        self.assertIn("Host:", result)

    def test_info_command_specific(self):
        """Test /info for specific session."""
        self.agent.session_manager.create_session("session1")
        self.agent.session_manager.create_session("session2")

        result = asyncio.run(self.agent._handle_slash_command("/info session1"))
        self.assertIn("#session1", result)

    def test_compact_command_no_args(self):
        """Test /compact without arguments."""
        result = asyncio.run(self.agent._handle_slash_command("/compact"))
        self.assertIn("Usage:", result)

    def test_compact_command_invalid_arg(self):
        """Test /compact with invalid argument."""
        result = asyncio.run(self.agent._handle_slash_command("/compact abc"))
        self.assertIn("Error", result)
        self.assertIn("number", result)

    def test_unknown_command(self):
        """Test unknown slash command."""
        result = asyncio.run(self.agent._handle_slash_command("/unknown"))
        self.assertIn("Unknown command", result)
        self.assertIn("/help", result)


class TestDevAgentTools(unittest.TestCase):
    """Test tool definitions and execution."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.config = {
            "anthropic_api_key": "test-key",
            "default_host": "testhost",
            "hosts": {
                "testhost": {"hostname": "test.example.com", "user": "testuser"},
            },
            "verbose": 0,
        }
        self.agent = create_test_agent(self.temp_dir, self.config)

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_tools_have_required_fields(self):
        """Test that all tools have required schema fields."""
        for tool in TOOLS:
            self.assertIn("name", tool)
            self.assertIn("description", tool)
            self.assertIn("input_schema", tool)
            self.assertEqual(tool["input_schema"]["type"], "object")
            self.assertIn("properties", tool["input_schema"])

    def test_tool_get_sessions(self):
        """Test get_sessions tool."""
        self.agent.session_manager.create_session("tool-test")

        result = self.agent._tool_get_sessions()
        data = json.loads(result)

        self.assertIn("sessions", data)
        self.assertIn("current", data)
        self.assertTrue(any(s["name"] == "#tool-test" for s in data["sessions"]))

    def test_tool_switch_session(self):
        """Test switch_session tool."""
        self.agent.session_manager.create_session("target")
        self.agent.session_manager.create_session("other")

        result = self.agent._tool_switch_session({"name": "target"})
        self.assertIn("Successfully switched", result)

    def test_tool_switch_session_not_found(self):
        """Test switch_session tool with non-existent session."""
        result = self.agent._tool_switch_session({"name": "nonexistent"})
        self.assertIn("not found", result)

    def test_tool_new_session(self):
        """Test new_session tool."""
        result = self.agent._tool_new_session({"name": "new-tool-session"})
        self.assertIn("Created", result)

        sessions = self.agent.session_manager.list_sessions()
        self.assertIn("#new-tool-session", sessions)

    def test_tool_info(self):
        """Test info tool."""
        self.agent.session_manager.create_session("info-test")

        result = self.agent._tool_info({})
        data = json.loads(result)

        self.assertEqual(data["name"], "#info-test")
        self.assertIn("hostname", data)

    def test_tool_get_hosts(self):
        """Test get_hosts tool."""
        result = self.agent._tool_get_hosts()
        data = json.loads(result)

        self.assertIn("testhost", data)
        self.assertEqual(data["testhost"]["hostname"], "test.example.com")


class TestDevAgentProxyMessage(unittest.TestCase):
    """Test proxy_message tool with mocked SSH."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.config = {
            "anthropic_api_key": "test-key",
            "default_host": "testhost",
            "hosts": {
                "testhost": {"hostname": "test.example.com", "user": "testuser"},
            },
            "verbose": 0,
        }
        self.agent = create_test_agent(self.temp_dir, self.config)

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_proxy_message_success(self):
        """Test proxy_message with successful SSH execution."""
        self.agent.session_manager.create_session("proxy-test")

        mock_result = ProxyResult(
            success=True,
            content="Response from remote Claude",
            session_id="sess_123",
            message_id="msg_456",
        )
        self.agent.ssh_proxy.execute = AsyncMock(return_value=mock_result)

        result = asyncio.run(
            self.agent._tool_proxy_message({"message": "Test message"})
        )

        self.assertEqual(result, "Response from remote Claude")

        messages = self.agent.session_manager.get_all_messages()
        self.assertEqual(len(messages), 2)
        self.assertEqual(messages[0]["role"], "user")
        self.assertTrue(messages[0]["proxied"])
        self.assertEqual(messages[1]["role"], "assistant")
        self.assertEqual(messages[1]["remote_message_id"], "msg_456")

    def test_proxy_message_ssh_failure(self):
        """Test proxy_message with SSH failure."""
        self.agent.session_manager.create_session("proxy-test")

        mock_result = ProxyResult(
            success=False,
            content="",
            error="Connection refused",
        )
        self.agent.ssh_proxy.execute = AsyncMock(return_value=mock_result)

        result = asyncio.run(
            self.agent._tool_proxy_message({"message": "Test message"})
        )

        self.assertIn("Error:", result)
        self.assertIn("Connection refused", result)

    def test_proxy_message_empty(self):
        """Test proxy_message with empty message."""
        result = asyncio.run(self.agent._tool_proxy_message({"message": ""}))
        self.assertIn("Error", result)
        self.assertIn("No message", result)


class TestDevAgentProcessInput(unittest.TestCase):
    """Test the main process_input method."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.config = {
            "anthropic_api_key": "test-key",
            "default_host": "testhost",
            "hosts": {
                "testhost": {"hostname": "test.example.com", "user": "testuser"},
            },
            "verbose": 0,
        }
        self.agent = create_test_agent(self.temp_dir, self.config)

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_process_input_slash_command(self):
        """Test that slash commands bypass LLM."""
        result = asyncio.run(self.agent.process_input("/help"))
        self.assertIn("Available commands", result)

    @patch("dev_agent.anthropic_completion")
    def test_process_input_text_response(self, mock_completion):
        """Test processing that returns text without tool use."""
        mock_completion.return_value = {
            "content": "Here is my response",
            "tool_use": None,
            "stop_reason": "end_turn",
        }

        result = asyncio.run(self.agent.process_input("Hello"))
        self.assertEqual(result, "Here is my response")
        mock_completion.assert_called_once()

    @patch("dev_agent.anthropic_completion")
    def test_process_input_tool_use(self, mock_completion):
        """Test processing with tool use."""
        # First call returns tool_use, second returns final response
        mock_completion.side_effect = [
            {
                "content": "",
                "tool_use": {
                    "id": "tool_123",
                    "name": "get_sessions",
                    "input": {},
                },
                "stop_reason": "tool_use",
            },
            {
                "content": "Here are your sessions...",
                "tool_use": None,
                "stop_reason": "end_turn",
            },
        ]

        result = asyncio.run(self.agent.process_input("Show me sessions"))
        self.assertEqual(result, "Here are your sessions...")
        self.assertEqual(mock_completion.call_count, 2)


class TestDevAgentSystemPrompt(unittest.TestCase):
    """Test system prompt and tool definitions."""

    def test_system_prompt_exists(self):
        """Test that system prompt is defined."""
        self.assertIsInstance(SYSTEM_PROMPT, str)
        self.assertGreater(len(SYSTEM_PROMPT), 100)

    def test_system_prompt_mentions_tools(self):
        """Test that system prompt mentions available tools."""
        self.assertIn("proxy_message", SYSTEM_PROMPT)
        self.assertIn("session", SYSTEM_PROMPT.lower())

    def test_all_tools_defined(self):
        """Test that expected tools are defined."""
        tool_names = {t["name"] for t in TOOLS}
        expected_tools = {
            "proxy_message",
            "get_sessions",
            "switch_session",
            "new_session",
            "info",
            "get_hosts",
            "compact_session",
        }
        self.assertEqual(tool_names, expected_tools)


if __name__ == "__main__":
    unittest.main()
