"""Tests for the --approve CLI option."""

import copy
import shutil
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime
from pathlib import Path
from unittest.mock import MagicMock, patch

import rip
from state import approve_state, new_state, resolve_state_arg, save_state


class TestApproveState(unittest.TestCase):
    """Unit tests for approve_state() pure function."""

    def _make_failed_state(self):
        state = new_state("/tmp/rip", "TEST_DISC")
        state["status"] = "failed"
        state["stages"]["verify"] = {"status": "failed"}
        state["verification"] = {
            "issues": [{"type": "count_mismatch", "severity": "warning", "detail": "14 != 7"}],
            "claude_verdict": {"verdict": "warn"},
        }
        return state

    def test_marks_verify_complete_with_approved_flag(self):
        state = self._make_failed_state()
        approve_state(state)
        verify = state["stages"]["verify"]
        self.assertEqual(verify["status"], "complete")
        self.assertIs(verify["approved"], True)
        self.assertIn("approved_at", verify)
        self.assertIn("completed_at", verify)
        datetime.fromisoformat(verify["approved_at"])
        datetime.fromisoformat(verify["completed_at"])

    def test_resets_top_level_status_to_running(self):
        state = self._make_failed_state()
        self.assertEqual(state["status"], "failed")
        approve_state(state)
        self.assertEqual(state["status"], "running")

    def test_preserves_verification_issues(self):
        state = self._make_failed_state()
        original_issues = copy.deepcopy(state["verification"]["issues"])
        original_verdict = copy.deepcopy(state["verification"]["claude_verdict"])
        approve_state(state)
        self.assertEqual(state["verification"]["issues"], original_issues)
        self.assertEqual(state["verification"]["claude_verdict"], original_verdict)

    def test_idempotent(self):
        state = self._make_failed_state()
        approve_state(state)
        approve_state(state)
        verify = state["stages"]["verify"]
        self.assertEqual(verify["status"], "complete")
        self.assertIs(verify["approved"], True)


