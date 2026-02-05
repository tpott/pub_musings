#!/usr/bin/env python3
"""
Ralph loop: repeatedly runs claude with RALPH.md until STOP_RALPH exists.
"""

import argparse
import base64
import json
import os
import re
import subprocess
import sys
import time
from datetime import datetime, timedelta, timezone
from collections.abc import Iterable
from pathlib import Path
from zoneinfo import ZoneInfo

DEFAULT_MAX_ITERATIONS = 10
CLAUDE_MODEL = "opus"
PROMPT_FILE = "RALPH.md"
STOP_FILE = "STOP_RALPH"
FEEDBACK_FILE = "FEEDBACK.md"
FETCH_FEEDBACK_SCRIPT = "scripts/fetch-feedback.py"

# Exponential backoff configuration for API errors (500, 529, overloaded)
INITIAL_BACKOFF_SECONDS = 15
MAX_BACKOFF_SECONDS = 240  # 4 minutes
MAX_RETRY_DURATION_SECONDS = 8 * 3600  # 8 hours


def generate_ralph_id() -> str:
    """Generate an 8-character base32 ID (40 bits of entropy)."""
    random_bytes = os.urandom(5)  # 5 bytes = 40 bits
    return base64.b32encode(random_bytes).decode("ascii").lower()


def parse_rate_limit_reset(result: str) -> tuple[int, str, str] | None:
    """
    Parse rate limit message, return (hour, am/pm, timezone) or None.

    Expected format: "You've hit your limit · resets 2am (America/Los_Angeles)"
    The middle dot (·) is U+00B7.
    """
    pattern = r"resets\s+(\d{1,2})(am|pm)\s+\(([^)]+)\)"
    match = re.search(pattern, result, re.IGNORECASE)
    if match is None:
        return None
    hour = int(match.group(1))
    ampm = match.group(2).lower()
    tz = match.group(3)
    return (hour, ampm, tz)


def calculate_sleep_seconds(
    hour: int, ampm: str, reset_tz: str, now: datetime | None = None
) -> int:
    """
    Calculate seconds to sleep until the reset time.

    Args:
        hour: Hour of reset (1-12)
        ampm: "am" or "pm"
        reset_tz: IANA timezone string (e.g., "America/Los_Angeles")
        now: Current time (for testing), defaults to datetime.now()

    Returns:
        Number of seconds to sleep until reset time (plus 60s buffer)
    """
    # Convert 12-hour to 24-hour format
    if ampm == "am":
        hour_24 = 0 if hour == 12 else hour
    else:
        hour_24 = 12 if hour == 12 else hour + 12

    tz = ZoneInfo(reset_tz)

    if now is None:
        now = datetime.now(tz)
    elif now.tzinfo is None:
        now = now.replace(tzinfo=tz)
    else:
        now = now.astimezone(tz)

    # Create reset time for today at the specified hour
    reset_time = now.replace(hour=hour_24, minute=0, second=0, microsecond=0)

    # If reset time has passed today, it's tomorrow
    if reset_time <= now:
        reset_time += timedelta(days=1)

    delta = reset_time - now
    # Add 60 second buffer to ensure we're past the reset
    return int(delta.total_seconds()) + 60


def is_api_server_error(result: str) -> bool:
    """
    Check if the result indicates an API server error (500, 529, overloaded).

    These errors are transient and should be retried with exponential backoff.
    """
    error_patterns = [
        r"status[_\s]?code[:\s]+5\d{2}",  # status_code: 500, status code: 529
        r"\b5\d{2}\b.*error",  # 500 error, 529 error
        r"error.*\b5\d{2}\b",  # error...500
        r"overloaded",  # API overloaded
        r"internal[_\s]?server[_\s]?error",  # internal server error
        r"service[_\s]?unavailable",  # service unavailable
        r"APIStatusError.*5\d{2}",  # APIStatusError with 5xx
    ]
    result_lower = result.lower()
    for pattern in error_patterns:
        if re.search(pattern, result_lower, re.IGNORECASE):
            return True
    return False


def calculate_backoff(attempt: int) -> int:
    """
    Calculate backoff time using exponential backoff.

    Args:
        attempt: The retry attempt number (0-indexed)

    Returns:
        Backoff time in seconds, capped at MAX_BACKOFF_SECONDS
    """
    backoff = INITIAL_BACKOFF_SECONDS * (2**attempt)
    return int(min(backoff, MAX_BACKOFF_SECONDS))


def get_git_head() -> str | None:
    """Get current git HEAD commit hash (short form)."""
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except Exception:
        pass
    return None


