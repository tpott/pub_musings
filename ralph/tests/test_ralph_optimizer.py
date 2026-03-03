"""Tests for ralph_optimizer.py"""

import json
import tempfile
from pathlib import Path
from unittest import TestCase, main

from ralph.ralph_optimizer import (
    AgentOverhead,
    AgentSession,
    CostAnalyzer,
    CostBreakdown,
    Iteration,
    LateTestRun,
    LogParser,
    Pattern,
    PatternDetector,
    RedundantRead,
    Reporter,
    Session,
    ToolCall,
    _fmt_tokens,
    _summarize_input,
)


class TestFmtTokens(TestCase):
    def test_small(self) -> None:
        self.assertEqual(_fmt_tokens(500), "500")

    def test_thousands(self) -> None:
        self.assertEqual(_fmt_tokens(1500), "1.5K")

    def test_millions(self) -> None:
        self.assertEqual(_fmt_tokens(2_500_000), "2.5M")

    def test_zero(self) -> None:
        self.assertEqual(_fmt_tokens(0), "0")


class TestSummarizeInput(TestCase):
    def test_read(self) -> None:
        tc = ToolCall(name="Read", input={"file_path": "/foo/bar/baz.ts"})
        self.assertEqual(_summarize_input(tc), "baz.ts")

    def test_edit(self) -> None:
        tc = ToolCall(name="Edit", input={"file_path": "/foo/bar/baz.ts"})
        self.assertEqual(_summarize_input(tc), "baz.ts")

    def test_bash(self) -> None:
        tc = ToolCall(name="Bash", input={"command": "git status"})
        self.assertEqual(_summarize_input(tc), "git status")

    def test_bash_long(self) -> None:
        cmd = "a" * 100
        tc = ToolCall(name="Bash", input={"command": cmd})
        result = _summarize_input(tc)
        self.assertTrue(result.endswith("..."))
        self.assertEqual(len(result), 63)

    def test_grep(self) -> None:
        tc = ToolCall(name="Grep", input={"pattern": "import.*foo"})
        self.assertEqual(_summarize_input(tc), '"import.*foo"')

    def test_task(self) -> None:
        tc = ToolCall(name="Task", input={"description": "Read files"})
        self.assertEqual(_summarize_input(tc), "Read files")

    def test_todo(self) -> None:
        tc = ToolCall(name="TodoWrite", input={"todos": []})
        self.assertEqual(_summarize_input(tc), "(todo update)")


