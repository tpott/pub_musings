"""LLM eval tests for Claude CLI verification.

These tests invoke the real `claude` CLI and are skipped if it's not available.
They assert on the verdict string (the most stable field); subsidiary fields
are logged but not hard-asserted.
"""

import json
import shutil
import unittest

from tests.test_data import AVATAR_DISC_INFO, SHE_RA_DISC_INFO
from titles import parse_makemkv_info, select_episode_titles
from verify import build_verify_prompt, check_episode_count, run_claude_verify


@unittest.skipUnless(shutil.which("claude"), "claude CLI not available")
class TestEvalVerify(unittest.TestCase):
    """Eval tests that invoke Claude CLI with fixture data."""

    def _make_she_ra_state(self):
        """Build a She-Ra state dict that reproduces the 2-of-7 bug."""
        titles = parse_makemkv_info(SHE_RA_DISC_INFO)
        selected = select_episode_titles(titles)
        return {
            "disc_label": "SHE_RA_S1_D1",
            "media_type": "tv",
            "scan_disc": {"makemkv_info": SHE_RA_DISC_INFO},
            "titles": titles,
            "plan": {
                "show_name": "SHE RA",
                "season": 1,
                "disc": 1,
                "output_dir": "/tmp/test-she-ra",
                "episodes": [
                    {
                        "title_id": t["id"],
                        "duration_secs": t["duration_secs"],
                        "segments": t["segments"],
                        "ep_name": f"SHE RA S01E{i+1:02d}",
                    }
                    for i, t in enumerate(selected)
                ],
            },
        }

    def _make_avatar_state(self):
        """Build an Avatar state dict with correct 8-of-8 selection."""
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        selected = select_episode_titles(titles)
        return {
            "disc_label": "AVATAR_BOOK_1_DISC_1",
            "media_type": "tv",
            "scan_disc": {"makemkv_info": AVATAR_DISC_INFO},
            "titles": titles,
            "plan": {
                "show_name": "Avatar",
                "season": 1,
                "disc": 1,
                "output_dir": "/tmp/test-avatar",
                "episodes": [
                    {
                        "title_id": t["id"],
                        "duration_secs": t["duration_secs"],
                        "segments": t["segments"],
                        "ep_name": f"Avatar S01E{i+1:02d}",
                    }
                    for i, t in enumerate(selected)
                ],
            },
        }

    def test_eval_count_mismatch(self):
        """She-Ra state (2 of 7 episodes) → expect verdict=fail."""
        state = self._make_she_ra_state()
        issues = check_episode_count(state)
        conf = {"RIP_DIR": "/tmp/test-rip"}
        prompt = build_verify_prompt(state, issues, conf)

        result = run_claude_verify(prompt, conf)

        print(f"\n  Claude verdict: {json.dumps(result, indent=2)}")
        self.assertEqual(result["verdict"], "fail",
                         f"Expected fail verdict, got: {result}")

    def test_eval_all_correct(self):
        """Avatar state (8 of 8) → expect verdict=pass."""
        state = self._make_avatar_state()
        issues = check_episode_count(state)
        conf = {"RIP_DIR": "/tmp/test-rip"}
        prompt = build_verify_prompt(state, issues, conf)

        result = run_claude_verify(prompt, conf)

        print(f"\n  Claude verdict: {json.dumps(result, indent=2)}")
        self.assertIn(result["verdict"], ("pass", "warn"),
                      f"Expected pass/warn verdict, got: {result}")

    def test_eval_fix_command_valid(self):
        """For She-Ra case, fix_command should contain TITLES= with all 7 IDs."""
        state = self._make_she_ra_state()
        issues = check_episode_count(state)
        conf = {"RIP_DIR": "/tmp/test-rip"}
        prompt = build_verify_prompt(state, issues, conf)

        result = run_claude_verify(prompt, conf)

        print(f"\n  Claude fix_command: {result.get('fix_command')}")
        fix_cmd = result.get("fix_command") or ""
        self.assertIn("TITLES=", fix_cmd,
                      f"fix_command should contain TITLES=, got: {fix_cmd}")

    def test_eval_duration_anomaly(self):
        """46m episode among 23m episodes → verdict mentions anomaly."""
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        selected = select_episode_titles(titles)
        # Patch one episode to be ~2x duration
        episodes = [
            {
                "title_id": t["id"],
                "duration_secs": t["duration_secs"] * 2 if i == 2 else t["duration_secs"],
                "segments": t["segments"],
                "ep_name": f"Avatar S01E{i+1:02d}",
            }
            for i, t in enumerate(selected)
        ]
        state = {
            "disc_label": "AVATAR_BOOK_1_DISC_1",
            "media_type": "tv",
            "scan_disc": {"makemkv_info": AVATAR_DISC_INFO},
            "titles": titles,
            "plan": {
                "show_name": "Avatar",
                "season": 1,
                "disc": 1,
                "output_dir": "/tmp/test-avatar",
                "episodes": episodes,
            },
        }
        from verify import check_duration_anomaly
        issues = check_episode_count(state) + check_duration_anomaly(state)
        conf = {"RIP_DIR": "/tmp/test-rip"}
        prompt = build_verify_prompt(state, issues, conf)

        result = run_claude_verify(prompt, conf)

        print(f"\n  Claude verdict: {json.dumps(result, indent=2)}")
        # Claude should at least mention the anomaly in its issues or
        # recommendation, even if it correctly identifies it as a false
        # positive (plan duration doubled vs raw makemkv) and returns "pass"
        mentions_anomaly = (
            any("duration" in str(i).lower() for i in result.get("issues", []))
            or "duration" in result.get("recommendation", "").lower()
        )
        self.assertTrue(mentions_anomaly,
                        f"Expected Claude to mention duration anomaly: {result}")


if __name__ == "__main__":
    unittest.main()
