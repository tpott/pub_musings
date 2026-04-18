"""Tests that --resume with no active state sends an openclaw notification."""

import shutil
import tempfile
import unittest
from pathlib import Path
from unittest.mock import MagicMock, patch

import rip


class TestNoActiveStateNotification(unittest.TestCase):
    """When --resume is passed with no arg and discover_active_state()
    returns None, the user should receive a Matrix notification about
    the failure."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.conf = {
            "RIP_DIR": self.tmpdir,
            "OPENCLAW_BIN": "/usr/bin/openclaw",
            "OPENCLAW_TARGET": "@user:matrix.org",
        }
        # Create rip.conf in tmpdir (main reads it relative to __file__)
        (Path(self.tmpdir) / "rip.conf").write_text(
            f'RIP_DIR="{self.tmpdir}"\n'
            'OPENCLAW_BIN="/usr/bin/openclaw"\n'
            'OPENCLAW_TARGET="@user:matrix.org"\n'
        )

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    @patch("rip.notify")
    @patch("rip.analyze_error", return_value={"recommendation": "No state"})
    @patch("rip.discover_active_state", return_value=None)
    def test_no_active_state_sends_notification(
        self, mock_discover, mock_analyze, mock_notify
    ):
        """--resume with no active state should notify via openclaw,
        not just exit."""
        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--resume"]):

            with self.assertRaises(SystemExit) as ctx:
                rip.main()

            self.assertEqual(ctx.exception.code, 1)

        # Key assertion: notify must be called with an error message
        mock_notify.assert_called()
        msg = mock_notify.call_args[0][1]
        self.assertIn("FAILED", msg)


if __name__ == "__main__":
    unittest.main()
