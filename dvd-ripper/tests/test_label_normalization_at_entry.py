"""Tests that disc label normalization is applied at pipeline entry points."""

import unittest
from unittest.mock import MagicMock, patch

import rip


class TestLabelNormalizationAtEntry(unittest.TestCase):
    def test_stage_scan_disc_normalizes_label(self):
        """stage_scan_disc stores normalized disc label in state."""
        blkid_result = MagicMock()
        blkid_result.stdout = "Legend_of_Korra_Book_3_Disc_1\n"
        makemkv_result = MagicMock()
        makemkv_result.stdout = ""

        def fake_subprocess_run(cmd, *args, **kwargs):
            if cmd[0] == "blkid":
                return blkid_result
            return makemkv_result

        with patch("subprocess.run", side_effect=fake_subprocess_run), \
             patch("rip.parse_makemkv_info", return_value=[]), \
             patch("rip.detect_media_type", return_value="movie"):
            result = rip.stage_scan_disc({}, {})

        self.assertEqual(result["disc_label"], "Legend_Of_Korra_Book_3_Disc_1")

    def test_main_fresh_run_normalizes_label(self):
        """_read_disc_label returns a normalized label."""
        mock_result = MagicMock()
        mock_result.stdout = "Legend_of_Korra_Book_3_Disc_1\n"
        with patch("subprocess.run", return_value=mock_result):
            label = rip._read_disc_label()
        self.assertEqual(label, "Legend_Of_Korra_Book_3_Disc_1")


if __name__ == "__main__":
    unittest.main()
