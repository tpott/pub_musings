"""Tests for rip.select_episode_titles."""

import unittest

from rip import parse_makemkv_info, select_episode_titles
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

    def test_sorted_by_title_id(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        episodes = select_episode_titles(titles)
        ids = [e["id"] for e in episodes]
        self.assertEqual(ids, sorted(ids))

    def test_no_episodes_returns_empty(self):
        titles = parse_makemkv_info(MOVIE_DISC_INFO)
        self.assertEqual(select_episode_titles(titles), [])

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
