"""Tests that the failure notification suggests --approve on VerificationError
with verdict=warn and no fix_command."""

import shutil
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import rip
from state import new_state, save_state
from verify import VerificationError


class TestVerificationErrorApproveNotify(unittest.TestCase):
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

    def _run_with_pipeline_error(self, err, verdict):
        """Run main() with --resume TEST_DISC; run_pipeline raises err after
        writing verdict into state["verification"]["claude_verdict"]."""
        def fake_run_pipeline(stages, conf, state, save_fn):
            state["verification"] = {
                "issues": [{"type": "count_mismatch", "severity": "error", "detail": "x"}],
                "claude_verdict": {"verdict": verdict},
            }
            raise err

        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--resume", "TEST_DISC"]), \
             patch("rip.run_pipeline", side_effect=fake_run_pipeline):
            with self.assertRaises(SystemExit):
                rip.main()

    @patch("rip.notify")
    def test_warn_no_fix_suggests_approve(self, mock_notify):
        """VerificationError with verdict=warn and fix_command=None should
        trigger a second notification with the --approve command."""
        err = VerificationError("count mismatch", recommendation="rip is fine", fix_command=None)
        self._run_with_pipeline_error(err, verdict="warn")

        msgs = [c[0][1] for c in mock_notify.call_args_list]
        approve_msgs = [m for m in msgs if "--approve" in m]
        self.assertEqual(len(approve_msgs), 1, f"expected exactly one --approve notification, got: {msgs}")
        self.assertIn("TEST_DISC", approve_msgs[0])
        self.assertIn("systemd-run", approve_msgs[0])

    @patch("rip.notify")
    def test_fail_verdict_no_fix_suggests_approve(self, mock_notify):
        """VerificationError with verdict=fail and no fix_command should suggest --approve.

        Even a fail verdict with no specific fix command is recoverable — the user
        can --approve to bypass verify and inspect the files manually.
        """
        err = VerificationError("episodes missing", recommendation="re-rip", fix_command=None)
        self._run_with_pipeline_error(err, verdict="fail")

        msgs = [c[0][1] for c in mock_notify.call_args_list]
        approve_msgs = [m for m in msgs if "--approve" in m]
        self.assertEqual(len(approve_msgs), 1, f"expected one --approve notification, got: {msgs}")

    @patch("rip.notify")
    def test_warn_with_fix_command_no_approve_suggestion(self, mock_notify):
        """When fix_command is present, send it instead — do not also suggest --approve."""
        err = VerificationError(
            "count mismatch",
            recommendation="re-rip with TITLES=",
            fix_command="systemd-run --user -- /path/rip.py --force",
        )
        self._run_with_pipeline_error(err, verdict="warn")

        msgs = [c[0][1] for c in mock_notify.call_args_list]
        self.assertFalse(any("--approve" in m for m in msgs), f"unexpected --approve in: {msgs}")
        # fix_command itself should have been sent
        self.assertTrue(any("systemd-run" in m and "--force" in m for m in msgs))

    @patch("rip.notify")
    @patch("rip.analyze_error", return_value={"recommendation": "check logs"})
    def test_non_verification_error_no_approve_suggestion(
        self, mock_analyze, mock_notify
    ):
        """A plain RuntimeError must not trigger --approve, even if state
        has a warn verdict."""
        err = RuntimeError("unexpected failure")
        self._run_with_pipeline_error(err, verdict="warn")

        msgs = [c[0][1] for c in mock_notify.call_args_list]
        self.assertFalse(any("--approve" in m for m in msgs), f"unexpected --approve in: {msgs}")


if __name__ == "__main__":
    unittest.main()
