"""Tests for rip.parse_makemkv_info."""

import unittest

from rip import parse_makemkv_info
from tests.test_data import AVATAR_DISC_INFO, MOVIE_DISC_INFO


class TestParseMakemkvInfo(unittest.TestCase):

    def test_avatar_disc_title_count(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        self.assertEqual(len(titles), 19)

    def test_title_fields(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        t0 = titles[0]
        self.assertEqual(t0["id"], 0)
        self.assertEqual(t0["name"],
                         "Avatar: The Last Airbender Book One: Water Disc 1")
        self.assertEqual(t0["duration_secs"], 23 * 60 + 39)
        self.assertEqual(t0["segment_count"], 1)
        self.assertEqual(t0["segments"], "1062")
        self.assertEqual(t0["filename"], "Avatar_t00.mkv")

    def test_play_all_duration(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        self.assertEqual(titles[8]["duration_secs"], 3 * 3600 + 7 * 60 + 12)

    def test_multi_segment_title(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        t10 = titles[10]
        self.assertEqual(t10["segment_count"], 3)
        self.assertEqual(t10["segments"], "1100,1086,1087")

    def test_two_segment_title(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        t17 = titles[17]
        self.assertEqual(t17["segment_count"], 2)
        self.assertEqual(t17["segments"], "1100,1062")

    def test_movie_disc(self):
        titles = parse_makemkv_info(MOVIE_DISC_INFO)
        self.assertEqual(len(titles), 3)
        self.assertEqual(titles[0]["duration_secs"], 1 * 3600 + 54 * 60 + 48)
        self.assertEqual(titles[0]["name"], "Ender's Game")

    def test_empty_output(self):
        self.assertEqual(parse_makemkv_info(""), [])

    def test_sorted_by_title_id(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        ids = [t["id"] for t in titles]
        self.assertEqual(ids, list(range(19)))


if __name__ == "__main__":
    unittest.main()
