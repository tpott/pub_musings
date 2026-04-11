"""Tests for mp4 overwrite protection in stage_plan."""

import os
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from rip import stage_plan


def _make_tv_state(disc_label, titles):
    """Build a minimal state dict for stage_plan with TV media type."""
    return {
        "disc_label": disc_label,
        "media_type": "tv",
        "titles": titles,
        "stages": {},
        "plan": {},
        "verification": {"issues": []},
    }


def _simple_titles(count):
    """Create N episode-length titles with unique segments."""
    return [
        {
            "id": i,
            "duration_secs": 1400,
            "segment_count": 1,
            "segments": str(i + 1),
            "name": "",
            "filename": f"D1_t{i:02d}.mkv",
        }
        for i in range(count)
    ]


class TestOverwriteProtection(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.conf = {
            "RIP_DIR": self.tmpdir,
            "BACKUP_DEST": "/backup/Movies",
            "BACKUP_DEST_TV": "/backup/TV",
        }
        # Clear env vars that stage_plan reads
        for var in ("SHOW_NAME", "SEASON", "DISC", "TITLES"):
            os.environ.pop(var, None)

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        for var in ("SHOW_NAME", "SEASON", "DISC", "TITLES"):
            os.environ.pop(var, None)

    def test_no_existing_files_succeeds(self):
        """First disc rip with no existing files should succeed."""
        state = _make_tv_state("SHOW_S1_D1", _simple_titles(3))
        result = stage_plan(self.conf, state)
        self.assertEqual(result["plan"]["season"], 1)
        self.assertEqual(len(result["plan"]["episodes"]), 3)

    def test_error_when_overwriting_without_force(self):
        """Should raise RuntimeError when mp4 files would be overwritten."""
        # Rip disc 1 first
        state = _make_tv_state("SHOW_S1_D1", _simple_titles(3))
        result = stage_plan(self.conf, state)
        # Create the mp4 files that disc 1 would have produced
        output_dir = Path(result["plan"]["output_dir"])
        for ep in result["plan"]["episodes"]:
            (output_dir / f"{ep['ep_name']}.mp4").write_text("fake")

        # Now try to plan another disc=1 rip (same season) without --force
        state2 = _make_tv_state("SHOW_S1_D1", _simple_titles(3))
        with self.assertRaises(RuntimeError) as ctx:
            stage_plan(self.conf, state2)
        self.assertIn("Would overwrite", str(ctx.exception))
        self.assertIn("--force", str(ctx.exception))
        self.assertIn("SEASON=", str(ctx.exception))

    def test_warn_when_overwriting_with_force(self):
        """Should warn but proceed when --force is set."""
        conf = {**self.conf, "_force": True}
        # Create existing files
        state = _make_tv_state("SHOW_S1_D1", _simple_titles(3))
        result = stage_plan(conf, state)
        output_dir = Path(result["plan"]["output_dir"])
        for ep in result["plan"]["episodes"]:
            (output_dir / f"{ep['ep_name']}.mp4").write_text("fake")

        # Re-plan with --force should succeed with warning
        state2 = _make_tv_state("SHOW_S1_D1", _simple_titles(3))
        with patch("sys.stderr") as mock_stderr:
            result2 = stage_plan(conf, state2)
        # Should have proceeded and returned a plan
        self.assertEqual(len(result2["plan"]["episodes"]), 3)

    def test_she_ra_label_would_overwrite(self):
        """SHE_RA label defaults to S1D1 — second disc insert would overwrite."""
        state = _make_tv_state("SHE_RA", _simple_titles(7))
        os.environ["SHOW_NAME"] = "SHE RA"
        result = stage_plan(self.conf, state)
        output_dir = Path(result["plan"]["output_dir"])
        for ep in result["plan"]["episodes"]:
            (output_dir / f"{ep['ep_name']}.mp4").write_text("fake")

        # Insert "next disc" — also labeled SHE_RA, also defaults to S1D1
        state2 = _make_tv_state("SHE_RA", _simple_titles(7))
        with self.assertRaises(RuntimeError) as ctx:
            stage_plan(self.conf, state2)
        self.assertIn("Would overwrite", str(ctx.exception))
        self.assertIn("7", str(ctx.exception))

    def test_different_season_no_conflict(self):
        """Setting SEASON=2 should not conflict with Season 01 files."""
        state = _make_tv_state("SHE_RA", _simple_titles(7))
        os.environ["SHOW_NAME"] = "SHE RA"
        result = stage_plan(self.conf, state)
        output_dir = Path(result["plan"]["output_dir"])
        for ep in result["plan"]["episodes"]:
            (output_dir / f"{ep['ep_name']}.mp4").write_text("fake")

        # Now rip season 2 — different output dir, no conflict
        state2 = _make_tv_state("SHE_RA", _simple_titles(7))
        os.environ["SEASON"] = "2"
        result2 = stage_plan(self.conf, state2)
        self.assertEqual(result2["plan"]["season"], 2)
        self.assertIn("Season 02", result2["plan"]["output_dir"])


if __name__ == "__main__":
    unittest.main()
