"""Tests for post-rip verification checks."""

import tempfile
import unittest
from pathlib import Path

from tests.test_data import AVATAR_DISC_INFO, MOVIE_DISC_INFO, SHE_RA_DISC_INFO
from titles import parse_makemkv_info, select_episode_titles
from verify import (
    VerificationError,
    check_duration_anomaly,
    check_episode_count,
    check_file_integrity,
    stage_verify,
)


class TestCheckEpisodeCount(unittest.TestCase):
    def test_avatar_count_matches(self):
        """Avatar disc: 8 episode-length titles, 8 selected → pass."""
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        state = {
            "titles": [t for t in titles],
            "plan": {
                "episodes": [{"title_id": i} for i in range(8)],
            },
        }
        issues = check_episode_count(state)
        errors = [i for i in issues if i["severity"] == "error"]
        self.assertEqual(errors, [])

    def test_count_mismatch_detected(self):
        """7 episode-length titles on disc but only 2 selected → error."""
        # Simulate a disc with 7 episode-length titles
        titles = [
            {"id": i, "duration_secs": 1400, "segment_count": 2,
             "segments": f"seg{i}", "name": "", "filename": ""}
            for i in range(7)
        ]
        state = {
            "titles": titles,
            "plan": {
                "episodes": [{"title_id": 0}, {"title_id": 1}],
            },
        }
        issues = check_episode_count(state)
        errors = [i for i in issues if i["severity"] == "error"]
        self.assertEqual(len(errors), 1)
        self.assertEqual(errors[0]["type"], "count_mismatch")
        self.assertIn("7", errors[0]["detail"])
        self.assertIn("2", errors[0]["detail"])

    def test_movie_no_count_check(self):
        """Movie discs should not trigger count mismatch."""
        titles = parse_makemkv_info(MOVIE_DISC_INFO)
        state = {
            "titles": titles,
            "media_type": "movie",
            "plan": {
                "episodes": [{"title_id": 0}],
            },
        }
        issues = check_episode_count(state)
        self.assertEqual(issues, [])

    def test_she_ra_count_mismatch(self):
        """She-Ra: 7 episode-length disc titles, only 2 selected → error."""
        titles = parse_makemkv_info(SHE_RA_DISC_INFO)
        # Simulate what select_episode_titles would return (2 titles due to dedup bug)
        selected = select_episode_titles(titles)
        self.assertEqual(len(selected), 2, "Precondition: dedup bug gives 2")

        state = {
            "titles": titles,
            "plan": {
                "episodes": [
                    {"title_id": t["id"], "duration_secs": t["duration_secs"]}
                    for t in selected
                ],
            },
        }
        issues = check_episode_count(state)
        errors = [i for i in issues if i["severity"] == "error"]
        self.assertEqual(len(errors), 1)
        self.assertEqual(errors[0]["type"], "count_mismatch")
        self.assertIn("7", errors[0]["detail"])
        self.assertIn("2", errors[0]["detail"])

    def test_fewer_disc_titles_than_selected_is_ok(self):
        """If disc has 3 episode-length but we selected 3, that's fine."""
        titles = [
            {"id": i, "duration_secs": 1400, "segment_count": 1,
             "segments": str(i), "name": "", "filename": ""}
            for i in range(3)
        ]
        state = {
            "titles": titles,
            "plan": {
                "episodes": [{"title_id": i} for i in range(3)],
            },
        }
        issues = check_episode_count(state)
        errors = [i for i in issues if i["severity"] == "error"]
        self.assertEqual(errors, [])


class TestCheckDurationAnomaly(unittest.TestCase):
    def test_all_similar_durations(self):
        """All episodes ~23 min → no anomaly."""
        episodes = [
            {"title_id": i, "duration_secs": d, "mp4_path": f"/tmp/ep{i}.mp4"}
            for i, d in enumerate([1419, 1300, 1356, 1341, 1366, 1384, 1387, 1344])
        ]
        state = {"plan": {"episodes": episodes}}
        issues = check_duration_anomaly(state)
        self.assertEqual(issues, [])

    def test_double_duration_flagged(self):
        """One 46m episode among 23m episodes → combined episode warning."""
        episodes = [
            {"title_id": 0, "duration_secs": 1400, "mp4_path": "/tmp/ep0.mp4"},
            {"title_id": 1, "duration_secs": 1380, "mp4_path": "/tmp/ep1.mp4"},
            {"title_id": 2, "duration_secs": 2760, "mp4_path": "/tmp/ep2.mp4"},
            {"title_id": 3, "duration_secs": 1350, "mp4_path": "/tmp/ep3.mp4"},
        ]
        state = {"plan": {"episodes": episodes}}
        issues = check_duration_anomaly(state)
        self.assertTrue(len(issues) >= 1)
        combined = [i for i in issues if i["type"] == "combined_episode"]
        self.assertEqual(len(combined), 1)
        self.assertIn("2", str(combined[0]["detail"]))

    def test_large_deviation_flagged(self):
        """One episode >40% off median → duration anomaly."""
        episodes = [
            {"title_id": 0, "duration_secs": 1400, "mp4_path": "/tmp/ep0.mp4"},
            {"title_id": 1, "duration_secs": 1380, "mp4_path": "/tmp/ep1.mp4"},
            {"title_id": 2, "duration_secs": 700, "mp4_path": "/tmp/ep2.mp4"},
            {"title_id": 3, "duration_secs": 1350, "mp4_path": "/tmp/ep3.mp4"},
        ]
        state = {"plan": {"episodes": episodes}}
        issues = check_duration_anomaly(state)
        anomalies = [i for i in issues if i["type"] == "duration_anomaly"]
        self.assertTrue(len(anomalies) >= 1)

    def test_single_episode_no_anomaly(self):
        """Single episode can't have anomaly (no median comparison)."""
        episodes = [
            {"title_id": 0, "duration_secs": 6888, "mp4_path": "/tmp/ep0.mp4"},
        ]
        state = {"plan": {"episodes": episodes}}
        issues = check_duration_anomaly(state)
        self.assertEqual(issues, [])


