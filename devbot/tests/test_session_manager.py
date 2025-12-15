# test_session_manager.py

import json
import os
import shutil
import tempfile
import time
import unittest
from pathlib import Path
from unittest.mock import patch

import sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from session_manager import SessionManager


class TestSessionManager(unittest.TestCase):
    """Unit tests for SessionManager class."""

    def setUp(self):
        """Set up test fixtures with a temporary directory."""
        self.temp_dir = tempfile.mkdtemp()
        self.config = {
            "default_host": "testhost",
            "hosts": {"testhost": {"hostname": "test.example.com", "user": "testuser"}},
        }

        # Patch get_data_dir to return our temp directory
        self.data_dir_patcher = patch(
            "session_manager.get_data_dir", return_value=Path(self.temp_dir)
        )
        self.default_host_patcher = patch(
            "session_manager.get_default_host", return_value="testhost"
        )
        self.data_dir_patcher.start()
        self.default_host_patcher.start()

        self.manager = SessionManager(self.config)

    def tearDown(self):
        """Clean up temporary directory."""
        self.data_dir_patcher.stop()
        self.default_host_patcher.stop()

        # Clean up temp files
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_init_creates_directories(self):
        """Test that initialization creates required directories."""
        self.assertTrue(self.manager.history_dir.exists())
        self.assertTrue(self.manager.archive_dir.exists())

    def test_create_session(self):
        """Test creating a new session."""
        session = self.manager.create_session("test-session")

        self.assertIn("session_id", session)
        self.assertEqual(session["hostname"], "testhost")
        self.assertEqual(session["message_count"], 0)
        self.assertIsNone(session["last_message_id"])
        self.assertIn("created_at", session)
        self.assertIn("updated_at", session)

    def test_create_session_adds_hashtag(self):
        """Test that session names get # prefix if missing."""
        self.manager.create_session("mysession")
        sessions = self.manager.list_sessions()
        self.assertIn("#mysession", sessions)

    def test_create_session_with_custom_hostname(self):
        """Test creating session with specific hostname."""
        session = self.manager.create_session("custom", hostname="customhost")
        self.assertEqual(session["hostname"], "customhost")

    def test_switch_session(self):
        """Test switching between sessions."""
        self.manager.create_session("session1")
        self.manager.create_session("session2")

        # Should be on session2 (just created)
        self.assertEqual(self.manager.get_current_session_name(), "#session2")

        # Switch to session1
        result = self.manager.switch_session("session1")
        self.assertIsNotNone(result)
        self.assertEqual(self.manager.get_current_session_name(), "#session1")

    def test_switch_session_nonexistent(self):
        """Test switching to non-existent session returns None."""
        result = self.manager.switch_session("nonexistent")
        self.assertIsNone(result)

    def test_get_current_session_creates_default(self):
        """Test that get_current_session creates default if needed."""
        session = self.manager.get_current_session()
        self.assertIsNotNone(session)
        self.assertEqual(self.manager.get_current_session_name(), "#default")

    def test_add_message(self):
        """Test adding messages to session history."""
        self.manager.create_session("test")

        self.manager.add_message("user", "Hello")
        self.manager.add_message("assistant", "Hi there!")

        messages = self.manager.get_all_messages()
        self.assertEqual(len(messages), 2)
        self.assertEqual(messages[0]["role"], "user")
        self.assertEqual(messages[0]["content"], "Hello")
        self.assertEqual(messages[1]["role"], "assistant")
        self.assertEqual(messages[1]["content"], "Hi there!")

    def test_add_message_with_proxied_flag(self):
        """Test adding message with proxied flag."""
        self.manager.create_session("test")
        self.manager.add_message("user", "Test", proxied=True)

        messages = self.manager.get_all_messages()
        self.assertTrue(messages[0].get("proxied"))

    def test_add_message_with_remote_message_id(self):
        """Test adding message with remote message ID."""
        self.manager.create_session("test")
        self.manager.add_message("assistant", "Response", remote_message_id="msg_123")

        messages = self.manager.get_all_messages()
        self.assertEqual(messages[0]["remote_message_id"], "msg_123")

        # Check that session was updated with last_message_id
        session = self.manager.get_current_session()
        self.assertEqual(session["last_message_id"], "msg_123")

    def test_add_message_updates_count(self):
        """Test that message count is updated."""
        self.manager.create_session("test")

        self.manager.add_message("user", "Message 1")
        self.manager.add_message("user", "Message 2")
        self.manager.add_message("user", "Message 3")

        session = self.manager.get_current_session()
        self.assertEqual(session["message_count"], 3)

    def test_list_sessions(self):
        """Test listing all sessions."""
        self.manager.create_session("session1")
        self.manager.create_session("session2")
        self.manager.create_session("session3")

        sessions = self.manager.list_sessions()
        self.assertEqual(len(sessions), 3)
        self.assertIn("#session1", sessions)
        self.assertIn("#session2", sessions)
        self.assertIn("#session3", sessions)

    def test_get_session_info(self):
        """Test getting detailed session info."""
        self.manager.create_session("infotest")
        self.manager.add_message("user", "Test message")

        info = self.manager.get_session_info()
        self.assertEqual(info["name"], "#infotest")
        self.assertEqual(info["hostname"], "testhost")
        self.assertIn("session_id", info)
        self.assertIn("actual_message_count", info)

    def test_get_session_info_by_name(self):
        """Test getting info for a specific session."""
        self.manager.create_session("session1")
        self.manager.create_session("session2")

        info = self.manager.get_session_info("session1")
        self.assertEqual(info["name"], "#session1")

    def test_get_session_info_nonexistent(self):
        """Test getting info for non-existent session."""
        info = self.manager.get_session_info("nonexistent")
        self.assertIsNone(info)

    def test_get_recent_messages(self):
        """Test getting recent messages."""
        self.manager.create_session("test")

        for i in range(10):
            self.manager.add_message("user", f"Message {i}")

        recent = self.manager.get_recent_messages(5)
        self.assertEqual(len(recent), 5)
        self.assertEqual(recent[0]["content"], "Message 5")
        self.assertEqual(recent[4]["content"], "Message 9")

    def test_get_recent_messages_less_than_n(self):
        """Test getting recent messages when fewer exist."""
        self.manager.create_session("test")

        self.manager.add_message("user", "Only message")

        recent = self.manager.get_recent_messages(10)
        self.assertEqual(len(recent), 1)

    def test_get_all_messages(self):
        """Test getting all messages."""
        self.manager.create_session("test")

        for i in range(5):
            self.manager.add_message("user", f"Message {i}")

        messages = self.manager.get_all_messages()
        self.assertEqual(len(messages), 5)

    def test_update_session(self):
        """Test updating session fields."""
        self.manager.create_session("test")
        original_updated = self.manager.get_current_session()["updated_at"]

        time.sleep(0.01)  # Ensure time difference
        self.manager.update_session("#test", last_message_id="msg_new")

        session = self.manager.get_current_session()
        self.assertEqual(session["last_message_id"], "msg_new")
        self.assertGreater(session["updated_at"], original_updated)

    def test_compact_messages(self):
        """Test compacting messages into summary."""
        self.manager.create_session("test")

        for i in range(10):
            self.manager.add_message("user", f"Message {i}")

        count = self.manager.compact_messages(5, "Summary of last 5 messages")

        self.assertEqual(count, 5)

        messages = self.manager.get_all_messages()
        # Should have 5 original + 1 summary = 6
        self.assertEqual(len(messages), 6)

        # Last message should be the summary
        self.assertEqual(messages[-1]["role"], "system")
        self.assertIn("Summary of last 5 messages", messages[-1]["content"])
        self.assertEqual(messages[-1]["compacted_count"], 5)

    def test_compact_messages_creates_archive(self):
        """Test that compact creates archive file."""
        self.manager.create_session("archivetest")

        for i in range(5):
            self.manager.add_message("user", f"Message {i}")

        self.manager.compact_messages(3, "Test summary")

        # Check archive directory has a file
        archive_files = list(self.manager.archive_dir.glob("archivetest_*.jsonl"))
        self.assertEqual(len(archive_files), 1)

        # Verify archive content
        with open(archive_files[0]) as f:
            archived = [json.loads(line) for line in f]
        self.assertEqual(len(archived), 3)

    def test_compact_messages_fewer_than_n(self):
        """Test compacting when fewer messages exist than requested."""
        self.manager.create_session("test")

        self.manager.add_message("user", "Message 1")
        self.manager.add_message("user", "Message 2")

        count = self.manager.compact_messages(10, "Summary")
        self.assertEqual(count, 2)

    def test_compact_messages_empty(self):
        """Test compacting with no messages."""
        self.manager.create_session("test")

        count = self.manager.compact_messages(5, "Summary")
        self.assertEqual(count, 0)

    def test_persistence_across_instances(self):
        """Test that sessions persist across manager instances."""
        self.manager.create_session("persistent")
        self.manager.add_message("user", "Persistent message")

        # Create new manager instance
        new_manager = SessionManager(self.config)

        sessions = new_manager.list_sessions()
        self.assertIn("#persistent", sessions)

        messages = new_manager.get_all_messages("#persistent")
        self.assertEqual(len(messages), 1)
        self.assertEqual(messages[0]["content"], "Persistent message")

    def test_message_timestamp(self):
        """Test that messages have timestamps."""
        self.manager.create_session("test")

        before = time.time()
        self.manager.add_message("user", "Timed message")
        after = time.time()

        messages = self.manager.get_all_messages()
        self.assertGreaterEqual(messages[0]["timestamp"], before)
        self.assertLessEqual(messages[0]["timestamp"], after)


