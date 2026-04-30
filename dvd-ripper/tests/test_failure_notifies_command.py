"""Tests that every failure notification includes an actionable follow-up command."""

import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import rip
from state import new_state, save_state
from verify import VerificationError


class TestFailureNotifiesCommand(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        (Path(self.tmpdir) / "rip.conf").write_text(
            f'RIP_DIR="{self.tmpdir}"\n'
            'OPENCLAW_BIN="/usr/bin/openclaw"\n'
            'OPENCLAW_TARGET="@user:matrix.org"\n'
        )
        state = new_state(self.tmpdir, "TEST_DISC")
        state["stages"] = {}
        state["plan"] = {"notify_msg": "Test", "episodes": []}
        state["verification"] = {"issues": []}
        save_state(state)

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _run_main_with_error(self, err):
        """Run main() with --resume TEST_DISC; run_pipeline raises err."""
        def fake_run_pipeline(stages, conf, state, save_fn):
            raise err

        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--resume", "TEST_DISC"]), \
             patch("rip.run_pipeline", side_effect=fake_run_pipeline), \
             patch("rip.analyze_error",
                   return_value={"recommendation": "manual check", "fix_command": None}):
            with self.assertRaises(SystemExit):
                rip.main()

    @patch("rip.notify")
    def test_count_mismatch_without_claude_includes_approve(self, mock_notify):
        """VerificationError with no fix_command gets --approve as second notification."""
        err = VerificationError(
            "count mismatch",
            recommendation="7 on disc, 2 selected",
            fix_command=None,
        )
        self._run_main_with_error(err)

        msgs = [c[0][1] for c in mock_notify.call_args_list]
        approve_msgs = [m for m in msgs if "--approve" in m]
        self.assertEqual(
            len(approve_msgs), 1,
            f"expected exactly one --approve notification, got: {msgs}",
        )
        self.assertIn("TEST_DISC", approve_msgs[0])
        self.assertIn("systemd-run", approve_msgs[0])

    @patch("rip.notify")
    def test_subprocess_failure_includes_resume(self, mock_notify):
        """subprocess.CalledProcessError with valid state gets --resume as second notification."""
        err = subprocess.CalledProcessError(1, "scp")
        self._run_main_with_error(err)

        msgs = [c[0][1] for c in mock_notify.call_args_list]
        resume_msgs = [m for m in msgs if "--resume" in m]
        self.assertEqual(
            len(resume_msgs), 1,
            f"expected exactly one --resume notification, got: {msgs}",
        )
        self.assertIn("TEST_DISC", resume_msgs[0])
        self.assertIn("systemd-run", resume_msgs[0])

    @patch("rip.notify")
    def test_verification_fail_with_null_fix_includes_approve(self, mock_notify):
        """VerificationError from Claude verdict=fail with no fix_command gets --approve."""
        err = VerificationError(
            "wrong episodes",
            recommendation="check title frame scan",
            fix_command=None,
        )
        self._run_main_with_error(err)

        msgs = [c[0][1] for c in mock_notify.call_args_list]
        approve_msgs = [m for m in msgs if "--approve" in m]
        self.assertEqual(
            len(approve_msgs), 1,
            f"expected exactly one --approve notification, got: {msgs}",
        )
        self.assertIn("TEST_DISC", approve_msgs[0])


if __name__ == "__main__":
    unittest.main()
