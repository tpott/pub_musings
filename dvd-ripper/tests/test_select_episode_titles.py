"""Tests for rip.select_episode_titles."""

import unittest

from titles import parse_makemkv_info, select_episode_titles
from tests.test_data import AVATAR_DISC_INFO, MOVIE_DISC_INFO


class TestSelectEpisodeTitles(unittest.TestCase):

    def test_avatar_selects_eight_episodes(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        episodes = select_episode_titles(titles)
        self.assertEqual(len(episodes), 8)

    def test_avatar_episode_ids(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        episodes = select_episode_titles(titles)
        self.assertEqual([e["id"] for e in episodes], list(range(8)))

    def test_excludes_play_all_and_extras(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        ep_ids = {e["id"] for e in select_episode_titles(titles)}
        self.assertNotIn(8, ep_ids)   # 3:07:12 play-all
        self.assertNotIn(9, ep_ids)   # 0:04:38 extra
        self.assertNotIn(18, ep_ids)  # 0:03:59 extra

    def test_excludes_bumper_duplicates(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        ep_ids = {e["id"] for e in select_episode_titles(titles)}
        for dup_id in range(10, 18):
            self.assertNotIn(dup_id, ep_ids)

    def test_episodes_are_single_segment(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        for ep in select_episode_titles(titles):
            self.assertEqual(ep["segment_count"], 1)

    def test_sorted_by_segment_number(self):
        """Episodes should be sorted by segment number (m2ts stream ID),
        not by MakeMKV title ID, since title IDs can be scrambled."""
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        episodes = select_episode_titles(titles)
        segments = [int(e["segments"]) for e in episodes]
        self.assertEqual(segments, sorted(segments))

    def test_scrambled_title_ids_sorted_by_segment(self):
        """Title IDs may not match episode order; segment number wins."""
        titles = [
            {"id": 0, "duration_secs": 1400, "segment_count": 1,
             "segments": "1087"},
            {"id": 1, "duration_secs": 1420, "segment_count": 1,
             "segments": "1094"},
            {"id": 2, "duration_secs": 1380, "segment_count": 1,
             "segments": "1062"},
        ]
        episodes = select_episode_titles(titles)
        self.assertEqual([e["segments"] for e in episodes],
                         ["1062", "1087", "1094"])

    def test_no_episodes_returns_empty(self):
        titles = parse_makemkv_info(MOVIE_DISC_INFO)
        self.assertEqual(select_episode_titles(titles), [])

    def test_excludes_raw_m2ts_duplicate(self):
        """A raw m2ts whose segment isn't in any bumper title is excluded."""
        titles = [
            # 3 single-segment episodes (from playlists)
            {"id": 0, "duration_secs": 1400, "segment_count": 1,
             "segments": "1087"},
            {"id": 1, "duration_secs": 1420, "segment_count": 1,
             "segments": "1094"},
            {"id": 2, "duration_secs": 1380, "segment_count": 1,
             "segments": "1095"},
            # Bumper versions confirming those 3 episodes
            {"id": 10, "duration_secs": 1450, "segment_count": 3,
             "segments": "1100,1086,1087"},
            {"id": 11, "duration_secs": 1470, "segment_count": 3,
             "segments": "1100,1088,1094"},
            {"id": 12, "duration_secs": 1430, "segment_count": 3,
             "segments": "1100,1089,1095"},
            # Raw m2ts duplicate — segment 5 not in any bumper
            {"id": 18, "duration_secs": 1410, "segment_count": 1,
             "segments": "5"},
        ]
        episodes = select_episode_titles(titles)
        self.assertEqual(len(episodes), 3)
        self.assertNotIn(18, [e["id"] for e in episodes])

    def test_play_all_overrides_segment_order(self):
        """When a play-all title exists, its segment order is used
        instead of sorting by m2ts stream number."""
        titles = [
            # Episodes stored with scrambled segment numbers:
            # segments 1090-1092 are Ch.6-8, segments 1099-1100 are Ch.1-2
            {"id": 0, "duration_secs": 1471, "segment_count": 1,
             "segments": "1090"},
            {"id": 1, "duration_secs": 1473, "segment_count": 1,
             "segments": "1091"},
            {"id": 2, "duration_secs": 1476, "segment_count": 1,
             "segments": "1092"},
            {"id": 3, "duration_secs": 1399, "segment_count": 1,
             "segments": "1099"},
            {"id": 4, "duration_secs": 1400, "segment_count": 1,
             "segments": "1100"},
            # Play-all: correct viewing order (1099,1100 before 1090-1092)
            {"id": 10, "duration_secs": 7219, "segment_count": 7,
             "segments": "1093,1099,1100,1090,1091,1092,1084"},
            # Bumper titles confirming all 5 episodes
            {"id": 20, "duration_secs": 1500, "segment_count": 2,
             "segments": "1093,1090"},
            {"id": 21, "duration_secs": 1500, "segment_count": 2,
             "segments": "1093,1091"},
            {"id": 22, "duration_secs": 1500, "segment_count": 2,
             "segments": "1093,1092"},
            {"id": 23, "duration_secs": 1500, "segment_count": 2,
             "segments": "1093,1099"},
            {"id": 24, "duration_secs": 1500, "segment_count": 2,
             "segments": "1093,1100"},
        ]
        episodes = select_episode_titles(titles)
        # Play-all says 1099,1100 come before 1090,1091,1092
        self.assertEqual([e["segments"] for e in episodes],
                         ["1099", "1100", "1090", "1091", "1092"])

    def test_play_all_not_used_when_single_segment(self):
        """A single-segment play-all (one big stream) can't provide order."""
        titles = [
            {"id": 0, "duration_secs": 1400, "segment_count": 1,
             "segments": "1087"},
            {"id": 1, "duration_secs": 1420, "segment_count": 1,
             "segments": "1094"},
            {"id": 2, "duration_secs": 1380, "segment_count": 1,
             "segments": "1062"},
            # Single-segment play-all — can't determine episode order
            {"id": 8, "duration_secs": 11232, "segment_count": 1,
             "segments": "1060"},
        ]
        episodes = select_episode_titles(titles)
        # Falls back to segment number sort
        self.assertEqual([e["segments"] for e in episodes],
                         ["1062", "1087", "1094"])

    def test_fallback_when_all_multi_segment(self):
        """If no single-segment episodes exist, still return candidates."""
        titles = [
            {"id": 0, "duration_secs": 1400, "segment_count": 2,
             "segments": "100,200"},
            {"id": 1, "duration_secs": 1420, "segment_count": 2,
             "segments": "100,201"},
            {"id": 2, "duration_secs": 1380, "segment_count": 2,
             "segments": "100,202"},
        ]
        episodes = select_episode_titles(titles)
        self.assertEqual(len(episodes), 3)


if __name__ == "__main__":
    unittest.main()
