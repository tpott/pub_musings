"""Tests for rip.detect_media_type."""

import unittest

from titles import detect_media_type, parse_makemkv_info
from tests.test_data import AVATAR_DISC_INFO, MOVIE_DISC_INFO


class TestDetectMediaType(unittest.TestCase):

    def test_avatar_is_tv(self):
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        self.assertEqual(detect_media_type(titles), "tv")

    def test_movie_is_movie(self):
        titles = parse_makemkv_info(MOVIE_DISC_INFO)
        self.assertEqual(detect_media_type(titles), "movie")

    def test_two_similar_titles_is_movie(self):
        """Fewer than 3 episode-length titles should be movie."""
        titles = [
            {"id": 0, "duration_secs": 1400, "segment_count": 1, "segments": "1"},
            {"id": 1, "duration_secs": 1420, "segment_count": 1, "segments": "2"},
        ]
        self.assertEqual(detect_media_type(titles), "movie")

    def test_three_similar_titles_is_tv(self):
        """3 clustered episode-length titles triggers TV detection."""
        titles = [
            {"id": 0, "duration_secs": 1400, "segment_count": 1, "segments": "1"},
            {"id": 1, "duration_secs": 1420, "segment_count": 1, "segments": "2"},
            {"id": 2, "duration_secs": 1380, "segment_count": 1, "segments": "3"},
        ]
        self.assertEqual(detect_media_type(titles), "tv")

    def test_empty_titles(self):
        self.assertEqual(detect_media_type([]), "movie")

    def test_scattered_durations_is_movie(self):
        """Titles in episode range but not clustered should be movie."""
        titles = [
            {"id": 0, "duration_secs": 900, "segment_count": 1, "segments": "1"},
            {"id": 1, "duration_secs": 2400, "segment_count": 1, "segments": "2"},
            {"id": 2, "duration_secs": 3800, "segment_count": 1, "segments": "3"},
        ]
        self.assertEqual(detect_media_type(titles), "movie")


if __name__ == "__main__":
    unittest.main()
