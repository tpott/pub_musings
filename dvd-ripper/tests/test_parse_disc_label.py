"""Tests for rip.parse_disc_label."""

import unittest

from disc import parse_disc_label


class TestParseDiscLabel(unittest.TestCase):

    def test_movie_label(self):
        result = parse_disc_label("ENDERS_GAME", media_type="movie")
        self.assertEqual(result, {"movie_name": "ENDERS_GAME"})

    def test_avatar_label(self):
        result = parse_disc_label("Avatar_Book_1_Disc_1", media_type="tv")
        self.assertEqual(result["show_name"], "Avatar")
        self.assertEqual(result["season"], 1)
        self.assertEqual(result["disc"], 1)

    def test_standard_season_disc_label(self):
        result = parse_disc_label("BREAKING_BAD_S3_D2", media_type="tv")
        self.assertEqual(result["show_name"], "BREAKING BAD")
        self.assertEqual(result["season"], 3)
        self.assertEqual(result["disc"], 2)

    def test_season_word_label(self):
        result = parse_disc_label("THE_OFFICE_Season2_Disc3", media_type="tv")
        self.assertEqual(result["show_name"], "THE OFFICE")
        self.assertEqual(result["season"], 2)
        self.assertEqual(result["disc"], 3)

    def test_no_season_defaults_to_1(self):
        result = parse_disc_label("SOME_SHOW_D1", media_type="tv")
        self.assertEqual(result["season"], 1)
        self.assertEqual(result["disc"], 1)

    def test_no_disc_defaults_to_1(self):
        result = parse_disc_label("SOME_SHOW_S2", media_type="tv")
        self.assertEqual(result["season"], 2)
        self.assertEqual(result["disc"], 1)

    def test_no_season_or_disc(self):
        result = parse_disc_label("SOME_SHOW", media_type="tv")
        self.assertEqual(result["show_name"], "SOME SHOW")
        self.assertEqual(result["season"], 1)
        self.assertEqual(result["disc"], 1)


if __name__ == "__main__":
    unittest.main()
