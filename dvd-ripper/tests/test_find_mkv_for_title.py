"""Tests for transcode.find_mkv_for_title."""

import os
import tempfile
import unittest
from pathlib import Path

from transcode import find_mkv_for_title


class TestFindMkvForTitle(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.addCleanup(lambda: os.rmdir(self.tmpdir) if not os.listdir(self.tmpdir) else None)

    def _touch(self, name):
        """Create an empty file in the temp directory."""
        p = Path(self.tmpdir) / name
        p.touch()
        self.addCleanup(lambda p=p: p.unlink(missing_ok=True))
        return p

    def test_stale_file_from_previous_disc_not_matched(self):
        """A leftover MKV from Disc 2 must not match when ripping Disc 3 title 0."""
        # Stale file from a previous disc rip
        self._touch("Avatar- The Last Airbender Book Two- Earth Disc 2_t00.mkv")
        # Fresh file from the current disc rip
        disc3_path = self._touch(
            "Avatar- The Last Airbender Book Two- Earth Disc 3_t00.mkv")

        result = find_mkv_for_title(
            self.tmpdir,
            title_id=0,
            filename="Avatar- The Last Airbender Book Two- Earth Disc 3_t00.mkv",
        )
        self.assertEqual(result, disc3_path)

    def test_exact_filename_match(self):
        """When the expected filename exists, return it directly."""
        expected = self._touch("Movie_Title_t02.mkv")

        result = find_mkv_for_title(
            self.tmpdir,
            title_id=2,
            filename="Movie_Title_t02.mkv",
        )
        self.assertEqual(result, expected)

    def test_returns_none_when_file_missing(self):
        """Return None when the expected file doesn't exist."""
        result = find_mkv_for_title(
            self.tmpdir,
            title_id=5,
            filename="Nonexistent_t05.mkv",
        )
        self.assertIsNone(result)


if __name__ == "__main__":
    unittest.main()
