"""Tests that state-file-already-exists errors send an openclaw notification."""

import subprocess
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest.mock import MagicMock, patch

import rip


class TestStateFileExistsNotification(unittest.TestCase):
    """When a disc is re-inserted and the state file already exists,
    the user should receive a Matrix notification about the failure."""

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
        self.disc_label = "MY_DISC"
        # Create pre-existing state file to trigger the collision
        state_dir = Path(self.tmpdir) / ".state"
        state_dir.mkdir(parents=True, exist_ok=True)
        (state_dir / f"{self.disc_label}.json").write_text('{"dummy": true}')

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    @patch("rip.notify")
    @patch("rip.analyze_error", return_value={"recommendation": "State collision"})
    def test_state_file_exists_sends_notification(self, mock_analyze, mock_notify):
        """Duplicate disc insert should notify via openclaw, not just exit."""
        # Mock blkid to return our disc label
        blkid_result = MagicMock()
        blkid_result.stdout = self.disc_label + "\n"

        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py"]), \
             patch("rip.subprocess") as mock_subprocess_mod:
            mock_subprocess_mod.run.return_value = blkid_result
            mock_subprocess_mod.CalledProcessError = subprocess.CalledProcessError

            with self.assertRaises(SystemExit) as ctx:
                rip.main()

            self.assertEqual(ctx.exception.code, 1)

        # Key assertion: notify must be called with an error message
        mock_notify.assert_called()
        msg = mock_notify.call_args[0][1]
        self.assertIn("FAILED", msg)


if __name__ == "__main__":
    unittest.main()
