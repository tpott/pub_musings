"""Tests for discover_active_state distinguishing no-disc from no-state-file."""

import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from state import discover_active_state, new_state, save_state


class TestDiscoverActiveStateNoDisc(unittest.TestCase):
    """When blkid exits 2 (no media in drive), discover_active_state should
    report 'no_disc' rather than 'no_state'."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    @patch("state.subprocess.run")
    def test_blkid_exit_2_reports_no_disc(self, mock_run):
        """blkid exit code 2 means the device had no recognizable media.
        The function should return (None, 'no_disc', None)."""
        mock_run.side_effect = subprocess.CalledProcessError(
            returncode=2, cmd=["blkid"], stderr=""
        )
        state, reason, label = discover_active_state(self.tmpdir)
        self.assertIsNone(state)
        self.assertEqual(reason, "no_disc")
        self.assertIsNone(label)

    @patch("state.subprocess.run")
    def test_blkid_file_not_found_reports_no_disc(self, mock_run):
        """If blkid binary is missing, treat as no_disc (best available signal)."""
        mock_run.side_effect = FileNotFoundError("blkid not found")
        state, reason, label = discover_active_state(self.tmpdir)
        self.assertIsNone(state)
        self.assertEqual(reason, "no_disc")
        self.assertIsNone(label)


class TestDiscoverActiveStateNoStateFile(unittest.TestCase):
    """When blkid returns a label but no state file matches, report 'no_state'
    with the label so main() can include it in the notification."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    @patch("state.subprocess.run")
    def test_label_present_but_no_state_file(self, mock_run):
        class FakeResult:
            stdout = "SOME_DISC_LABEL\n"
        mock_run.return_value = FakeResult()
        state, reason, label = discover_active_state(self.tmpdir)
        self.assertIsNone(state)
        self.assertEqual(reason, "no_state")
        self.assertEqual(label, "SOME_DISC_LABEL")

    @patch("state.subprocess.run")
    def test_label_present_and_state_file_loads(self, mock_run):
        class FakeResult:
            stdout = "MY_DISC\n"
        mock_run.return_value = FakeResult()
        # Pre-create a state file for the label
        st = new_state(self.tmpdir, "MY_DISC")
        save_state(st)

        state, reason, label = discover_active_state(self.tmpdir)
        self.assertIsNotNone(state)
        self.assertEqual(state["disc_label"], "MY_DISC")
        self.assertEqual(reason, "ok")
        self.assertEqual(label, "MY_DISC")


if __name__ == "__main__":
    unittest.main()
