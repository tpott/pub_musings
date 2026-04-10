"""Tests for the pipeline DAG runner."""

import tempfile
import unittest

from pipeline import run_pipeline
from state import new_state, save_state


def _make_stage(name, side_effect=None):
    """Create a stub stage function that logs its name."""
    def stage_fn(conf, state):
        state.setdefault("_log", []).append(name)
        if side_effect:
            side_effect()
        return state
    return stage_fn


class TestRunPipeline(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.state = new_state(self.tmpdir, "TEST_DISC")
        self.conf = {"RIP_DIR": self.tmpdir}
        self.saves = []

        def mock_save(state):
            self.saves.append(dict(state.get("stages", {})))

        self.mock_save = mock_save

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_stages_run_in_order(self):
        stages = [
            ("a", [], _make_stage("a")),
            ("b", ["a"], _make_stage("b")),
            ("c", ["b"], _make_stage("c")),
        ]
        state = run_pipeline(stages, self.conf, self.state, self.mock_save)
        self.assertEqual(state["_log"], ["a", "b", "c"])

    def test_complete_stages_are_skipped(self):
        self.state["stages"]["a"] = {"status": "complete"}
        stages = [
            ("a", [], _make_stage("a")),
            ("b", ["a"], _make_stage("b")),
        ]
        state = run_pipeline(stages, self.conf, self.state, self.mock_save)
        self.assertEqual(state["_log"], ["b"])

    def test_failed_stage_halts_pipeline(self):
        def fail():
            raise RuntimeError("stage b failed")

        stages = [
            ("a", [], _make_stage("a")),
            ("b", ["a"], _make_stage("b", side_effect=fail)),
            ("c", ["b"], _make_stage("c")),
        ]
        with self.assertRaises(RuntimeError):
            run_pipeline(stages, self.conf, self.state, self.mock_save)

        self.assertEqual(self.state["_log"], ["a", "b"])
        self.assertEqual(self.state["stages"]["a"]["status"], "complete")
        self.assertEqual(self.state["stages"]["b"]["status"], "failed")
        self.assertNotIn("c", self.state["stages"])

    def test_state_persisted_after_each_stage(self):
        stages = [
            ("a", [], _make_stage("a")),
            ("b", ["a"], _make_stage("b")),
        ]
        run_pipeline(stages, self.conf, self.state, self.mock_save)
        # save_fn called twice per stage (before running + after completing)
        # plus once at the end for overall status
        self.assertGreaterEqual(len(self.saves), 4)

    def test_overall_status_complete_on_success(self):
        stages = [
            ("a", [], _make_stage("a")),
        ]
        state = run_pipeline(stages, self.conf, self.state, self.mock_save)
        self.assertEqual(state["status"], "complete")

    def test_overall_status_failed_on_error(self):
        def fail():
            raise RuntimeError("boom")

        stages = [
            ("a", [], _make_stage("a", side_effect=fail)),
        ]
        with self.assertRaises(RuntimeError):
            run_pipeline(stages, self.conf, self.state, self.mock_save)
        self.assertEqual(self.state["status"], "failed")

    def test_resume_skips_all_complete(self):
        """If all stages are complete, pipeline is a no-op."""
        self.state["stages"]["a"] = {"status": "complete"}
        self.state["stages"]["b"] = {"status": "complete"}
        stages = [
            ("a", [], _make_stage("a")),
            ("b", ["a"], _make_stage("b")),
        ]
        state = run_pipeline(stages, self.conf, self.state, self.mock_save)
        self.assertNotIn("_log", state)
        self.assertEqual(state["status"], "complete")

    def test_resume_from_failed_stage(self):
        """A previously failed stage gets re-run on resume."""
        self.state["stages"]["a"] = {"status": "complete"}
        self.state["stages"]["b"] = {"status": "failed"}
        stages = [
            ("a", [], _make_stage("a")),
            ("b", ["a"], _make_stage("b")),
            ("c", ["b"], _make_stage("c")),
        ]
        state = run_pipeline(stages, self.conf, self.state, self.mock_save)
        self.assertEqual(state["_log"], ["b", "c"])
        self.assertEqual(state["stages"]["b"]["status"], "complete")
        self.assertEqual(state["stages"]["c"]["status"], "complete")

    def test_empty_stages_list(self):
        state = run_pipeline([], self.conf, self.state, self.mock_save)
        self.assertEqual(state["status"], "complete")


if __name__ == "__main__":
    unittest.main()