class TestLogParser(TestCase):
    def test_cwd_to_runtime_dir(self) -> None:
        """Convert cwd path to runtime directory format."""
        parser = LogParser()
        result = parser._cwd_to_runtime_dir("/home/trevor/pub_musings/peekaboo")
        self.assertEqual(result, "-home-trevor-pub-musings-peekaboo")

    def test_cwd_to_runtime_dir_root(self) -> None:
        """Handle root path."""
        parser = LogParser()
        result = parser._cwd_to_runtime_dir("/")
        self.assertEqual(result, "-")

    def test_parse_ralph_log_extracts_cwd(self) -> None:
        """Parse ralph log and extract cwd for runtime_dir."""
        log_content = (
            "=== Iteration 1/1 === 2026-01-28 12:00:00\n"
            '{"type":"system","subtype":"init","session_id":"abc-123",'
            '"cwd":"/home/user/my_project","model":"opus"}\n'
        )
        with tempfile.NamedTemporaryFile(mode="w", suffix=".log", delete=False) as f:
            f.write(log_content)
            f.flush()
            parser = LogParser()
            parser.parse_ralph_log(Path(f.name))

        self.assertEqual(parser.runtime_dir, "-home-user-my-project")

    def test_parse_ralph_log(self) -> None:
        """Parse a minimal ralph log with one iteration."""
        log_content = (
            "=== Iteration 1/3 === 2026-01-28 12:00:00\n"
            '{"type":"system","subtype":"init","session_id":"abc-123","model":"opus"}\n'
            '{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}\n'
            "Result: All done\n"
        )
        with tempfile.NamedTemporaryFile(mode="w", suffix=".log", delete=False) as f:
            f.write(log_content)
            f.flush()
            parser = LogParser()
            iterations = parser.parse_ralph_log(Path(f.name))

        self.assertEqual(len(iterations), 1)
        self.assertEqual(iterations[0].number, 1)
        self.assertEqual(iterations[0].total, 3)
        self.assertEqual(iterations[0].session_id, "abc-123")
        self.assertEqual(iterations[0].result, "All done")
        self.assertFalse(iterations[0].is_error)

    def test_parse_ralph_log_multiple_iterations(self) -> None:
        """Parse a log with multiple iterations."""
        log_content = (
            "=== Iteration 1/2 === 2026-01-28 12:00:00\n"
            '{"type":"system","subtype":"init","session_id":"sess-1","model":"opus"}\n'
            "Result: Done 1\n"
            "=== Iteration 2/2 === 2026-01-28 13:00:00\n"
            '{"type":"system","subtype":"init","session_id":"sess-2","model":"opus"}\n'
            "Result: Done 2\n"
        )
        with tempfile.NamedTemporaryFile(mode="w", suffix=".log", delete=False) as f:
            f.write(log_content)
            f.flush()
            parser = LogParser()
            iterations = parser.parse_ralph_log(Path(f.name))

        self.assertEqual(len(iterations), 2)
        self.assertEqual(iterations[0].session_id, "sess-1")
        self.assertEqual(iterations[1].session_id, "sess-2")

    def test_parse_ralph_log_no_iterations(self) -> None:
        """Handle log with no iterations."""
        with tempfile.NamedTemporaryFile(mode="w", suffix=".log", delete=False) as f:
            f.write("some random text\n")
            f.flush()
            parser = LogParser()
            iterations = parser.parse_ralph_log(Path(f.name))

        self.assertEqual(len(iterations), 0)

    def test_parse_session_nonexistent(self) -> None:
        """Handle nonexistent session file gracefully."""
        parser = LogParser()
        # Override dir to a temp path
        parser.CLAUDE_PROJECTS_DIR = Path("/tmp/nonexistent-ralph-test")
        session = parser.parse_session("fake-session-id")
        self.assertEqual(session.session_id, "fake-session-id")
        self.assertEqual(session.tool_calls, [])

    def test_parse_session_with_tool_calls(self) -> None:
        """Parse a session file with tool calls."""
        session_data = [
            {
                "type": "assistant",
                "message": {
                    "content": [
                        {
                            "type": "tool_use",
                            "id": "t1",
                            "name": "Read",
                            "input": {"file_path": "/foo/bar.ts"},
                        }
                    ],
                    "usage": {
                        "input_tokens": 100,
                        "cache_creation_input_tokens": 50,
                        "cache_read_input_tokens": 200,
                        "output_tokens": 30,
                    },
                },
                "parent_tool_use_id": None,
                "session_id": "test-session",
            },
            {
                "type": "assistant",
                "message": {
                    "content": [
                        {
                            "type": "tool_use",
                            "id": "t2",
                            "name": "Edit",
                            "input": {
                                "file_path": "/foo/bar.ts",
                                "old_string": "a",
                                "new_string": "b",
                            },
                        }
                    ],
                    "usage": {
                        "input_tokens": 150,
                        "cache_creation_input_tokens": 0,
                        "cache_read_input_tokens": 300,
                        "output_tokens": 20,
                    },
                },
                "parent_tool_use_id": None,
                "session_id": "test-session",
            },
        ]

        with tempfile.TemporaryDirectory() as tmpdir:
            project_dir = Path(tmpdir) / "project"
            project_dir.mkdir()
            session_file = project_dir / "test-session.jsonl"
            with open(session_file, "w") as f:
                for entry in session_data:
                    f.write(json.dumps(entry) + "\n")

            parser = LogParser()
            parser.CLAUDE_PROJECTS_DIR = Path(tmpdir)
            parser.runtime_dir = "project"
            session = parser.parse_session("test-session")

        self.assertEqual(session.session_id, "test-session")
        self.assertEqual(len(session.tool_calls), 2)
        self.assertEqual(session.tool_calls[0].name, "Read")
        self.assertEqual(session.tool_calls[1].name, "Edit")
        self.assertEqual(session.total_input_tokens, 800)  # 100+50+200+150+0+300
        self.assertEqual(session.total_output_tokens, 50)  # 30+20

    def test_parse_session_skips_agent_tool_calls(self) -> None:
        """Tool calls with parent_tool_use_id should be skipped (they're agent calls)."""
        session_data = [
            {
                "type": "assistant",
                "message": {
                    "content": [
                        {
                            "type": "tool_use",
                            "id": "t1",
                            "name": "Task",
                            "input": {"description": "explore"},
                        }
                    ],
                    "usage": {"input_tokens": 100, "output_tokens": 10},
                },
                "parent_tool_use_id": None,
                "session_id": "test-session",
            },
            {
                "type": "assistant",
                "message": {
                    "content": [
                        {
                            "type": "tool_use",
                            "id": "t2",
                            "name": "Read",
                            "input": {"file_path": "/foo.ts"},
                        }
                    ],
                    "usage": {"input_tokens": 50, "output_tokens": 5},
                },
                "parent_tool_use_id": "t1",  # This is an agent's call
                "session_id": "test-session",
            },
        ]

        with tempfile.TemporaryDirectory() as tmpdir:
            project_dir = Path(tmpdir) / "project"
            project_dir.mkdir()
            session_file = project_dir / "test-session.jsonl"
            with open(session_file, "w") as f:
                for entry in session_data:
                    f.write(json.dumps(entry) + "\n")

            parser = LogParser()
            parser.CLAUDE_PROJECTS_DIR = Path(tmpdir)
            parser.runtime_dir = "project"
            session = parser.parse_session("test-session")

        # Only the top-level Task call should be included
        self.assertEqual(len(session.tool_calls), 1)
        self.assertEqual(session.tool_calls[0].name, "Task")