class TestCheckFileIntegrity(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_all_files_exist(self):
        """All planned mp4 files exist → pass."""
        for i in range(3):
            path = Path(self.tmpdir) / f"ep{i}.mp4"
            path.write_text("fake")
        episodes = [
            {"title_id": i, "mp4_path": str(Path(self.tmpdir) / f"ep{i}.mp4")}
            for i in range(3)
        ]
        state = {"plan": {"episodes": episodes}}
        issues = check_file_integrity(state)
        self.assertEqual(issues, [])

    def test_missing_file_detected(self):
        """One planned mp4 missing → error."""
        Path(self.tmpdir, "ep0.mp4").write_text("fake")
        Path(self.tmpdir, "ep1.mp4").write_text("fake")
        # ep2.mp4 missing
        episodes = [
            {"title_id": i, "mp4_path": str(Path(self.tmpdir) / f"ep{i}.mp4")}
            for i in range(3)
        ]
        state = {"plan": {"episodes": episodes}}
        issues = check_file_integrity(state)
        errors = [i for i in issues if i["severity"] == "error"]
        self.assertEqual(len(errors), 1)
        self.assertEqual(errors[0]["type"], "missing_episode")

    def test_empty_file_detected(self):
        """Zero-byte mp4 → error."""
        Path(self.tmpdir, "ep0.mp4").write_text("fake")
        Path(self.tmpdir, "ep1.mp4").write_bytes(b"")  # empty
        episodes = [
            {"title_id": i, "mp4_path": str(Path(self.tmpdir) / f"ep{i}.mp4")}
            for i in range(2)
        ]
        state = {"plan": {"episodes": episodes}}
        issues = check_file_integrity(state)
        errors = [i for i in issues if i["severity"] == "error"]
        self.assertEqual(len(errors), 1)


class TestStageVerify(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_clean_rip_passes(self):
        """No issues → stage completes normally."""
        titles = parse_makemkv_info(AVATAR_DISC_INFO)
        episode_titles = [t for t in titles if 900 <= t["duration_secs"] <= 3900
                          and t["segment_count"] == 1]
        episodes = []
        for t in episode_titles:
            mp4 = Path(self.tmpdir) / f"ep{t['id']}.mp4"
            mp4.write_text("fake content")
            episodes.append({
                "title_id": t["id"],
                "duration_secs": t["duration_secs"],
                "mp4_path": str(mp4),
            })
        state = {
            "titles": titles,
            "plan": {"episodes": episodes},
            "verification": {"issues": []},
        }
        conf = {"VERIFY_ENABLED": "true"}
        result = stage_verify(conf, state)
        errors = [i for i in result["verification"]["issues"]
                  if i["severity"] == "error"]
        self.assertEqual(errors, [])

    def test_verification_disabled_skips(self):
        """VERIFY_ENABLED=false → skip all checks."""
        state = {
            "titles": [],
            "plan": {"episodes": []},
            "verification": {"issues": []},
        }
        conf = {"VERIFY_ENABLED": "false"}
        result = stage_verify(conf, state)
        self.assertEqual(result["verification"]["issues"], [])

    def test_count_mismatch_raises(self):
        """Episode count mismatch should raise VerificationError."""
        titles = [
            {"id": i, "duration_secs": 1400, "segment_count": 1,
             "segments": str(i), "name": "", "filename": ""}
            for i in range(7)
        ]
        state = {
            "titles": titles,
            "plan": {
                "episodes": [
                    {"title_id": 0, "duration_secs": 1400, "mp4_path": "/dev/null"},
                    {"title_id": 1, "duration_secs": 1400, "mp4_path": "/dev/null"},
                ],
            },
            "verification": {"issues": []},
        }
        conf = {"VERIFY_ENABLED": "true"}
        with self.assertRaises(VerificationError) as ctx:
            stage_verify(conf, state)
        self.assertIn("7", str(ctx.exception))
        self.assertIn("2", str(ctx.exception))


if __name__ == "__main__":
    unittest.main()
