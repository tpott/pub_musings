"""Tests for rip.select_movie_title."""

import unittest

from titles import parse_makemkv_info, select_movie_title
from tests.test_data import AVATAR_DISC_INFO, MOVIE_DISC_INFO


class TestSelectMovieTitle(unittest.TestCase):

    def test_picks_longest(self):
        titles = parse_makemkv_info(MOVIE_DISC_INFO)
        result = select_movie_title(titles)
        self.assertEqual(result["id"], 0)
        self.assertEqual(result["duration_secs"], 1 * 3600 + 54 * 60 + 48)

    def test_avatar_picks_play_all(self):
        """On a TV disc, select_movie_title picks the play-all (wrong for TV)."""
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        self.assertEqual(select_movie_title(titles)["id"], 8)


if __name__ == "__main__":
    unittest.main()
