"""Tests for cross-season episode context in build_verify_prompt."""

import shutil
import tempfile
import unittest
from pathlib import Path

from verify import build_verify_prompt


class TestBuildVerifyPromptCrossSeasonContext(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.rip_dir = self.tmpdir

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _make_state(self, show_name, season, episodes, output_dir):
        return {
            "disc_label": "SHE_RA",
            "media_type": "tv",
            "plan": {
                "show_name": show_name,
                "season": season,
                "output_dir": output_dir,
                "episodes": episodes,
            },
            "scan_disc": {"makemkv_info": "TINFO:0,9,0,\"0:23:00\""},
        }

    def _create_mp4s(self, show_name, season, ep_names):
        """Create fake mp4 files in the expected directory structure."""
        season_dir = (
            Path(self.rip_dir) / "TV" / show_name / f"Season {season:02d}"
        )
        season_dir.mkdir(parents=True, exist_ok=True)
        for name in ep_names:
            (season_dir / name).write_bytes(b"x" * 1000)

    def test_includes_existing_episodes_from_other_seasons(self):
        """Prompt should list episodes from all seasons, not just current."""
        # Season 1 already ripped
        self._create_mp4s("SHE RA", 1, [
            "SHE RA S01E01.mp4", "SHE RA S01E02.mp4", "SHE RA S01E03.mp4",
        ])
        # Currently ripping season 2
        s2_dir = Path(self.rip_dir) / "TV" / "SHE RA" / "Season 02"
        s2_dir.mkdir(parents=True, exist_ok=True)

        state = self._make_state("SHE RA", 2, [], str(s2_dir))
        conf = {"RIP_DIR": self.rip_dir}
        prompt = build_verify_prompt(state, [], conf)

        self.assertIn("All existing episodes for this show", prompt)
        self.assertIn("Season 01", prompt)
        self.assertIn("SHE RA S01E01.mp4", prompt)
        self.assertIn("SHE RA S01E02.mp4", prompt)
        self.assertIn("SHE RA S01E03.mp4", prompt)

    def test_includes_file_sizes(self):
        """File sizes should be included to help detect duplicates."""
        self._create_mp4s("SHE RA", 1, ["SHE RA S01E01.mp4"])

        s1_dir = Path(self.rip_dir) / "TV" / "SHE RA" / "Season 01"
        state = self._make_state("SHE RA", 1, [], str(s1_dir))
        conf = {"RIP_DIR": self.rip_dir}
        prompt = build_verify_prompt(state, [], conf)

        self.assertIn("1000 bytes", prompt)

    def test_multiple_seasons_listed(self):
        """Both Season 01 and Season 02 files should appear."""
        self._create_mp4s("SHE RA", 1, [
            "SHE RA S01E01.mp4", "SHE RA S01E02.mp4",
        ])
        self._create_mp4s("SHE RA", 2, [
            "SHE RA S02E01.mp4",
        ])

        s2_dir = Path(self.rip_dir) / "TV" / "SHE RA" / "Season 02"
        state = self._make_state("SHE RA", 2, [], str(s2_dir))
        conf = {"RIP_DIR": self.rip_dir}
        prompt = build_verify_prompt(state, [], conf)

        self.assertIn("Season 01", prompt)
        self.assertIn("Season 02", prompt)
        self.assertIn("SHE RA S01E01.mp4", prompt)
        self.assertIn("SHE RA S02E01.mp4", prompt)

    def test_no_show_dir_no_crash(self):
        """If the show directory doesn't exist yet, prompt should not crash."""
        state = self._make_state("NEW SHOW", 1, [], "/tmp/nonexistent")
        conf = {"RIP_DIR": self.rip_dir}
        prompt = build_verify_prompt(state, [], conf)
        self.assertNotIn("All existing episodes", prompt)

    def test_no_show_name_no_crash(self):
        """If show_name is missing from plan, should not crash."""
        state = {
            "disc_label": "DISC",
            "media_type": "tv",
            "plan": {"episodes": []},
            "scan_disc": {"makemkv_info": ""},
        }
        conf = {"RIP_DIR": self.rip_dir}
        prompt = build_verify_prompt(state, [], conf)
        self.assertNotIn("All existing episodes", prompt)


if __name__ == "__main__":
    unittest.main()