class TestApproveLoadState(unittest.TestCase):
    """Unit tests for resolve_state_arg()."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_load_by_label_missing_raises(self):
        with self.assertRaises(RuntimeError) as ctx:
            resolve_state_arg(self.tmpdir, "NONEXISTENT")
        msg = str(ctx.exception)
        self.assertIn("NONEXISTENT", msg)
        self.assertIn("No state file", msg)

    def test_load_by_label_present(self):
        st = new_state(self.tmpdir, "MY_DISC")
        save_state(st)
        loaded = resolve_state_arg(self.tmpdir, "MY_DISC")
        self.assertEqual(loaded["disc_label"], "MY_DISC")

    def test_load_by_absolute_path(self):
        st = new_state(self.tmpdir, "MY_DISC")
        save_state(st)
        path = st["_state_path"]
        loaded = resolve_state_arg(self.tmpdir, path)
        self.assertEqual(loaded["disc_label"], "MY_DISC")

    @patch("state.subprocess.run")
    def test_load_by_blkid_autodetect(self, mock_run):
        class FakeResult:
            stdout = "AUTO_DISC\n"
        mock_run.return_value = FakeResult()
        st = new_state(self.tmpdir, "AUTO_DISC")
        save_state(st)
        loaded = resolve_state_arg(self.tmpdir, True)
        self.assertEqual(loaded["disc_label"], "AUTO_DISC")

    @patch("state.subprocess.run")
    def test_load_blkid_no_disc(self, mock_run):
        mock_run.side_effect = subprocess.CalledProcessError(
            returncode=2, cmd=["blkid"], stderr=""
        )
        with self.assertRaises(RuntimeError) as ctx:
            resolve_state_arg(self.tmpdir, True)
        self.assertIn("No disc", str(ctx.exception))

    @patch("state.subprocess.run")
    def test_load_blkid_disc_no_state(self, mock_run):
        class FakeResult:
            stdout = "MISSING_DISC\n"
        mock_run.return_value = FakeResult()
        with self.assertRaises(RuntimeError) as ctx:
            resolve_state_arg(self.tmpdir, True)
        self.assertIn("No state file", str(ctx.exception))


class TestApproveCLI(unittest.TestCase):
    """Tests for --approve argument parsing."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        (Path(self.tmpdir) / "rip.conf").write_text(
            f'RIP_DIR="{self.tmpdir}"\n'
            'OPENCLAW_BIN="/usr/bin/openclaw"\n'
            'OPENCLAW_TARGET="@user:matrix.org"\n'
        )

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_approve_alone_parses(self):
        """--approve LABEL should call the resolver with "LABEL"."""
        fake_state = new_state(self.tmpdir, "LABEL")
        fake_state["plan"] = {"notify_msg": "ok", "episodes": []}

        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--approve", "LABEL"]), \
             patch("rip.resolve_state_arg", return_value=fake_state) as mock_resolver, \
             patch("rip.save_state"), \
             patch("rip.run_pipeline", return_value=fake_state), \
             patch("rip.archive_state"), \
             patch("rip.notify"):
            rip.main()

        mock_resolver.assert_called_once_with(self.tmpdir, "LABEL")

    def test_approve_no_arg_triggers_autodetect(self):
        """--approve with no arg should call resolver with True."""
        fake_state = new_state(self.tmpdir, "AUTO_DISC")
        fake_state["plan"] = {"notify_msg": "ok", "episodes": []}

        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--approve"]), \
             patch("rip.resolve_state_arg", return_value=fake_state) as mock_resolver, \
             patch("rip.save_state"), \
             patch("rip.run_pipeline", return_value=fake_state), \
             patch("rip.archive_state"), \
             patch("rip.notify"):
            rip.main()

        mock_resolver.assert_called_once_with(self.tmpdir, True)

    def test_approve_and_resume_mutually_exclusive(self):
        """--approve and --resume together should fail with SystemExit(2)."""
        with patch("sys.argv", ["rip.py", "--approve", "X", "--resume", "Y"]):
            with self.assertRaises(SystemExit) as ctx:
                rip.main()
        self.assertEqual(ctx.exception.code, 2)


