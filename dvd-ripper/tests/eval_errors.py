"""LLM eval tests for Claude CLI error analysis.

These tests invoke the real `claude` CLI and are skipped if it's not available.
Each scenario reproduces a real failure we've seen in production.
"""

import json
import re
import shutil
import tempfile
import unittest
from pathlib import Path

from error_analysis import analyze_error


@unittest.skipUnless(shutil.which("claude"), "claude CLI not available")
class TestEvalErrors(unittest.TestCase):

    def test_overwrite_suggests_next_season_and_disc(self):
        """Reproduces the 2026-04-11 ZORGON_TALES failure.

        Disc label "ZORGON_TALES" has no season/disc markers, so parse_disc_label
        defaults to season=1, disc=1. Season 01 is already complete (E01-E13)
        and Season 02 is in progress (E01-E07), so the correct fix is
        SEASON=2 DISC=2 — the next disc of the ongoing season.
        """
        with tempfile.TemporaryDirectory() as tmpdir:
            rip_dir = Path(tmpdir)
            show_dir = rip_dir / "TV" / "ZORGON TALES"
            s1 = show_dir / "Season 01"
            s2 = show_dir / "Season 02"
            s1.mkdir(parents=True)
            s2.mkdir(parents=True)
            # Season 01 complete: E01-E13
            for i in range(1, 14):
                (s1 / f"ZORGON TALES S01E{i:02d}.mp4").write_bytes(b"x" * 500_000_000)
            # Season 02 partial: E01-E07 (disc 1 ripped, disc 2 pending)
            for i in range(1, 8):
                (s2 / f"ZORGON TALES S02E{i:02d}.mp4").write_bytes(b"x" * 500_000_000)

            exc = RuntimeError(
                "Would overwrite 1 existing files: ZORGON TALES S01E01.mp4. "
                "Use --force to override, or set SEASON=/DISC= to target "
                "the correct season and disc."
            )

            state = {
                "disc_label": "ZORGON_TALES",
                "media_type": "tv",
                "plan": {
                    "show_name": "ZORGON TALES",
                    "season": 1,
                    "disc": 1,
                    "output_dir": str(s1),
                    "episodes": [
                        {
                            "title_id": 0,
                            "ep_name": "ZORGON TALES S01E01",
                            "duration_secs": 1400,
                            "segments": "1,2,3",
                        },
                    ],
                },
            }
            conf = {
                "RIP_DIR": str(rip_dir),
                "ERROR_CLAUDE_ENABLED": "true",
            }

            result = analyze_error(conf, state, exc)
            print(f"\n  Claude result: {json.dumps(result, indent=2)}")

            fix_cmd = result.get("fix_command") or ""
            self.assertTrue(
                fix_cmd,
                f"expected fix_command, got: {result}",
            )
            self.assertIn(
                "SEASON=2", fix_cmd,
                f"expected SEASON=2 (Season 01 complete, Season 02 partial), "
                f"got: {fix_cmd}",
            )
            # DISC must be >=2 to bypass the overwrite guard. The exact value
            # doesn't matter functionally: compute_episode_start() counts
            # existing files in the output dir for any disc>1, so DISC=2..N
            # all produce the correct ep_start=E08. Claude may reasonably pick
            # 2 (naive "next disc") or 3+ (reasoning about episodes per disc).
            m = re.search(r'DISC=(\d+)', fix_cmd)
            self.assertIsNotNone(
                m, f"expected DISC=N in fix_command, got: {fix_cmd}",
            )
            self.assertGreaterEqual(
                int(m.group(1)), 2,
                f"expected DISC>=2 (DISC=1 would re-overwrite Season 01), "
                f"got: {fix_cmd}",
            )


if __name__ == "__main__":
    unittest.main()
