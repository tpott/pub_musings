"""Tests for state management: save/load/archive pipeline state."""

import json
import os
import tempfile
import unittest
from pathlib import Path

from state import (
    archive_state,
    load_state,
    new_state,
    save_state,
    state_path_for_label,
)


class TestStatePathForLabel(unittest.TestCase):
    def test_returns_state_dir_path(self):
        path = state_path_for_label("/tmp/rip", "AVATAR_BOOK_1_DISC_1")
        self.assertEqual(path, Path("/tmp/rip/.state/AVATAR_BOOK_1_DISC_1.json"))

    def test_label_with_spaces(self):
        path = state_path_for_label("/tmp/rip", "SHE RA")
        self.assertEqual(path, Path("/tmp/rip/.state/SHE RA.json"))


class TestNewState(unittest.TestCase):
    def test_creates_state_with_required_fields(self):
        state = new_state("/tmp/rip", "AVATAR_BOOK_1_DISC_1")
        self.assertEqual(state["version"], 1)
        self.assertEqual(state["status"], "running")
        self.assertEqual(state["disc_label"], "AVATAR_BOOK_1_DISC_1")
        self.assertIn("created_at", state)
        self.assertIn("updated_at", state)
        self.assertIsInstance(state["stages"], dict)
        self.assertEqual(state["stages"], {})
        self.assertIsInstance(state["verification"], dict)

    def test_state_path_set(self):
        state = new_state("/tmp/rip", "TEST_DISC")
        self.assertEqual(
            state["_state_path"],
            str(Path("/tmp/rip/.state/TEST_DISC.json")),
        )


class TestSaveAndLoadState(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_save_creates_file(self):
        state = new_state(self.tmpdir, "TEST_DISC")
        save_state(state)
        path = Path(state["_state_path"])
        self.assertTrue(path.exists())

    def test_roundtrip(self):
        state = new_state(self.tmpdir, "TEST_DISC")
        state["plan"] = {"show_name": "Test Show", "season": 1}
        save_state(state)
        loaded = load_state(state["_state_path"])
        self.assertEqual(loaded["disc_label"], "TEST_DISC")
        self.assertEqual(loaded["plan"]["show_name"], "Test Show")
        self.assertEqual(loaded["_state_path"], state["_state_path"])

    def test_save_creates_state_dir(self):
        """save_state should create .state/ directory if missing."""
        state = new_state(self.tmpdir, "TEST_DISC")
        state_dir = Path(self.tmpdir) / ".state"
        self.assertFalse(state_dir.exists())
        save_state(state)
        self.assertTrue(state_dir.exists())

    def test_save_updates_timestamp(self):
        state = new_state(self.tmpdir, "TEST_DISC")
        original_updated = state["updated_at"]
        # Force a small time difference
        import time
        time.sleep(0.01)
        save_state(state)
        loaded = load_state(state["_state_path"])
        self.assertGreaterEqual(loaded["updated_at"], original_updated)

    def test_save_atomic_no_tmp_leftover(self):
        """After save, no .tmp file should remain."""
        state = new_state(self.tmpdir, "TEST_DISC")
        save_state(state)
        state_dir = Path(self.tmpdir) / ".state"
        tmp_files = list(state_dir.glob("*.tmp"))
        self.assertEqual(tmp_files, [])

    def test_save_valid_json(self):
        state = new_state(self.tmpdir, "TEST_DISC")
        state["plan"] = {"episodes": [{"id": 1}, {"id": 2}]}
        save_state(state)
        with open(state["_state_path"]) as f:
            data = json.load(f)
        self.assertEqual(data["plan"]["episodes"][0]["id"], 1)


class TestArchiveState(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_moves_to_history(self):
        state = new_state(self.tmpdir, "TEST_DISC")
        save_state(state)
        original_path = Path(state["_state_path"])
        self.assertTrue(original_path.exists())

        archive_state(state)
        self.assertFalse(original_path.exists())

        history_dir = Path(self.tmpdir) / ".state" / "history"
        history_files = list(history_dir.glob("TEST_DISC-*.json"))
        self.assertEqual(len(history_files), 1)

    def test_archived_file_is_valid_json(self):
        state = new_state(self.tmpdir, "TEST_DISC")
        state["plan"] = {"show_name": "Test"}
        save_state(state)
        archive_state(state)

        history_dir = Path(self.tmpdir) / ".state" / "history"
        history_files = list(history_dir.glob("TEST_DISC-*.json"))
        with open(history_files[0]) as f:
            data = json.load(f)
        self.assertEqual(data["plan"]["show_name"], "Test")

    def test_updates_state_path(self):
        state = new_state(self.tmpdir, "TEST_DISC")
        save_state(state)
        archive_state(state)
        self.assertIn("history", state["_state_path"])


if __name__ == "__main__":
    unittest.main()