class TestCostAnalyzer(TestCase):
    def test_estimate_cost_empty_session(self) -> None:
        session = Session(session_id="empty")
        analyzer = CostAnalyzer()
        cost = analyzer.estimate_cost(session)
        self.assertEqual(cost.estimated_cost_usd, 0.0)
        self.assertEqual(cost.input_tokens, 0)
        self.assertEqual(cost.output_tokens, 0)

    def test_estimate_cost_with_tokens(self) -> None:
        session = Session(
            session_id="test",
            total_input_tokens=100_000,
            total_output_tokens=10_000,
        )
        analyzer = CostAnalyzer()
        cost = analyzer.estimate_cost(session)
        # 100K * $15/1M + 10K * $75/1M = $1.50 + $0.75 = $2.25
        self.assertAlmostEqual(cost.estimated_cost_usd, 2.25, places=2)

    def test_estimate_cost_with_haiku_agents(self) -> None:
        agent = AgentSession(
            agent_id="agent1",
            agent_type="Explore/Haiku",
            total_input_tokens=50_000,
            total_output_tokens=5_000,
        )
        session = Session(
            session_id="test",
            total_input_tokens=100_000,
            total_output_tokens=10_000,
            agents=[agent],
        )
        analyzer = CostAnalyzer()
        cost = analyzer.estimate_cost(session)
        # Main: 100K * $15/1M + 10K * $75/1M = $1.50 + $0.75 = $2.25
        # Agent: 50K * $0.80/1M + 5K * $4/1M = $0.04 + $0.02 = $0.06
        self.assertAlmostEqual(cost.estimated_cost_usd, 2.31, places=2)
        self.assertEqual(cost.input_tokens, 150_000)
        self.assertEqual(cost.output_tokens, 15_000)

    def test_by_tool_counts(self) -> None:
        session = Session(
            session_id="test",
            tool_calls=[
                ToolCall(name="Read"),
                ToolCall(name="Read"),
                ToolCall(name="Edit"),
                ToolCall(name="Bash"),
            ],
        )
        analyzer = CostAnalyzer()
        cost = analyzer.estimate_cost(session)
        self.assertEqual(cost.by_tool["Read"], 2)
        self.assertEqual(cost.by_tool["Edit"], 1)
        self.assertEqual(cost.by_tool["Bash"], 1)


class TestPatternDetector(TestCase):
    def test_find_redundant_reads(self) -> None:
        session = Session(
            session_id="test",
            tool_calls=[
                ToolCall(name="Read", input={"file_path": "/foo.ts"}, index=0),
                ToolCall(name="Read", input={"file_path": "/foo.ts"}, index=1),
                ToolCall(name="Read", input={"file_path": "/foo.ts"}, index=2),
            ],
        )
        detector = PatternDetector()
        redundant = detector.find_redundant_reads(session)
        self.assertEqual(len(redundant), 1)
        self.assertEqual(redundant[0].file_path, "/foo.ts")
        self.assertEqual(redundant[0].read_count, 3)
        self.assertEqual(redundant[0].total_wasted_tokens, 1000)  # 2 wasted * 500

    def test_no_redundant_reads_with_edits(self) -> None:
        session = Session(
            session_id="test",
            tool_calls=[
                ToolCall(name="Read", input={"file_path": "/foo.ts"}, index=0),
                ToolCall(name="Edit", input={"file_path": "/foo.ts"}, index=1),
                ToolCall(name="Read", input={"file_path": "/foo.ts"}, index=2),
            ],
        )
        detector = PatternDetector()
        redundant = detector.find_redundant_reads(session)
        self.assertEqual(len(redundant), 0)

    def test_find_late_test_run(self) -> None:
        session = Session(
            session_id="test",
            tool_calls=[
                ToolCall(name="Edit", input={"file_path": "/a.ts"}, index=0),
                ToolCall(name="Edit", input={"file_path": "/b.ts"}, index=1),
                ToolCall(name="Edit", input={"file_path": "/c.ts"}, index=2),
                ToolCall(name="Edit", input={"file_path": "/d.ts"}, index=3),
                ToolCall(name="Edit", input={"file_path": "/e.ts"}, index=4),
                ToolCall(name="Bash", input={"command": "go test ./..."}, index=5),
            ],
        )
        detector = PatternDetector()
        late = detector.find_late_test_runs(session)
        self.assertEqual(len(late), 1)
        self.assertEqual(late[0].edits_before_test, 5)
        self.assertEqual(late[0].first_test_index, 5)

    def test_no_late_test_with_few_edits(self) -> None:
        session = Session(
            session_id="test",
            tool_calls=[
                ToolCall(name="Edit", input={"file_path": "/a.ts"}, index=0),
                ToolCall(name="Bash", input={"command": "npm test"}, index=1),
            ],
        )
        detector = PatternDetector()
        late = detector.find_late_test_runs(session)
        self.assertEqual(len(late), 0)

    def test_find_agent_overhead(self) -> None:
        session = Session(
            session_id="test",
            agents=[
                AgentSession(
                    agent_id="a1",
                    agent_type="Explore/Haiku",
                    tool_calls=[ToolCall(name="Read")],
                ),
                AgentSession(
                    agent_id="a2",
                    agent_type="Opus",
                    tool_calls=[ToolCall(name="Read")] * 5,
                ),
            ],
        )
        detector = PatternDetector()
        overhead = detector.find_agent_overhead(session)
        self.assertEqual(len(overhead), 1)
        self.assertEqual(overhead[0].agent_id, "a1")

    def test_detect_all_patterns(self) -> None:
        sessions = [
            Session(
                session_id="s1",
                tool_calls=[
                    ToolCall(name="Read", input={"file_path": "/big.md"}, index=0),
                    ToolCall(name="Read", input={"file_path": "/big.md"}, index=1),
                ],
            ),
        ]
        detector = PatternDetector()
        patterns = detector.detect_all_patterns(sessions)
        self.assertTrue(len(patterns) >= 1)
        self.assertEqual(patterns[0].name, "Redundant File Reads")


