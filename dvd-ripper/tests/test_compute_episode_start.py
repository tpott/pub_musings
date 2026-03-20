"""Tests for rip.compute_episode_start."""

import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

from rip import compute_episode_start


class TestComputeEpisodeStart(unittest.TestCase):

    def test_disc_1_always_starts_at_1(self):
        with TemporaryDirectory() as d:
            self.assertEqual(compute_episode_start(d, disc=1, ep_count=8), 1)

    def test_disc_1_starts_at_1_even_with_existing_files(self):
        with TemporaryDirectory() as d:
            Path(d, "Show S01E01.mp4").touch()
            Path(d, "Show S01E02.mp4").touch()
            self.assertEqual(compute_episode_start(d, disc=1, ep_count=8), 1)

    def test_disc_2_after_8_episodes(self):
        with TemporaryDirectory() as d:
            for i in range(1, 9):
                Path(d, f"Show S01E{i:02d}.mp4").touch()
            self.assertEqual(compute_episode_start(d, disc=2, ep_count=9), 9)

    def test_disc_3_after_17_episodes(self):
        with TemporaryDirectory() as d:
            for i in range(1, 18):
                Path(d, f"Show S01E{i:02d}.mp4").touch()
            self.assertEqual(compute_episode_start(d, disc=3, ep_count=5), 18)

    def test_disc_2_no_existing_files_raises(self):
        with TemporaryDirectory() as d:
            with self.assertRaises(RuntimeError) as ctx:
                compute_episode_start(d, disc=2, ep_count=8)
            self.assertIn("cannot be ripped before disc 1", str(ctx.exception))

    def test_disc_2_with_gap_raises(self):
        with TemporaryDirectory() as d:
            Path(d, "Show S01E01.mp4").touch()
            Path(d, "Show S01E03.mp4").touch()  # gap: missing E02
            with self.assertRaises(RuntimeError) as ctx:
                compute_episode_start(d, disc=2, ep_count=8)
            self.assertIn("gaps", str(ctx.exception))

    def test_ignores_non_episode_files(self):
        with TemporaryDirectory() as d:
            for i in range(1, 5):
                Path(d, f"Show S01E{i:02d}.mp4").touch()
            Path(d, "some_random_file.mkv").touch()
            Path(d, "Show_behind_the_scenes.mp4").touch()
            self.assertEqual(compute_episode_start(d, disc=2, ep_count=4), 5)


if __name__ == "__main__":
    unittest.main()
