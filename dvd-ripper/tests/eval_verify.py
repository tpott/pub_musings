"""LLM eval tests for Claude CLI verification, plus deterministic tests
for the movie/show classification sanity checks.

The eval tests invoke the real `claude` CLI and are skipped if it's not
available. They assert on the verdict string (the most stable field);
subsidiary fields are logged but not hard-asserted.

The deterministic test classes (TestCheckIsMovie, TestCheckIsShow,
TestCheckExpectedDuration) run without Claude CLI.
"""

import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from tests.test_data import (
    AVATAR_DISC_INFO,
    BLUEY_DISC_INFO,
    MOVIE_DISC_INFO,
    SHE_RA_DISC_INFO,
)
from titles import parse_makemkv_info, select_episode_titles, select_movie_title
from verify import (
    build_verify_prompt,
    check_episode_count,
    check_expected_duration,
    check_is_movie,
    check_is_show,
    run_claude_verify,
    stage_verify,
)


class TestCheckIsMovie(unittest.TestCase):
    """Deterministic tests for check_is_movie."""

    def test_bluey_misclassified_as_movie_label_warning(self):
        """Bluey disc label 'Bluey_S1_First_Half' has TV indicators → warning."""
        titles = parse_makemkv_info(BLUEY_DISC_INFO)
        state = {
            "disc_label": "Bluey_S1_First_Half",
            "media_type": "movie",
            "titles": titles,
            "plan": {
                "episodes": [{
                    "title_id": 0,
                    "duration_secs": 441,
                    "segments": "2001",
                }],
            },
        }
        issues = check_is_movie(state)
        warnings = [i for i in issues if i["severity"] == "warning"]
        label_warnings = [i for i in warnings if i["type"] == "suspect_movie"]
        self.assertTrue(
            any("TV indicators" in w["detail"] for w in label_warnings),
            f"Expected label warning, got: {label_warnings}",
        )

    def test_bluey_misclassified_as_movie_cluster_warning(self):
        """Bluey disc has 27 similar-duration titles → cluster warning."""
        titles = parse_makemkv_info(BLUEY_DISC_INFO)
        state = {
            "disc_label": "Bluey_S1_First_Half",
            "media_type": "movie",
            "titles": titles,
            "plan": {
                "episodes": [{
                    "title_id": 0,
                    "duration_secs": 441,
                    "segments": "2001",
                }],
            },
        }
        issues = check_is_movie(state)
        cluster_warnings = [i for i in issues
                            if "similar-duration" in i.get("detail", "")]
        self.assertEqual(len(cluster_warnings), 1,
                         f"Expected 1 cluster warning, got: {cluster_warnings}")

    def test_real_movie_no_warning(self):
        """Ender's Game (real movie) → no warnings."""
        titles = parse_makemkv_info(MOVIE_DISC_INFO)
        state = {
            "disc_label": "Enders_Game",
            "media_type": "movie",
            "titles": titles,
            "plan": {
                "episodes": [{
                    "title_id": 0,
                    "duration_secs": 6888,
                    "segments": "1000",
                }],
            },
        }
        issues = check_is_movie(state)
        self.assertEqual(issues, [])

    def test_tv_media_type_skipped(self):
        """check_is_movie returns [] for TV media type."""
        state = {"media_type": "tv", "disc_label": "Bluey_S1_First_Half",
                 "titles": []}
        self.assertEqual(check_is_movie(state), [])

    def test_label_with_season(self):
        """Disc label with 'Season' triggers warning."""
        state = {
            "disc_label": "Show_Season_2",
            "media_type": "movie",
            "titles": [],
            "plan": {"episodes": [{"title_id": 0, "duration_secs": 6000}]},
        }
        issues = check_is_movie(state)
        self.assertTrue(
            any("TV indicators" in i["detail"] for i in issues),
            f"Expected label warning for 'Season', got: {issues}",
        )

    def test_label_with_book(self):
        """Disc label with 'Book' triggers warning."""
        state = {
            "disc_label": "Avatar_Book_1",
            "media_type": "movie",
            "titles": [],
            "plan": {"episodes": [{"title_id": 0, "duration_secs": 6000}]},
        }
        issues = check_is_movie(state)
        self.assertTrue(
            any("TV indicators" in i["detail"] for i in issues),
            f"Expected label warning for 'Book', got: {issues}",
        )

    def test_label_with_vol(self):
        """Disc label with 'Vol' triggers warning."""
        state = {
            "disc_label": "Show_Vol_3",
            "media_type": "movie",
            "titles": [],
            "plan": {"episodes": [{"title_id": 0, "duration_secs": 6000}]},
        }
        issues = check_is_movie(state)
        self.assertTrue(
            any("TV indicators" in i["detail"] for i in issues),
            f"Expected label warning for 'Vol', got: {issues}",
        )

    def test_label_with_disc_number(self):
        """Disc label with 'Disc_2' triggers warning."""
        state = {
            "disc_label": "Show_Disc_2",
            "media_type": "movie",
            "titles": [],
            "plan": {"episodes": [{"title_id": 0, "duration_secs": 6000}]},
        }
        issues = check_is_movie(state)
        self.assertTrue(
            any("TV indicators" in i["detail"] for i in issues),
            f"Expected label warning for 'Disc_2', got: {issues}",
        )


