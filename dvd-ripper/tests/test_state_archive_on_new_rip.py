"""Tests that starting a new rip never clobbers an existing state file.

When a fresh rip is started for a disc whose .state/<label>.json already
exists, the previous state must be archived to .state/history/ rather than
overwritten. This applies whether or not --force is used.
"""

import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import MagicMock, patch

import rip


class TestStateArchiveOnNewRip(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.disc_label = "MY_DISC"
        self.state_dir = Path(self.tmpdir) / ".state"
        self.state_dir.mkdir(parents=True, exist_ok=True)
        self.existing_path = self.state_dir / f"{self.disc_label}.json"
        self.existing_payload = {
            "version": 1,
            "status": "failed",
            "disc_label": self.disc_label,
            "created_at": "2026-04-24T10:00:00",
            "updated_at": "2026-04-24T10:05:00",
            "marker": "ORIGINAL_STATE_DO_NOT_LOSE",
            "stages": {},
            "plan": {},
            "verification": {"issues": []},
        }
        self.existing_path.write_text(json.dumps(self.existing_payload))

        (Path(self.tmpdir) / "rip.conf").write_text(
            f'RIP_DIR="{self.tmpdir}"\n'
            'OPENCLAW_BIN="/usr/bin/openclaw"\n'
            'OPENCLAW_TARGET="@user:matrix.org"\n'
        )

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _run_main_with_force(self):
        """Invoke rip.main() with --force and a stubbed pipeline.

        The pipeline is patched so we don't actually rip; we only want to
        observe what happens to the state files at startup.
        """
        blkid_result = MagicMock()
        blkid_result.stdout = self.disc_label + "\n"

        # run_pipeline is patched to raise immediately — we only care about
        # the pre-pipeline state-file handling, not the success-path
        # archive_state() call that fires after a successful pipeline.
        def fake_run_pipeline(stages, conf, state, save_fn):
            save_fn(state)
            raise RuntimeError("stubbed pipeline failure")

        with patch.object(rip, "__file__", str(Path(self.tmpdir) / "rip.py")), \
             patch("sys.argv", ["rip.py", "--force"]), \
             patch("rip.run_pipeline", side_effect=fake_run_pipeline), \
             patch("rip.analyze_error", return_value={"recommendation": "stub"}), \
             patch("rip.notify"), \
             patch("rip.subprocess") as mock_subprocess_mod:
            mock_subprocess_mod.run.return_value = blkid_result
            mock_subprocess_mod.CalledProcessError = subprocess.CalledProcessError
            try:
                rip.main()
            except SystemExit:
                pass

    def test_force_preserves_old_state_in_history(self):
        """--force must move the existing state to history, not overwrite."""
        self._run_main_with_force()

        history_dir = self.state_dir / "history"
        self.assertTrue(
            history_dir.exists(),
            f"history dir should exist after fresh rip, contents={list(self.state_dir.iterdir())}",
        )
        history_files = list(history_dir.glob(f"{self.disc_label}-*.json"))
        self.assertEqual(
            len(history_files), 1,
            f"expected 1 archived file in {history_dir}, found {history_files}",
        )

        with open(history_files[0]) as f:
            archived = json.load(f)
        self.assertEqual(
            archived.get("marker"), "ORIGINAL_STATE_DO_NOT_LOSE",
            "archived file should contain the ORIGINAL state payload, "
            "not the new one — the old state must not be clobbered",
        )

    def test_force_writes_fresh_state_at_label_path(self):
        """After --force, a fresh state file should exist at the label path
        (and it must NOT contain the original marker)."""
        self._run_main_with_force()

        self.assertTrue(
            self.existing_path.exists(),
            "a fresh state file should exist at the label path after the rip starts",
        )
        with open(self.existing_path) as f:
            current = json.load(f)
        self.assertNotEqual(
            current.get("marker"), "ORIGINAL_STATE_DO_NOT_LOSE",
            "current state file must be a fresh one, not the old payload",
        )
        self.assertEqual(current.get("disc_label"), self.disc_label)


if __name__ == "__main__":
    unittest.main()
