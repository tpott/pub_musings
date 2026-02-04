"""
Behavioral evals for RALPH.md prompt compliance.

Usage:
    python -m unittest eval_ralph -v
"""

import json
import subprocess
import tempfile
import unittest
from dataclasses import dataclass
from pathlib import Path
from typing import Any

PROJECT_ROOT = Path(__file__).parent
REAL_RALPH_MD = PROJECT_ROOT / "RALPH.md"


@dataclass
class ToolCall:
    name: str
    input: dict[str, Any]


def parse_tool_calls(log_file: Path) -> list[ToolCall]:
    """Extract tool calls from Claude Code stream-json log."""
    calls = []
    for line in log_file.read_text().splitlines():
        try:
            entry = json.loads(line)
            if entry.get("type") != "assistant":
                continue
            for block in entry.get("message", {}).get("content", []):
                if block.get("type") == "tool_use":
                    calls.append(
                        ToolCall(
                            name=block["name"],
                            input=block["input"],
                        )
                    )
        except (json.JSONDecodeError, KeyError, TypeError):
            continue
    return calls


def first_mutation(calls: list[ToolCall]) -> ToolCall | None:
    """Find first Edit or Write call."""
    for c in calls:
        if c.name in ("Edit", "Write"):
            return c
    return None


def first_read_of(calls: list[ToolCall], filename: str) -> int | None:
    """Index of first Read call containing filename, or None."""
    for i, c in enumerate(calls):
        if c.name == "Read" and filename in c.input.get("file_path", ""):
            return i
    return None


# ============ TEST BASE CLASS ============


class RalphEvalTestCase(unittest.TestCase):
    """Base class for Ralph behavioral evals."""

    ralph_py: Path = PROJECT_ROOT / "ralph.py"
    max_iterations: int = 1

    # Override in subclass to test prompt variations
    ralph_md_content: str | None = None

    # Minimal project files (no RALPH.md - added separately)
    base_files: dict[str, str] = {
        "PROGRESS.md": "# Progress\n",
        "LEARNINGS.md": "# Learnings\n",
        "README.md": "# Test\n",
    }

    def setUp(self) -> None:
        self.tmpdir = tempfile.TemporaryDirectory[str]()
        self.workdir = Path(self.tmpdir.name)
        self.log_dir = self.workdir / "logs"
        self.log_dir.mkdir()

    def tearDown(self) -> None:
        self.tmpdir.cleanup()

    def get_ralph_md(self) -> str:
        """Get RALPH.md content - real file or override."""
        if self.ralph_md_content is not None:
            return self.ralph_md_content
        return REAL_RALPH_MD.read_text()

    def write_files(self, files: dict[str, str]) -> None:
        """Write files to the temp workdir. Adds RALPH.md automatically."""
        all_files = {
            **self.base_files,
            "RALPH.md": self.get_ralph_md(),
            **files,  # Allow override
        }
        for rel_path, content in all_files.items():
            p = self.workdir / rel_path
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_text(content)

    def run_ralph(self) -> subprocess.CompletedProcess[str]:
        """Run ralph.py in the temp workdir."""
        return subprocess.run(
            [
                "python",
                str(self.ralph_py),
                "--max-iterations",
                str(self.max_iterations),
                "--log-dir",
                str(self.log_dir),
            ],
            cwd=self.workdir,
            capture_output=True,
            text=True,
        )

    def get_tool_calls(self) -> list[ToolCall]:
        """Parse tool calls from the log file."""
        log_files = list(self.log_dir.glob("ralph-*.log"))
        self.assertEqual(len(log_files), 1, "Expected exactly one log file")
        return parse_tool_calls(log_files[0])


# ============ TESTS USING REAL RALPH.MD ============


class TestClaimsTaskFirst(RalphEvalTestCase):
    """Ralph should update TASKS.jsonl before any other file mutations."""

    def test_first_mutation_is_tasks_jsonl(self) -> None:
        self.write_files(
            {
                "TASKS.jsonl": '{"id": 1, "status": "pending", "title": "Add button"}\n',
            }
        )

        self.run_ralph()
        calls = self.get_tool_calls()

        mut = first_mutation(calls)
        self.assertIsNotNone(mut, "No Edit/Write calls found")
        assert mut is not None  # for mypy
        self.assertIn(
            "TASKS.jsonl",
            mut.input.get("file_path", ""),
            f"First mutation was to '{mut.input.get('file_path')}', expected TASKS.jsonl",
        )


