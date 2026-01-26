"""Tests for ralph.py utility functions."""

import io
import json
import re
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

from ralph import generate_ralph_id, get_timestamp, log, process_claude_output


class TestGenerateRalphId(unittest.TestCase):
    def test_returns_8_characters(self) -> None:
        ralph_id = generate_ralph_id()
        self.assertEqual(len(ralph_id), 8)

    def test_returns_lowercase(self) -> None:
        ralph_id = generate_ralph_id()
        self.assertEqual(ralph_id, ralph_id.lower())

    def test_returns_valid_base32_characters(self) -> None:
        ralph_id = generate_ralph_id()
        # Base32 uses a-z and 2-7
        self.assertRegex(ralph_id, r"^[a-z2-7]+$")

    def test_generates_unique_ids(self) -> None:
        ids = {generate_ralph_id() for _ in range(100)}
        self.assertEqual(len(ids), 100)


class TestGetTimestamp(unittest.TestCase):
    def test_contains_utc_marker(self) -> None:
        ts = get_timestamp()
        self.assertIn("UTC", ts)

    def test_contains_epoch(self) -> None:
        ts = get_timestamp()
        # Should contain a float epoch like "1234567890.123"
        self.assertIsNotNone(re.search(r"\d{10,}\.\d{3}", ts))

    def test_contains_pipe_separators(self) -> None:
        ts = get_timestamp()
        self.assertEqual(ts.count("|"), 2)


class TestLog(unittest.TestCase):
    def test_writes_to_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            log_file = Path(tmp_dir) / "test.log"
            log("test message", log_file)
            self.assertEqual(log_file.read_text(), "test message\n")

    def test_appends_to_existing_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            log_file = Path(tmp_dir) / "test.log"
            log("first", log_file)
            log("second", log_file)
            self.assertEqual(log_file.read_text(), "first\nsecond\n")

    def test_newline_before(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            log_file = Path(tmp_dir) / "test.log"
            log("first", log_file)
            log("second", log_file, newline_before=True)
            self.assertEqual(log_file.read_text(), "first\n\nsecond\n")

    def test_no_crash_when_log_file_none(self) -> None:
        # Just verify it doesn't crash when log_file is None
        log("test message", None)


class TestProcessClaudeOutput(unittest.TestCase):
    def test_returns_last_line(self) -> None:
        lines = ["first\n", "second\n", "third\n"]
        with redirect_stdout(io.StringIO()):
            result = process_claude_output(lines, verbose=False, log_file=None)
        self.assertEqual(result, "third")

    def test_returns_none_for_empty_input(self) -> None:
        with redirect_stdout(io.StringIO()):
            result = process_claude_output([], verbose=False, log_file=None)
        self.assertIsNone(result)

    def test_strips_newlines(self) -> None:
        lines = ["line with newline\n"]
        with redirect_stdout(io.StringIO()):
            result = process_claude_output(lines, verbose=False, log_file=None)
        self.assertEqual(result, "line with newline")

    def test_verbose_prints_full_lines(self) -> None:
        lines = ["line1\n", "line2\n"]
        stdout = io.StringIO()
        with redirect_stdout(stdout):
            process_claude_output(lines, verbose=True, log_file=None)
        self.assertEqual(stdout.getvalue(), "line1\nline2\n")

    def test_non_verbose_prints_dots(self) -> None:
        lines = ["line1\n", "line2\n", "line3\n"]
        stdout = io.StringIO()
        with redirect_stdout(stdout):
            process_claude_output(lines, verbose=False, log_file=None)
        self.assertEqual(stdout.getvalue(), "...\n")

    def test_prints_session_id_once(self) -> None:
        lines = [
            json.dumps({"session_id": "abc123"}) + "\n",
            json.dumps({"session_id": "abc123"}) + "\n",
            json.dumps({"other": "data"}) + "\n",
        ]
        stdout = io.StringIO()
        with redirect_stdout(stdout):
            process_claude_output(lines, verbose=False, log_file=None)
        output = stdout.getvalue()
        self.assertEqual(output.count("session_id: abc123"), 1)
        self.assertIn("...\n", output)

    def test_handles_invalid_json(self) -> None:
        lines = ["not json\n", "also not json\n"]
        stdout = io.StringIO()
        with redirect_stdout(stdout):
            result = process_claude_output(lines, verbose=False, log_file=None)
        self.assertEqual(result, "also not json")
        self.assertEqual(stdout.getvalue(), "..\n")

    def test_handles_json_without_session_id(self) -> None:
        lines = [
            json.dumps({"other": "field"}) + "\n",
            json.dumps({"session_id": "found"}) + "\n",
        ]
        stdout = io.StringIO()
        with redirect_stdout(stdout):
            process_claude_output(lines, verbose=False, log_file=None)
        output = stdout.getvalue()
        self.assertIn("session_id: found", output)


if __name__ == "__main__":
    unittest.main()
