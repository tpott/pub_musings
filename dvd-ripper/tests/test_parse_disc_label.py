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
        self.assertEqual(result["show_name"], "Breaking Bad")
        self.assertEqual(result["season"], 3)
        self.assertEqual(result["disc"], 2)

    def test_season_word_label(self):
        result = parse_disc_label("THE_OFFICE_Season2_Disc3", media_type="tv")
        self.assertEqual(result["show_name"], "The Office")
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
        self.assertEqual(result["show_name"], "Some Show")
        self.assertEqual(result["season"], 1)
        self.assertEqual(result["disc"], 1)

    def test_case_insensitive_legend_of_korra(self):
        """Disc label 'Legend_of_Korra' should match SHOW_NAME='Legend Of Korra'.

        Regression test: disc labeled 'Legend_of_Korra_Book_1_Disc_1' was
        rejected because the parsed show_name 'Legend of Korra' (lowercase
        'of') didn't match SHOW_NAME='Legend Of Korra' (title case).
        """
        result = parse_disc_label(
            "Legend_of_Korra_Book_1_Disc_1", media_type="tv"
        )
        self.assertEqual(result["show_name"], "Legend Of Korra")
        self.assertEqual(result["season"], 1)
        self.assertEqual(result["disc"], 1)

    def test_all_caps_label_normalized_to_title(self):
        """ALL_CAPS disc labels should be title-cased."""
        result = parse_disc_label("LEGEND_OF_KORRA_S2_D1", media_type="tv")
        self.assertEqual(result["show_name"], "Legend Of Korra")
        self.assertEqual(result["season"], 2)
        self.assertEqual(result["disc"], 1)

    def test_mixed_case_label_normalized(self):
        """Mixed-case disc labels should be title-cased consistently."""
        result = parse_disc_label("breaking_BAD_S1_D1", media_type="tv")
        self.assertEqual(result["show_name"], "Breaking Bad")


if __name__ == "__main__":
    unittest.main()
