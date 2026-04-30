"""Tests for disc.normalize_disc_label."""

import unittest

from disc import normalize_disc_label


class TestNormalizeDiscLabel(unittest.TestCase):
    def test_lowercase_connector_normalized(self):
        self.assertEqual(
            normalize_disc_label("Legend_of_Korra_Book_3_Disc_1"),
            "Legend_Of_Korra_Book_3_Disc_1",
        )

    def test_already_normalized_passthrough(self):
        label = "Legend_Of_Korra_Book_3_Disc_1"
        self.assertEqual(normalize_disc_label(label), label)

    def test_all_caps_normalized(self):
        self.assertEqual(
            normalize_disc_label("BREAKING_BAD_S3_D2"),
            "Breaking_Bad_S3_D2",
        )

    def test_empty_string_passthrough(self):
        self.assertEqual(normalize_disc_label(""), "")

    def test_no_underscore_label(self):
        self.assertEqual(normalize_disc_label("INCEPTION"), "INCEPTION")


if __name__ == "__main__":
    unittest.main()