class TestSessionManagerEdgeCases(unittest.TestCase):
    """Edge case tests for SessionManager."""

    def setUp(self):
        self.temp_dir = tempfile.mkdtemp()
        self.config = {"default_host": "testhost"}

        self.data_dir_patcher = patch(
            "session_manager.get_data_dir", return_value=Path(self.temp_dir)
        )
        self.default_host_patcher = patch(
            "session_manager.get_default_host", return_value="testhost"
        )
        self.data_dir_patcher.start()
        self.default_host_patcher.start()

        self.manager = SessionManager(self.config)

    def tearDown(self):
        self.data_dir_patcher.stop()
        self.default_host_patcher.stop()
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def test_session_name_with_special_chars(self):
        """Test session names with various characters."""
        self.manager.create_session("my-session_v2")
        sessions = self.manager.list_sessions()
        self.assertIn("#my-session_v2", sessions)

    def test_switch_with_and_without_hashtag(self):
        """Test switching works with or without # prefix."""
        self.manager.create_session("test")

        # Switch without #
        result = self.manager.switch_session("test")
        self.assertIsNotNone(result)

        # Switch with #
        result = self.manager.switch_session("#test")
        self.assertIsNotNone(result)

    def test_empty_content_message(self):
        """Test adding message with empty content."""
        self.manager.create_session("test")
        self.manager.add_message("user", "")

        messages = self.manager.get_all_messages()
        self.assertEqual(len(messages), 1)
        self.assertEqual(messages[0]["content"], "")

    def test_large_message_content(self):
        """Test adding message with large content."""
        self.manager.create_session("test")
        large_content = "x" * 100000  # 100KB
        self.manager.add_message("user", large_content)

        messages = self.manager.get_all_messages()
        self.assertEqual(messages[0]["content"], large_content)


if __name__ == "__main__":
    unittest.main()
