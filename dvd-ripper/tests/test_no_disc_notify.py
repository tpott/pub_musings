"""Tests that --resume with no disc in drive sends a distinct notification
from the 'disc present but no state file' case."""

import shutil
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import rip


class TestNoDiscInDriveNotification(unittest.TestCase):
    """When the optical drive is empty, the notification should clearly
    indicate the drive is empty (not that no matching state was found)."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        (Path(self.tmpdir) / "rip.conf").write_text(
            f'RIP_DIR="{self.tmpdir}"\n'
            'OPENCLAW_BIN="/usr/bin/openclaw"\n'
            'OPENCLAW_TARGET="@user:matrix.org"\n'
        )

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    @patch("rip.notify")
    @patch("rip.analyze_error", return_value={"recommendation": ""})
    @patch("rip.discover_active_state", return_value=(None, "no_disc", None))
    def test_no_disc_message(self, mock_discover, mock_analyze, mock_notify):
        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--resume"]):
            with self.assertRaises(SystemExit):
                rip.main()

        mock_notify.assert_called()
        msg = mock_notify.call_args[0][1]
        self.assertIn("No disc in drive", msg)
        self.assertIn("/dev/sr0", msg)

    @patch("rip.notify")
    @patch("rip.analyze_error", return_value={"recommendation": ""})
    @patch(
        "rip.discover_active_state",
        return_value=(None, "no_state", "WEIRD_LABEL"),
    )
    def test_no_state_message_includes_label(
        self, mock_discover, mock_analyze, mock_notify
    ):
        """Disc present, label read, but no state file — notification must
        include the disc label so the user knows which disc was seen."""
        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--resume"]):
            with self.assertRaises(SystemExit):
                rip.main()

        mock_notify.assert_called()
        msg = mock_notify.call_args[0][1]
        self.assertIn("No state file matches inserted disc", msg)
        self.assertIn("WEIRD_LABEL", msg)


if __name__ == "__main__":
    unittest.main()