class TestCheckIsShow(unittest.TestCase):
    """Deterministic tests for check_is_show."""

    def test_single_ep_single_title_warns(self):
        """1 episode selected, 1 episode-length title on disc → warning."""
        state = {
            "media_type": "tv",
            "titles": [
                {"id": 0, "duration_secs": 2400, "segment_count": 1,
                 "segments": "100"},
            ],
            "plan": {
                "episodes": [{"title_id": 0, "duration_secs": 2400}],
            },
        }
        issues = check_is_show(state)
        self.assertEqual(len(issues), 1)
        self.assertEqual(issues[0]["type"], "suspect_show")
        self.assertEqual(issues[0]["severity"], "warning")

    def test_single_ep_multiple_titles_no_warning(self):
        """1 episode selected but 5 episode-length titles → no warning."""
        state = {
            "media_type": "tv",
            "titles": [
                {"id": i, "duration_secs": 1400, "segment_count": 1,
                 "segments": str(i)}
                for i in range(5)
            ],
            "plan": {
                "episodes": [{"title_id": 0, "duration_secs": 1400}],
            },
        }
        issues = check_is_show(state)
        self.assertEqual(issues, [])

    def test_multiple_episodes_no_warning(self):
        """Multiple episodes selected → no warning."""
        state = {
            "media_type": "tv",
            "titles": [
                {"id": i, "duration_secs": 1400, "segment_count": 1,
                 "segments": str(i)}
                for i in range(5)
            ],
            "plan": {
                "episodes": [
                    {"title_id": 0, "duration_secs": 1400},
                    {"title_id": 1, "duration_secs": 1400},
                ],
            },
        }
        issues = check_is_show(state)
        self.assertEqual(issues, [])

    def test_movie_media_type_skipped(self):
        """check_is_show returns [] for movie media type."""
        state = {"media_type": "movie", "titles": [],
                 "plan": {"episodes": [{"title_id": 0}]}}
        self.assertEqual(check_is_show(state), [])


class TestCheckExpectedDuration(unittest.TestCase):
    """Deterministic tests for check_expected_duration."""

    def test_bluey_short_movie_warning(self):
        """7-minute 'movie' → short_movie warning."""
        titles = parse_makemkv_info(BLUEY_DISC_INFO)
        state = {
            "media_type": "movie",
            "titles": titles,
            "plan": {
                "episodes": [{
                    "title_id": 0,
                    "duration_secs": 441,
                    "segments": "2001",
                }],
            },
        }
        issues = check_expected_duration(state)
        short = [i for i in issues if i["type"] == "short_movie"]
        self.assertEqual(len(short), 1,
                         f"Expected short_movie warning, got: {issues}")
        self.assertIn("7m", short[0]["detail"])

    def test_normal_movie_no_warning(self):
        """1h55m movie → no warnings."""
        titles = parse_makemkv_info(MOVIE_DISC_INFO)
        movie = select_movie_title(titles)
        state = {
            "media_type": "movie",
            "titles": titles,
            "plan": {
                "episodes": [{
                    "title_id": movie["id"],
                    "duration_secs": movie["duration_secs"],
                    "segments": movie["segments"],
                }],
            },
        }
        issues = check_expected_duration(state)
        self.assertEqual(issues, [])

    def test_longer_title_exists_warning(self):
        """Selected short title when a 2h title exists → warning."""
        titles = [
            {"id": 0, "duration_secs": 600, "segment_count": 1,
             "segments": "100"},
            {"id": 1, "duration_secs": 7200, "segment_count": 1,
             "segments": "200"},
        ]
        state = {
            "media_type": "movie",
            "titles": titles,
            "plan": {
                "episodes": [{
                    "title_id": 0,
                    "duration_secs": 600,
                    "segments": "100",
                }],
            },
        }
        issues = check_expected_duration(state)
        longer = [i for i in issues if i["type"] == "longer_title_exists"]
        self.assertEqual(len(longer), 1)
        self.assertIn("120m", longer[0]["detail"])

    def test_no_much_longer_titles_no_warning(self):
        """Selected longest title, short extras → no longer_title_exists."""
        titles = [
            {"id": 0, "duration_secs": 7200, "segment_count": 1,
             "segments": "100"},
            {"id": 1, "duration_secs": 300, "segment_count": 1,
             "segments": "200"},
        ]
        state = {
            "media_type": "movie",
            "titles": titles,
            "plan": {
                "episodes": [{
                    "title_id": 0,
                    "duration_secs": 7200,
                    "segments": "100",
                }],
            },
        }
        issues = check_expected_duration(state)
        longer = [i for i in issues if i["type"] == "longer_title_exists"]
        self.assertEqual(longer, [])

    def test_tv_media_type_skipped(self):
        """check_expected_duration returns [] for TV media type."""
        state = {"media_type": "tv", "titles": [],
                 "plan": {"episodes": [{"title_id": 0, "duration_secs": 441}]}}
        self.assertEqual(check_expected_duration(state), [])

    def test_no_episodes_no_warning(self):
        """Empty episodes list → no warnings."""
        state = {"media_type": "movie", "titles": [], "plan": {"episodes": []}}
        self.assertEqual(check_expected_duration(state), [])