def log_feedback_before(feedback_log: Path, feedback_file: Path) -> str | None:
    """
    Log feedback content and git state before processing.

    Returns git commit hash if feedback was logged, None otherwise.
    """
    if not feedback_file.exists():
        return None

    git_before = get_git_head()
    content = feedback_file.read_text()
    timestamp = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    with open(feedback_log, "a") as f:
        f.write(f"=== {timestamp} ===\n")
        f.write(f"git_commit_before: {git_before or 'unknown'}\n")
        f.write("--- FEEDBACK.md content ---\n")
        f.write(content)
        if not content.endswith("\n"):
            f.write("\n")
        f.write("---\n")

    return git_before


def log_feedback_after(feedback_log: Path, git_before: str | None) -> None:
    """Log git state after processing feedback."""
    if git_before is None:
        return

    git_after = get_git_head()
    with open(feedback_log, "a") as f:
        f.write(f"git_commit_after: {git_after or 'unknown'}\n\n")


def log(msg: str, log_file: Path | None, newline_before: bool = False) -> None:
    """Print message and optionally append to log file."""
    print(msg)
    if log_file is None:
        return
    with open(log_file, "a") as f:
        if newline_before:
            f.write("\n")
        f.write(f"{msg}\n")


def get_timestamp() -> str:
    """Return formatted timestamp string with local, UTC, and epoch times."""
    now = datetime.now()
    utc_now = datetime.now(timezone.utc)
    epoch_ms = time.time()

    local_str = now.strftime("%Y-%m-%d %H:%M:%S %Z").strip()
    utc_str = utc_now.strftime("%Y-%m-%d %H:%M:%S UTC")

    return f"{local_str} | {utc_str} | {epoch_ms:.3f}"


def fetch_feedback(log_file: Path | None, script_path: Path | None = None) -> None:
    """Run the feedback fetch script. Logs result but never blocks the loop."""
    script = script_path if script_path is not None else Path(FETCH_FEEDBACK_SCRIPT)
    if not script.exists():
        log(f"Feedback: {script} not found, skipping", log_file)
        return

    try:
        result = subprocess.run(
            [sys.executable, str(script)],
            capture_output=True,
            text=True,
            timeout=30,
        )
        if result.returncode == 0:
            log(f"Feedback: {result.stdout.strip()}", log_file)
        elif result.returncode == 1:
            log("Feedback: No new feedback", log_file)
        else:
            stderr_msg = result.stderr.strip()
            log(
                f"Feedback: fetch failed (exit {result.returncode}): {stderr_msg}",
                log_file,
            )
    except subprocess.TimeoutExpired:
        log("Feedback: fetch timed out after 30s", log_file)
    except Exception as e:
        log(f"Feedback: fetch error: {e}", log_file)


def process_claude_output(
    lines: Iterable[str], verbose: bool, log_file: Path | None
) -> str | None:
    """
    Process lines from claude output, return the result line.

    Scans for the line containing '"type":"result","subtype":"success"'.
    If verbose, streams all output to stdout.
    If not verbose, prints session_id once, then '.' for each line received.
    If log_file is not None, writes all JSON lines to the log file.
    """
    result_line = None
    session_id_printed = False

    for line in lines:
        stripped = line.rstrip("\n")

        # Log all JSON to file if log_file is provided
        if log_file is not None:
            with open(log_file, "a") as f:
                f.write(f"{stripped}\n")

        if verbose:
            print(line, end="", flush=True)
        else:
            # Print session_id once before the dots
            if not session_id_printed:
                data: dict[str, str] = {}
                try:
                    data = json.loads(stripped)
                except json.JSONDecodeError:
                    pass
                if "session_id" in data:
                    print(f"session_id: {data['session_id']}")
                    session_id_printed = True
                elif "sessionId" in data:
                    print(f"sessionId: {data['sessionId']}")
                    session_id_printed = True
            print(".", end="", flush=True)

        # Track the result line
        if '"type":"result","subtype":"success"' in stripped:
            result_line = stripped

    if not verbose:
        print()  # Newline after dots

    return result_line


