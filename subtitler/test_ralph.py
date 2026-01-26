"""Tests for ralph.py utility functions."""

import io
import json
import re
import tempfile
import unittest
from contextlib import redirect_stdout
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo

from ralph import (
    calculate_sleep_seconds,
    generate_ralph_id,
    get_timestamp,
    log,
    parse_rate_limit_reset,
    process_claude_output,
)


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


class TestParseRateLimitReset(unittest.TestCase):
    def test_parses_standard_message(self) -> None:
        msg = "You've hit your limit · resets 2am (America/Los_Angeles)"
        result = parse_rate_limit_reset(msg)
        self.assertEqual(result, (2, "am", "America/Los_Angeles"))

    def test_parses_pm_time(self) -> None:
        msg = "You've hit your limit · resets 5pm (America/New_York)"
        result = parse_rate_limit_reset(msg)
        self.assertEqual(result, (5, "pm", "America/New_York"))

    def test_parses_12_hour(self) -> None:
        msg = "You've hit your limit · resets 12pm (UTC)"
        result = parse_rate_limit_reset(msg)
        self.assertEqual(result, (12, "pm", "UTC"))

    def test_parses_uppercase_ampm(self) -> None:
        msg = "You've hit your limit · resets 3AM (Europe/London)"
        result = parse_rate_limit_reset(msg)
        self.assertEqual(result, (3, "am", "Europe/London"))

    def test_returns_none_for_non_matching(self) -> None:
        msg = "Some other error message"
        result = parse_rate_limit_reset(msg)
        self.assertIsNone(result)

    def test_returns_none_for_empty_string(self) -> None:
        result = parse_rate_limit_reset("")
        self.assertIsNone(result)

    def test_handles_different_whitespace(self) -> None:
        msg = "resets  10am  (Asia/Tokyo)"
        result = parse_rate_limit_reset(msg)
        self.assertEqual(result, (10, "am", "Asia/Tokyo"))


class TestCalculateSleepSeconds(unittest.TestCase):
    def test_reset_in_future_same_day(self) -> None:
        tz = ZoneInfo("America/Los_Angeles")
        # It's 1am, reset at 2am = 1 hour + 60s buffer
        now = datetime(2024, 1, 15, 1, 0, 0, tzinfo=tz)
        result = calculate_sleep_seconds(2, "am", "America/Los_Angeles", now=now)
        self.assertEqual(result, 3600 + 60)  # 1 hour + 60s buffer

    def test_reset_tomorrow_when_past_today(self) -> None:
        tz = ZoneInfo("America/Los_Angeles")
        # It's 3am, reset at 2am = 23 hours + 60s buffer
        now = datetime(2024, 1, 15, 3, 0, 0, tzinfo=tz)
        result = calculate_sleep_seconds(2, "am", "America/Los_Angeles", now=now)
        self.assertEqual(result, 23 * 3600 + 60)  # 23 hours + 60s buffer

    def test_pm_conversion(self) -> None:
        tz = ZoneInfo("UTC")
        # It's 1pm (13:00), reset at 5pm (17:00) = 4 hours + 60s buffer
        now = datetime(2024, 1, 15, 13, 0, 0, tzinfo=tz)
        result = calculate_sleep_seconds(5, "pm", "UTC", now=now)
        self.assertEqual(result, 4 * 3600 + 60)

    def test_12am_is_midnight(self) -> None:
        tz = ZoneInfo("UTC")
        # It's 11pm (23:00), reset at 12am (00:00) = 1 hour + 60s buffer
        now = datetime(2024, 1, 15, 23, 0, 0, tzinfo=tz)
        result = calculate_sleep_seconds(12, "am", "UTC", now=now)
        self.assertEqual(result, 3600 + 60)

    def test_12pm_is_noon(self) -> None:
        tz = ZoneInfo("UTC")
        # It's 11am (11:00), reset at 12pm (12:00) = 1 hour + 60s buffer
        now = datetime(2024, 1, 15, 11, 0, 0, tzinfo=tz)
        result = calculate_sleep_seconds(12, "pm", "UTC", now=now)
        self.assertEqual(result, 3600 + 60)

    def test_handles_partial_hours(self) -> None:
        tz = ZoneInfo("UTC")
        # It's 1:30am, reset at 2am = 30 minutes + 60s buffer
        now = datetime(2024, 1, 15, 1, 30, 0, tzinfo=tz)
        result = calculate_sleep_seconds(2, "am", "UTC", now=now)
        self.assertEqual(result, 30 * 60 + 60)  # 30 minutes + 60s buffer

    def test_cross_timezone(self) -> None:
        # Now is in UTC, but reset is specified in LA time
        utc = ZoneInfo("UTC")
        # 10am UTC = 2am LA (UTC-8 in winter)
        now = datetime(2024, 1, 15, 10, 0, 0, tzinfo=utc)
        # Reset at 3am LA time = 11am UTC = 1 hour from now
        result = calculate_sleep_seconds(3, "am", "America/Los_Angeles", now=now)
        self.assertEqual(result, 3600 + 60)


if __name__ == "__main__":
    unittest.main()