class TestStageVerifyWarningsTriggerClaude(unittest.TestCase):
    """Verify that warnings (not just errors) trigger Claude invocation."""

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        import shutil as _shutil
        _shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_warnings_trigger_claude(self):
        """Classification warnings should invoke Claude for analysis."""
        titles = parse_makemkv_info(BLUEY_DISC_INFO)
        mp4 = Path(self.tmpdir) / "Bluey.mp4"
        mp4.write_text("fake content")
        state = {
            "disc_label": "Bluey_S1_First_Half",
            "media_type": "movie",
            "titles": titles,
            "plan": {
                "episodes": [{
                    "title_id": 0,
                    "duration_secs": 441,
                    "segments": "2001",
                    "ep_name": "Bluey_S1_First_Half",
                    "mp4_path": str(mp4),
                }],
            },
            "scan_disc": {"makemkv_info": BLUEY_DISC_INFO},
        }
        conf = {"VERIFY_ENABLED": "true", "VERIFY_CLAUDE_ALWAYS": "false"}
        fake_verdict = {
            "verdict": "warn",
            "confidence": 0.8,
            "issues": [],
            "recommendation": "This looks like a TV show",
            "fix_command": None,
        }
        with patch("verify.claude.is_available", return_value=True), \
             patch("verify.run_claude_verify", return_value=fake_verdict):
            result = stage_verify(conf, state)

        # Claude should have been invoked because of warnings
        self.assertIn("claude_verdict", result["verification"])
        self.assertEqual(
            result["verification"]["claude_verdict"]["verdict"], "warn")

    def test_no_issues_still_triggers_claude_default(self):
        """With default VERIFY_CLAUDE_ALWAYS=true, Claude runs even with no issues."""
        titles = parse_makemkv_info(MOVIE_DISC_INFO)
        movie = select_movie_title(titles)
        mp4 = Path(self.tmpdir) / "Enders_Game.mp4"
        mp4.write_text("fake content")
        state = {
            "disc_label": "Enders_Game",
            "media_type": "movie",
            "titles": titles,
            "plan": {
                "episodes": [{
                    "title_id": movie["id"],
                    "duration_secs": movie["duration_secs"],
                    "segments": movie["segments"],
                    "ep_name": "Enders Game",
                    "mp4_path": str(mp4),
                }],
            },
            "scan_disc": {"makemkv_info": MOVIE_DISC_INFO},
        }
        # No VERIFY_CLAUDE_ALWAYS set — should default to "true"
        conf = {"VERIFY_ENABLED": "true"}
        fake_verdict = {
            "verdict": "pass", "confidence": 0.95,
            "issues": [], "recommendation": "", "fix_command": None,
        }
        with patch("verify.claude.is_available", return_value=True), \
             patch("verify.run_claude_verify", return_value=fake_verdict):
            result = stage_verify(conf, state)

        self.assertIn("claude_verdict", result["verification"])


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
        self.assertIn("--force", fix_cmd,
                      f"fix_command should contain --force, got: {fix_cmd}")

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


    def test_eval_title_frame_scanner_invoked(self):
        """For TV shows with mp4 files, Claude should invoke /title-frame-scanner."""
        state = self._make_avatar_state()
        issues = check_episode_count(state)

        # Create a temp dir with small real mp4 files so the skill has
        # something to scan.
        with tempfile.TemporaryDirectory() as tmpdir:
            output_dir = Path(tmpdir)
            episodes = state["plan"]["episodes"]
            for ep in episodes:
                mp4_name = f"{ep['ep_name']}.mp4"
                mp4_path = output_dir / mp4_name
                ep["mp4_path"] = str(mp4_path)
                # Generate a 2-second silent black video
                subprocess.run(
                    [
                        "ffmpeg", "-y", "-f", "lavfi", "-i",
                        "color=c=black:s=320x240:d=2",
                        "-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono",
                        "-t", "2", "-c:v", "libx264", "-c:a", "aac",
                        "-shortest", str(mp4_path),
                    ],
                    capture_output=True,
                )

            state["plan"]["output_dir"] = str(output_dir)
            conf = {"RIP_DIR": str(output_dir)}
            prompt = build_verify_prompt(state, issues, conf)

            result = run_claude_verify(prompt, conf)

            print(f"\n  Claude verdict: {json.dumps(result, indent=2)}")
            tfc = result.get("title_frame_check")
            self.assertIsNotNone(
                tfc,
                f"Expected title_frame_check in response, got keys: "
                f"{list(result.keys())}",
            )
            self.assertTrue(
                tfc.get("attempted"),
                f"Expected title_frame_check.attempted=true, got: {tfc}",
            )


if __name__ == "__main__":
    unittest.main()