def run_claude(prompt_content: str, verbose: bool, log_file: Path | None) -> str | None:
    """Run claude subprocess and return the last line of output."""
    cmd = [
        "claude",
        "--print",
        "--dangerously-skip-permissions",
        "--output-format=stream-json",
        "--verbose",
        "--model",
        CLAUDE_MODEL,
    ]

    with subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    ) as proc:
        assert proc.stdin is not None
        assert proc.stdout is not None
        assert proc.stderr is not None

        # Send prompt and close stdin
        proc.stdin.write(prompt_content)
        proc.stdin.close()

        last_line = process_claude_output(proc.stdout, verbose, log_file)

        # Wait for process to complete
        proc.wait()

        if proc.returncode != 0:
            print(f"Process exited with code {proc.returncode}")

        stderr_output = proc.stderr.read()
        if stderr_output:
            print(f"STDERR: {stderr_output}", file=sys.stderr)

    return last_line


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Ralph loop: repeatedly runs claude until STOP_RALPH exists"
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Pass through all streamed JSON output (default: show dots for progress)",
    )
    parser.add_argument(
        "-n",
        "--max-iterations",
        type=int,
        default=DEFAULT_MAX_ITERATIONS,
        help=f"Maximum number of iterations (default: {DEFAULT_MAX_ITERATIONS})",
    )
    parser.add_argument(
        "--log-dir",
        type=Path,
        help="Directory to write logs to (generates ralph-<ID>.log filename)",
    )
    parser.add_argument(
        "--auto-fetch-feedback-script",
        type=Path,
        default=None,
        help="Path to feedback fetch script to run before each iteration",
    )
    args = parser.parse_args()

    max_iterations = args.max_iterations
    prompt_file = Path(PROMPT_FILE)
    stop_marker = Path(STOP_FILE)
    feedback_file = Path(FEEDBACK_FILE)

    # Set up logging if requested
    log_file = None
    feedback_log = None
    if args.log_dir:
        args.log_dir.mkdir(parents=True, exist_ok=True)
        ralph_id = generate_ralph_id()
        log_file = args.log_dir / f"ralph-{ralph_id}.log"
        feedback_log = args.log_dir / f"feedback-{ralph_id}.log"
        print(f"Logging to: {log_file}")

    for i in range(max_iterations):
        # Check if we should stop
        if stop_marker.exists():
            log(f"{STOP_FILE} found, stopping after {i} iteration(s)", log_file)
            break

        log(
            f"=== Iteration {i + 1}/{max_iterations} === {get_timestamp()}",
            log_file,
            newline_before=True,
        )

        # Fetch any new user feedback from production
        fetch_feedback(log_file, args.auto_fetch_feedback_script)

        # Read the prompt file
        if not prompt_file.exists():
            print(f"Error: {prompt_file} not found", file=sys.stderr)
            sys.exit(1)

        prompt_content = prompt_file.read_text()

        # Log feedback before Claude processes it (will be deleted by Claude)
        git_before = None
        if feedback_log is not None:
            git_before = log_feedback_before(feedback_log, feedback_file)

        last_log = {}
        try:
            last_line = run_claude(prompt_content, args.verbose, log_file)
            if last_line is not None:
                last_log = json.loads(last_line)
        except json.JSONDecodeError:
            log(f"Last line failed to parse as JSON: {last_line}", log_file)
            continue
        except FileNotFoundError:
            print("Error: 'claude' command not found", file=sys.stderr)
            sys.exit(1)
        except KeyboardInterrupt:
            print("\nInterrupted by user")
            sys.exit(130)

        # Log git state after feedback was processed
        if feedback_log is not None:
            log_feedback_after(feedback_log, git_before)

        if "result" not in last_log:
            log(f'Last line missing "result": {last_line}', log_file)
            continue

        # If ralph is run with --verbose then skip logging the Result so it's
        # easier to parse with `jq`. Grep for `"type":"result","subtype":"success"`
        if not args.verbose:
            result_text = last_log["result"]
            # print because we don't want this in logs
            print(f"Result: {result_text}")

        if "is_error" in last_log and last_log["is_error"]:
            result_text = last_log.get("result", "")

            # Check for rate limit first
            parsed = parse_rate_limit_reset(result_text)
            if parsed:
                hour, ampm, reset_tz = parsed
                sleep_secs = calculate_sleep_seconds(hour, ampm, reset_tz)
                sleep_mins = sleep_secs // 60
                log(
                    f"Rate limited. Sleeping {sleep_mins} minutes until {hour}{ampm} ({reset_tz})",
                    log_file,
                )
                time.sleep(sleep_secs)
                continue

            # Check for API server errors (500, 529, overloaded)
            if is_api_server_error(result_text):
                retry_start = time.time()
                attempt = 0
                while True:
                    backoff = calculate_backoff(attempt)
                    elapsed = time.time() - retry_start
                    if elapsed + backoff > MAX_RETRY_DURATION_SECONDS:
                        log(
                            f"API error retry exceeded {MAX_RETRY_DURATION_SECONDS // 3600} hours, giving up",
                            log_file,
                        )
                        break

                    log(
                        f"API server error (attempt {attempt + 1}). Retrying in {backoff}s...",
                        log_file,
                    )
                    time.sleep(backoff)

                    # Retry the claude call
                    retry_last_line = run_claude(prompt_content, args.verbose, log_file)
                    if retry_last_line is not None:
                        try:
                            retry_log = json.loads(retry_last_line)
                            if not retry_log.get("is_error", False):
                                # Success! Continue to next iteration
                                log("API error resolved, continuing", log_file)
                                break
                            retry_result = retry_log.get("result", "")
                            if not is_api_server_error(retry_result):
                                # Different error, stop retrying
                                log(f"Different error: {retry_result}", log_file)
                                break
                        except json.JSONDecodeError:
                            pass

                    attempt += 1

    else:
        log(f"Completed {max_iterations} iterations", log_file)


if __name__ == "__main__":
    main()
