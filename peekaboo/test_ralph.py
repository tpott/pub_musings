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
    INITIAL_BACKOFF_SECONDS,
    MAX_BACKOFF_SECONDS,
    calculate_backoff,
    calculate_sleep_seconds,
    fetch_feedback,
    generate_ralph_id,
    get_timestamp,
    is_api_server_error,
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
    def test_returns_result_line(self) -> None:
        result_line = '{"type":"result","subtype":"success","result":"done"}'
        lines = ["first\n", "second\n", f"{result_line}\n"]
        with redirect_stdout(io.StringIO()):
            result = process_claude_output(lines, verbose=False, log_file=None)
        self.assertEqual(result, result_line)

    def test_returns_none_without_result_line(self) -> None:
        lines = ["first\n", "second\n", "third\n"]
        with redirect_stdout(io.StringIO()):
            result = process_claude_output(lines, verbose=False, log_file=None)
        self.assertIsNone(result)

    def test_returns_none_for_empty_input(self) -> None:
        with redirect_stdout(io.StringIO()):
            result = process_claude_output([], verbose=False, log_file=None)
        self.assertIsNone(result)

    def test_strips_newlines_from_result(self) -> None:
        result_line = '{"type":"result","subtype":"success","result":"done"}'
        lines = [f"{result_line}\n"]
        with redirect_stdout(io.StringIO()):
            result = process_claude_output(lines, verbose=False, log_file=None)
        self.assertEqual(result, result_line)

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
        self.assertIsNone(result)  # No result line present
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


class TestFetchFeedback(unittest.TestCase):
    def test_uses_custom_script_path(self) -> None:
        """When script_path is provided, it should be used instead of the default."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            script = Path(tmp_dir) / "custom-feedback.py"
            script.write_text('import sys; print("custom output"); sys.exit(0)')
            log_file = Path(tmp_dir) / "test.log"
            fetch_feedback(log_file, script_path=script)
            log_content = log_file.read_text()
            self.assertIn("custom output", log_content)

    def test_custom_script_not_found(self) -> None:
        """When script_path points to a nonexistent file, should log and skip."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            log_file = Path(tmp_dir) / "test.log"
            fake_path = Path(tmp_dir) / "does-not-exist.py"
            fetch_feedback(log_file, script_path=fake_path)
            log_content = log_file.read_text()
            self.assertIn("not found, skipping", log_content)

    def test_default_script_when_none(self) -> None:
        """When script_path is None, should fall back to the default constant."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            log_file = Path(tmp_dir) / "test.log"
            # The default script likely doesn't exist in the test environment,
            # so we expect the "not found" message with the default path
            fetch_feedback(log_file, script_path=None)
            log_content = log_file.read_text()
            # Should reference the default script path
            self.assertIn("Feedback:", log_content)

    def test_custom_script_exit_1_no_feedback(self) -> None:
        """Exit code 1 means no new feedback."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            script = Path(tmp_dir) / "no-feedback.py"
            script.write_text("import sys; sys.exit(1)")
            log_file = Path(tmp_dir) / "test.log"
            fetch_feedback(log_file, script_path=script)
            log_content = log_file.read_text()
            self.assertIn("No new feedback", log_content)

    def test_custom_script_exit_2_error(self) -> None:
        """Non-zero, non-1 exit code means error."""
        with tempfile.TemporaryDirectory() as tmp_dir:
            script = Path(tmp_dir) / "error-feedback.py"
            script.write_text(
                'import sys; print("error details", file=sys.stderr); sys.exit(2)'
            )
            log_file = Path(tmp_dir) / "test.log"
            fetch_feedback(log_file, script_path=script)
            log_content = log_file.read_text()
            self.assertIn("fetch failed (exit 2)", log_content)
            self.assertIn("error details", log_content)


class TestIsApiServerError(unittest.TestCase):
    def test_detects_status_code_500(self) -> None:
        self.assertTrue(is_api_server_error("status_code: 500"))
        self.assertTrue(is_api_server_error("status code: 500"))
        self.assertTrue(is_api_server_error("statuscode:500"))

    def test_detects_status_code_529(self) -> None:
        self.assertTrue(is_api_server_error("status_code: 529"))
        self.assertTrue(is_api_server_error("Error with status code: 529"))

    def test_detects_5xx_with_error(self) -> None:
        self.assertTrue(is_api_server_error("500 error occurred"))
        self.assertTrue(is_api_server_error("error 503"))
        self.assertTrue(is_api_server_error("Got 502 error from server"))

    def test_detects_overloaded(self) -> None:
        self.assertTrue(is_api_server_error("API is overloaded"))
        self.assertTrue(is_api_server_error("Server overloaded, try again"))

    def test_detects_internal_server_error(self) -> None:
        self.assertTrue(is_api_server_error("internal server error"))
        self.assertTrue(is_api_server_error("Internal_Server_Error"))

    def test_detects_service_unavailable(self) -> None:
        self.assertTrue(is_api_server_error("service unavailable"))
        self.assertTrue(is_api_server_error("Service_Unavailable"))

    def test_detects_api_status_error(self) -> None:
        self.assertTrue(is_api_server_error("APIStatusError: 500"))
        self.assertTrue(is_api_server_error("anthropic.APIStatusError 529"))

    def test_returns_false_for_rate_limit(self) -> None:
        # Rate limits are not server errors
        self.assertFalse(
            is_api_server_error(
                "You've hit your limit · resets 2am (America/Los_Angeles)"
            )
        )

    def test_returns_false_for_other_errors(self) -> None:
        self.assertFalse(is_api_server_error("Invalid API key"))
        self.assertFalse(is_api_server_error("Authentication failed"))
        self.assertFalse(is_api_server_error("400 bad request"))

    def test_returns_false_for_empty_string(self) -> None:
        self.assertFalse(is_api_server_error(""))


class TestCalculateBackoff(unittest.TestCase):
    def test_first_attempt_is_initial_backoff(self) -> None:
        self.assertEqual(calculate_backoff(0), INITIAL_BACKOFF_SECONDS)

    def test_second_attempt_is_double(self) -> None:
        self.assertEqual(calculate_backoff(1), INITIAL_BACKOFF_SECONDS * 2)

    def test_third_attempt_is_quadruple(self) -> None:
        self.assertEqual(calculate_backoff(2), INITIAL_BACKOFF_SECONDS * 4)

    def test_caps_at_max_backoff(self) -> None:
        # After enough attempts, should cap at max
        result = calculate_backoff(10)  # Would be 15 * 1024 = 15360 without cap
        self.assertEqual(result, MAX_BACKOFF_SECONDS)

    def test_progression_sequence(self) -> None:
        # 15 -> 30 -> 60 -> 120 -> 240 (capped)
        expected = [15, 30, 60, 120, 240, 240, 240]
        for i, exp in enumerate(expected):
            self.assertEqual(calculate_backoff(i), exp)


if __name__ == "__main__":
    unittest.main()