class TestReporter(TestCase):
    def test_summary_report_basic(self) -> None:
        reporter = Reporter()
        iterations = [
            _make_iteration(1, "sess-1"),
        ]
        sessions = [
            Session(
                session_id="sess-1", total_input_tokens=50000, total_output_tokens=5000
            ),
        ]
        costs = [
            CostBreakdown(
                input_tokens=50000,
                output_tokens=5000,
                estimated_cost_usd=1.13,
                by_tool={"Read": 3, "Edit": 1},
            ),
        ]
        report = reporter.summary_report(
            Path("test.log"), iterations, sessions, costs, []
        )
        self.assertIn("Ralph Optimizer Report", report)
        self.assertIn("test.log", report)
        self.assertIn("$1.13", report)
        self.assertIn("No significant waste patterns", report)

    def test_summary_report_with_patterns(self) -> None:
        reporter = Reporter()
        iterations = [_make_iteration(1, "sess-1")]
        sessions = [Session(session_id="sess-1")]
        costs = [CostBreakdown()]
        patterns = [
            Pattern(
                name="Redundant Reads",
                description="5 redundant reads",
                occurrences=5,
                estimated_waste_tokens=2500,
                suggestion="Pre-load files",
            ),
        ]
        report = reporter.summary_report(
            Path("test.log"), iterations, sessions, costs, patterns
        )
        self.assertIn("Redundant Reads", report)
        self.assertIn("Pre-load files", report)

    def test_json_report(self) -> None:
        reporter = Reporter()
        iterations = [_make_iteration(1, "sess-1")]
        sessions = [
            Session(
                session_id="sess-1", total_input_tokens=1000, total_output_tokens=100
            )
        ]
        costs = [
            CostBreakdown(input_tokens=1000, output_tokens=100, estimated_cost_usd=0.02)
        ]
        result = reporter.json_report(Path("test.log"), iterations, sessions, costs, [])
        data = json.loads(result)
        self.assertEqual(data["iterations"], 1)
        self.assertEqual(data["total_input_tokens"], 1000)
        self.assertIsInstance(data["patterns"], list)

    def test_detailed_report(self) -> None:
        reporter = Reporter()
        session = Session(
            session_id="sess-1",
            total_input_tokens=5000,
            total_output_tokens=500,
            tool_calls=[
                ToolCall(name="Read", input={"file_path": "/foo.ts"}, index=0),
                ToolCall(name="Bash", input={"command": "npm test"}, index=1),
            ],
        )
        cost = CostBreakdown(
            input_tokens=5000, output_tokens=500, estimated_cost_usd=0.11
        )
        report = reporter.detailed_report(session, cost)
        self.assertIn("sess-1", report)
        self.assertIn("foo.ts", report)
        self.assertIn("npm test", report)


def _make_iteration(num: int, session_id: str) -> Iteration:
    return Iteration(
        number=num,
        total=1,
        session_id=session_id,
        timestamp="2026-01-28 12:00:00",
    )


if __name__ == "__main__":
    main()