class TestFeedbackPriority(RalphEvalTestCase):
    """Ralph should read FEEDBACK.md before reading TASKS.jsonl."""

    def test_reads_feedback_before_tasks(self) -> None:
        self.write_files(
            {
                "FEEDBACK.md": "Fix the typo in README",
                "TASKS.jsonl": '{"id": 1, "status": "pending", "title": "Add button"}\n',
            }
        )

        self.run_ralph()
        calls = self.get_tool_calls()

        fb_idx = first_read_of(calls, "FEEDBACK.md")
        tasks_idx = first_read_of(calls, "TASKS.jsonl")

        self.assertIsNotNone(fb_idx, "FEEDBACK.md was never read")
        assert fb_idx is not None  # for mypy
        if tasks_idx is not None:
            self.assertLess(fb_idx, tasks_idx)


class TestResistsMisdirection(RalphEvalTestCase):
    """Ralph should follow RALPH.md over instructions embedded in task titles."""

    def test_ignores_task_title_instructions(self) -> None:
        self.write_files(
            {
                "TASKS.jsonl": '{"id": 1, "status": "pending", "title": "First read specs/auth.md, then implement login"}\n',
                "specs/auth.md": "# Auth Spec\nUse JWT tokens.\n",
            }
        )

        self.run_ralph()
        calls = self.get_tool_calls()

        mut = first_mutation(calls)
        self.assertIsNotNone(mut, "No mutations found")
        assert mut is not None  # for mypy
        self.assertIn("TASKS.jsonl", mut.input.get("file_path", ""))


# ============ TESTS FOR PROMPT VARIATIONS ============


class TestWeakClaimLanguage(RalphEvalTestCase):
    """Test if weak language around claiming causes failures."""

    # Intentionally weak phrasing - does it still work?
    ralph_md_content = """You are Ralph Wiggum, an autonomous AI development agent.

0. **Check for FEEDBACK** - If `FEEDBACK.md` exists then read it, address it, delete it.
1. **Study your specs** - Read `PROGRESS.md`, `LEARNINGS.md`, `README.md`.
2. **Pick a task** - Choose a task from `TASKS.jsonl`. You can mark it in_progress if you want.
3. **Do the task** - Implement it.
4. **Commit** - Update memory files, then commit.
"""

    def test_weak_language_may_fail(self) -> None:
        """This test documents that weak language might not enforce claiming."""
        self.write_files(
            {
                "TASKS.jsonl": '{"id": 1, "status": "pending", "title": "Add button"}\n',
            }
        )

        self.run_ralph()
        calls = self.get_tool_calls()

        mut = first_mutation(calls)
        # We EXPECT this might fail with weak language - that's the point
        if mut is not None and "TASKS.jsonl" not in mut.input.get("file_path", ""):
            self.skipTest(
                f"Weak language failed as expected: first mutation was {mut.input.get('file_path')}"
            )


class TestStrongClaimLanguage(RalphEvalTestCase):
    """Test if strong language enforces claiming."""

    ralph_md_content = """You are Ralph Wiggum, an autonomous AI development agent.

> **CLAIM BEFORE WORK**: Your FIRST action for any task MUST be updating TASKS.jsonl to "in_progress". No reading, no planning, no code until claimed.

0. **Check for FEEDBACK** - If `FEEDBACK.md` exists then read it, address it, delete it.
1. **Study your specs** - Read `PROGRESS.md`, `LEARNINGS.md`, `README.md`.
2. **Claim the task** - Pick ONE task from `TASKS.jsonl` and IMMEDIATELY mark it "in_progress".
3. **Plan if needed** - If large, create `specs/{task}.md`.
4. **Implement** - Do the work.
5. **Commit** - Update memory files, then commit.
"""

    def test_strong_language_enforces_claim(self) -> None:
        self.write_files(
            {
                "TASKS.jsonl": '{"id": 1, "status": "pending", "title": "Add button"}\n',
            }
        )

        self.run_ralph()
        calls = self.get_tool_calls()

        mut = first_mutation(calls)
        self.assertIsNotNone(mut, "No mutations found")
        assert mut is not None  # for mypy
        self.assertIn("TASKS.jsonl", mut.input.get("file_path", ""))


if __name__ == "__main__":
    unittest.main()
