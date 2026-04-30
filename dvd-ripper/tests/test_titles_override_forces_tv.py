"""TITLES env var override should force TV plan even if media_type=='movie'."""

import os
import shutil
import tempfile
import unittest
from unittest.mock import patch

from rip import stage_plan


def _episode_titles_with_misclassification():
    """Build titles that cause detect_media_type to return 'movie'.

    7 episodes (~1420s each) plus a play-all (~9940s) and a few extras
    that fall in the in-range duration window. The point of this test
    isn't the classification logic — we just craft a state where
    media_type was set to 'movie' (whether legitimately or buggily)
    and verify the operator's TITLES override routes through TV.
    """
    titles = []
    for i in range(7):
        titles.append({
            "id": i,
            "duration_secs": 1420,
            "segment_count": 1,
            "segments": str(i + 1),
            "name": "",
            "filename": f"D1_t{i:02d}.mkv",
        })
    titles.append({
        "id": 7,
        "duration_secs": 1420 * 7,
        "segment_count": 7,
        "segments": "1,2,3,4,5,6,7",
        "name": "",
        "filename": "D1_t07.mkv",
    })
    return titles


class TestTitlesOverrideForcesTV(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.conf = {
            "RIP_DIR": self.tmpdir,
            "BACKUP_DEST": "/backup/Movies",
            "BACKUP_DEST_TV": "/backup/TV",
        }

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_titles_override_routes_misclassified_disc_to_tv(self):
        state = {
            "disc_label": "MY_SHOW_S2_D1",
            "media_type": "movie",
            "titles": _episode_titles_with_misclassification(),
            "stages": {},
            "plan": {},
            "verification": {"issues": []},
        }
        env = {
            "TITLES": "0,1,2,3,4,5,6",
            "SHOW_NAME": "My Show",
            "SEASON": "2",
            "DISC": "1",
        }
        with patch.dict(os.environ, env, clear=False):
            result = stage_plan(self.conf, state)

        plan = result["plan"]
        self.assertEqual(len(plan["episodes"]), 7)
        self.assertTrue(
            plan["output_dir"].endswith("TV/My Show/Season 02"),
            f"unexpected output_dir: {plan['output_dir']}",
        )
        self.assertEqual(plan["episodes"][0]["ep_name"], "My Show S02E01")
        self.assertEqual(plan["episodes"][6]["ep_name"], "My Show S02E07")
        self.assertEqual(
            [e["title_id"] for e in plan["episodes"]],
            [0, 1, 2, 3, 4, 5, 6],
        )


if __name__ == "__main__":
    unittest.main()