class TestApproveMainHappyPath(unittest.TestCase):
    """Integration tests for --approve happy path."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        (Path(self.tmpdir) / "rip.conf").write_text(
            f'RIP_DIR="{self.tmpdir}"\n'
            'OPENCLAW_BIN="/usr/bin/openclaw"\n'
            'OPENCLAW_TARGET="@user:matrix.org"\n'
        )
        self.disc_label = "TEST_DISC"
        state = new_state(self.tmpdir, self.disc_label)
        state["status"] = "failed"
        state["stages"] = {
            "scan_disc": {"status": "complete"},
            "plan": {"status": "complete"},
            "rip": {"status": "complete"},
            "transcode": {"status": "complete"},
            "verify": {"status": "failed"},
        }
        state["_jobs"] = []
        state["plan"] = {"notify_msg": "Test complete", "episodes": []}
        state["verification"] = {"issues": []}
        save_state(state)

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_approve_skips_verify_runs_remaining(self):
        """State passed to run_pipeline must have verify marked complete+approved."""
        captured = {}

        def fake_run_pipeline(stages, conf, state, save_fn):
            captured["state"] = copy.deepcopy(state)
            return state

        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--approve", self.disc_label]), \
             patch("rip.run_pipeline", side_effect=fake_run_pipeline), \
             patch("rip.archive_state") as mock_archive, \
             patch("rip.notify"):
            rip.main()

        verify = captured["state"]["stages"]["verify"]
        self.assertEqual(verify["status"], "complete")
        self.assertIs(verify["approved"], True)
        mock_archive.assert_called_once()

    def test_approve_state_saved_before_run_pipeline(self):
        """save_state must be called with the approved state before run_pipeline."""
        call_order = []

        def record_save(state):
            call_order.append("save_state")

        def record_run_pipeline(stages, conf, state, save_fn):
            call_order.append("run_pipeline")
            return state

        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--approve", self.disc_label]), \
             patch("rip.save_state", side_effect=record_save), \
             patch("rip.run_pipeline", side_effect=record_run_pipeline), \
             patch("rip.archive_state"), \
             patch("rip.notify"):
            rip.main()

        self.assertIn("save_state", call_order)
        self.assertIn("run_pipeline", call_order)
        self.assertLess(
            call_order.index("save_state"),
            call_order.index("run_pipeline"),
        )

    def test_approve_with_failed_transcode_still_fails_pipeline(self):
        """When transcode is also failed, --approve marks verify done but the
        pipeline still fails on the underlying transcode error."""
        state = new_state(self.tmpdir, "TRANS_FAIL")
        state["status"] = "failed"
        state["stages"] = {
            "scan_disc": {"status": "complete"},
            "plan": {"status": "complete"},
            "rip": {"status": "complete"},
            "transcode": {"status": "failed"},
            "verify": {"status": "failed"},
        }
        state["plan"] = {"notify_msg": "Test", "episodes": []}
        state["_jobs"] = []
        state["verification"] = {"issues": []}
        save_state(state)

        captured_state = {}

        def fake_run_pipeline(stages, conf, st, save_fn):
            captured_state.update(copy.deepcopy(st))
            raise RuntimeError("transcode failed again")

        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--approve", "TRANS_FAIL"]), \
             patch("rip.run_pipeline", side_effect=fake_run_pipeline), \
             patch("rip.analyze_error", return_value={"recommendation": "retry transcode"}), \
             patch("rip.notify"):
            with self.assertRaises(SystemExit) as ctx:
                rip.main()

        self.assertEqual(ctx.exception.code, 1)
        self.assertEqual(
            captured_state.get("stages", {}).get("verify", {}).get("status"),
            "complete",
        )


class TestStageFinalizeApprovedNote(unittest.TestCase):
    """Tests for the approved note appended by stage_finalize."""

    def _make_conf(self):
        return {
            "AUTO_EJECT": "false",
            "OPENCLAW_BIN": "/usr/bin/openclaw",
            "OPENCLAW_TARGET": "@user:matrix.org",
        }

    def _make_state(self, approved=False):
        return {
            "plan": {"notify_msg": "Rip complete: TestShow ready in Jellyfin"},
            "stages": {
                "verify": {"status": "complete", "approved": approved},
            },
            "verification": {"issues": []},
        }

    @patch("rip.notify")
    def test_finalize_appends_approved_note_when_approved(self, mock_notify):
        state = self._make_state(approved=True)
        rip.stage_finalize(self._make_conf(), state)
        msg = mock_notify.call_args[0][1]
        self.assertIn("(verification manually approved)", msg)

    @patch("rip.notify")
    def test_finalize_no_note_when_not_approved(self, mock_notify):
        state = self._make_state(approved=False)
        rip.stage_finalize(self._make_conf(), state)
        msg = mock_notify.call_args[0][1]
        self.assertNotIn("verification manually approved", msg)


class TestApproveMainMissingState(unittest.TestCase):
    """Integration test for --approve with a missing state file."""

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
    @patch("rip.analyze_error", return_value={})
    def test_approve_missing_state_notifies_and_exits(
        self, mock_analyze, mock_notify
    ):
        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--approve", "NONEXISTENT"]):
            with self.assertRaises(SystemExit) as ctx:
                rip.main()

        self.assertEqual(ctx.exception.code, 1)
        mock_notify.assert_called()
        msg = mock_notify.call_args[0][1]
        self.assertIn("FAILED", msg)
        self.assertIn("NONEXISTENT", msg)


if __name__ == "__main__":
    unittest.main()
