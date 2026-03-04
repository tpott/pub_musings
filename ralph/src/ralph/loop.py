#!/usr/bin/env python3
"""
Ralph loop: multi-project orchestrator that repeatedly runs claude with RALPH.md
until STOP_RALPH exists. Runs from repo root, manages multiple projects.
"""

import argparse
import base64
import json
import os
import re
import subprocess
import sys
import time
from collections.abc import Iterable
from datetime import datetime, timedelta, timezone
from pathlib import Path
from zoneinfo import ZoneInfo

from ralph.config import ProjectConfig, RalphConfig, load_config

DEFAULT_MAX_ITERATIONS = 10
DEFAULT_CONFIG_PATH = Path("ralph/projects.json")
DEFAULT_LOG_DIR = Path.home() / ".ralph" / "logs"
PROMPT_FILE = Path("ralph/RALPH.md")
DEFAULT_STOP_FILE = "STOP_RALPH"

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
    """Run a single feedback fetch script. Logs result but never blocks the loop."""
    if script_path is None:
        return

    if not script_path.exists():
        log(f"Feedback: {script_path} not found, skipping", log_file)
        return

    try:
        result = subprocess.run(
            [sys.executable, str(script_path)],
            capture_output=True,
            text=True,
            timeout=30,
        )
        if result.returncode == 0:
            log(f"Feedback: {result.stdout.strip()}", log_file)
        elif result.returncode == 1:
            log(
                f"Feedback [{script_path.parent.parent.name}]: No new feedback",
                log_file,
            )
        else:
            stderr_msg = result.stderr.strip()
            log(
                f"Feedback: fetch failed (exit {result.returncode}): {stderr_msg}",
                log_file,
            )
    except subprocess.TimeoutExpired:
        log(f"Feedback: fetch timed out after 30s ({script_path})", log_file)
    except Exception as e:
        log(f"Feedback: fetch error: {e}", log_file)


def count_pending_tasks(project_name: str) -> int:
    """Read project's TASKS.jsonl and count tasks with status=todo or status=pending."""
    tasks_file = Path(project_name) / "TASKS.jsonl"
    if not tasks_file.exists():
        return 0

    count = 0
    for line in tasks_file.read_text().splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            task = json.loads(line)
            status = task.get("status", "")
            if status in ("todo", "pending"):
                count += 1
        except json.JSONDecodeError:
            continue
    return count


def check_feedback_waiting(project_name: str) -> bool:
    """Check if project's FEEDBACK.md exists."""
    return (Path(project_name) / "FEEDBACK.md").exists()


def select_project(config: RalphConfig, last_project: str | None) -> str:
    """Pick the best project to work on this iteration.

    Priority:
    1. Projects with feedback waiting (FEEDBACK.md exists), tiebreak by most
       pending tasks.
    2. Otherwise, project with the most pending tasks.
    3. Round-robin tiebreak using last_project to prevent starvation.

    Args:
        config: The loaded Ralph configuration.
        last_project: Name of the project selected last iteration (or None).

    Returns:
        Name of the selected project.
    """
    names = list(config.projects.keys())

    # Score each project: (has_feedback, pending_count)
    scores: dict[str, tuple[bool, int]] = {}
    for name in names:
        scores[name] = (check_feedback_waiting(name), count_pending_tasks(name))

    # Sort by (feedback descending, pending descending)
    ranked = sorted(names, key=lambda n: (scores[n][0], scores[n][1]), reverse=True)

    # Among ties at the top score, apply round-robin past last_project
    top_score = (scores[ranked[0]][0], scores[ranked[0]][1])
    tied = [n for n in ranked if (scores[n][0], scores[n][1]) == top_score]

    if last_project is not None and last_project in tied and len(tied) > 1:
        # Rotate: pick the next project after last_project in config order
        tied_in_order = [n for n in names if n in tied]
        idx = tied_in_order.index(last_project)
        return tied_in_order[(idx + 1) % len(tied_in_order)]

    return tied[0]


def find_repo_root() -> Path | None:
    """Find the repository root directory.

    Tries `git rev-parse --show-toplevel` first, then walks up from CWD
    looking for `ralph/projects.json`.
    """
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode == 0:
            candidate = Path(result.stdout.strip())
            if (candidate / "ralph" / "projects.json").exists():
                return candidate
    except Exception:
        pass

    # Fallback: walk up from CWD
    current = Path.cwd().resolve()
    for parent in [current, *current.parents]:
        if (parent / "ralph" / "projects.json").exists():
            return parent
    return None


def detect_project_from_cwd(
    original_cwd: Path, repo_root: Path, config: RalphConfig
) -> str | None:
    """Detect project name from the current working directory.

    If CWD is inside a configured project directory, return its name.
    Returns None if CWD is the repo root or not a known project.
    """
    try:
        relative = original_cwd.relative_to(repo_root)
    except ValueError:
        return None

    parts = relative.parts
    if not parts:
        return None

    first_component = parts[0]
    if first_component in config.projects:
        return first_component
    return None


def parse_at_mentions(text: str) -> list[str]:
    """Parse @-mentioned file/directory paths from RALPH.md text.

    Extracts patterns like @FEEDBACK.md, @STATUS.md, @specs/ from the prompt.
    Strips trailing punctuation, filters out templated mentions (containing {),
    and returns a deduplicated sorted list of relative paths.
    """
    # Match @ followed by word chars, dots, slashes, hyphens
    raw = re.findall(r"@([\w./-]+)", text)

    seen: set[str] = set()
    result: list[str] = []
    for mention in raw:
        # Strip trailing punctuation (periods, commas)
        cleaned = mention.rstrip(".,;:!?")
        # Skip templated mentions like specs/{task}.md
        if "{" in cleaned:
            continue
        if cleaned and cleaned not in seen:
            seen.add(cleaned)
            result.append(cleaned)

    result.sort()
    return result


def validate_project_files(
    project_dir: Path, at_mentions: list[str]
) -> dict[str, bool]:
    """Check existence of @-mentioned files/directories in a project.

    Args:
        project_dir: Path to the project directory.
        at_mentions: List of relative paths parsed from RALPH.md.

    Returns a dict mapping each path to whether it exists.
    """
    return {path: (project_dir / path).exists() for path in at_mentions}


def print_status_report(config: RalphConfig, project_filter: str | None = None) -> None:
    """Print a status report showing project readiness without invoking Claude."""
    print(f"Ralph config: model={config.model}, max_iterations={config.max_iterations}")
    print(f"Stop file: {config.stop_file}")
    print()

    # Parse @-mentions from RALPH.md
    if not PROMPT_FILE.exists():
        print(f"Error: {PROMPT_FILE} not found", file=sys.stderr)
        sys.exit(1)

    prompt_text = PROMPT_FILE.read_text()
    at_mentions = parse_at_mentions(prompt_text)

    projects = config.projects
    if project_filter and project_filter in projects:
        projects = {project_filter: projects[project_filter]}

    for name, project in projects.items():
        project_dir = Path(name)
        print(f"=== {name} ===")

        # Validate @-mentioned files/directories
        readiness = validate_project_files(project_dir, at_mentions)
        for path, exists in readiness.items():
            marker = "ok" if exists else "MISSING"
            print(f"  @{path}: {marker}")

        # Pending tasks
        pending = count_pending_tasks(name)
        print(f"  Pending tasks: {pending}")

        # Feedback waiting
        has_feedback = check_feedback_waiting(name)
        print(f"  Feedback waiting: {'yes' if has_feedback else 'no'}")

        # Config details
        if project.implementation_plan:
            print(f"  Plan: {project.implementation_plan}")

        print()


def build_prompt(config: RalphConfig, project_filter: str | None = None) -> str:
    """Build the full prompt from RALPH.md template plus project context.

    Args:
        config: The loaded Ralph configuration.
        project_filter: If set, only include this project.

    Returns:
        The complete prompt string to send to Claude.
    """
    if not PROMPT_FILE.exists():
        print(f"Error: {PROMPT_FILE} not found", file=sys.stderr)
        sys.exit(1)

    template = PROMPT_FILE.read_text()

    # Build project context
    lines: list[str] = []
    lines.append("\n---\n")
    lines.append("## Active Projects\n")
    lines.append("| Project | Pending Tasks | Feedback Waiting |")
    lines.append("|---------|---------------|------------------|")

    projects = config.projects
    if project_filter and project_filter in projects:
        projects = {project_filter: projects[project_filter]}

    for name, project in projects.items():
        pending = count_pending_tasks(name)
        feedback = "Yes" if check_feedback_waiting(name) else "No"
        lines.append(f"| {name} | {pending} | {feedback} |")

    lines.append("")

    # Per-project details
    for name, project in projects.items():
        lines.append(f"### Project: {name}")

        if project.implementation_plan:
            lines.append(f"- **Implementation plan:** `{project.implementation_plan}`")
        if project.lint_commands:
            lines.append(f"- **Lint commands:** `{'; '.join(project.lint_commands)}`")
        if project.test_commands:
            lines.append(f"- **Test commands:** `{'; '.join(project.test_commands)}`")

        lines.append("")

    return template + "\n".join(lines)


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


def run_claude(
    prompt_content: str,
    model: str,
    verbose: bool,
    log_file: Path | None,
    cwd: Path | None = None,
) -> str | None:
    """Run claude subprocess and return the last line of output.

    Args:
        prompt_content: The prompt to send to Claude via stdin.
        model: Model name to use.
        verbose: Whether to stream all output.
        log_file: Path to write JSON output to.
        cwd: Working directory for the Claude subprocess. If None, inherits
            the current process's working directory.
    """
    cmd = [
        "claude",
        "--print",
        "--dangerously-skip-permissions",
        "--output-format=stream-json",
        "--verbose",
        "--model",
        model,
    ]

    with subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        cwd=cwd,
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
        description="Ralph loop: multi-project orchestrator that runs claude until STOP_RALPH exists"
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
        default=None,
        help=f"Maximum number of iterations (default: from config or {DEFAULT_MAX_ITERATIONS})",
    )
    parser.add_argument(
        "--config",
        type=Path,
        default=DEFAULT_CONFIG_PATH,
        help=f"Path to projects.json config file (default: {DEFAULT_CONFIG_PATH})",
    )
    parser.add_argument(
        "--project",
        type=str,
        default=None,
        help="Focus on a single project (optional)",
    )
    parser.add_argument(
        "--log-dir",
        type=Path,
        default=DEFAULT_LOG_DIR,
        help=f"Directory to write logs to (default: {DEFAULT_LOG_DIR})",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show project status and readiness without invoking Claude",
    )
    args = parser.parse_args()

    # Capture original CWD before any chdir
    original_cwd = Path.cwd().resolve()

    # Find and chdir to repo root
    repo_root = find_repo_root()
    if repo_root is not None:
        os.chdir(repo_root)
    else:
        # If we can't find repo root, try to proceed from current directory
        pass

    # Resolve --config against original CWD if it's a non-default relative path
    config_path = args.config
    if config_path != DEFAULT_CONFIG_PATH and not config_path.is_absolute():
        config_path = original_cwd / config_path

    # Load config
    config = load_config(config_path)

    # Auto-detect --project from CWD if not set
    if args.project is None and repo_root is not None:
        detected = detect_project_from_cwd(original_cwd, repo_root, config)
        if detected is not None:
            args.project = detected
            print(f"Auto-detected project: {detected}")

    # CLI overrides
    max_iterations = (
        args.max_iterations
        if args.max_iterations is not None
        else config.max_iterations
    )
    stop_marker = Path(config.stop_file)

    # Validate --project if provided
    if args.project and args.project not in config.projects:
        print(
            f"Error: project '{args.project}' not found in config. "
            f"Available: {', '.join(config.projects.keys())}",
            file=sys.stderr,
        )
        sys.exit(1)

    # Dry run: print status and exit
    if args.dry_run:
        print_status_report(config, project_filter=args.project)
        return

    # Validate --project cwd upfront if set
    if args.project:
        fixed_cwd = Path(args.project).resolve()
        if not fixed_cwd.is_dir():
            print(
                f"Error: project directory '{fixed_cwd}' does not exist",
                file=sys.stderr,
            )
            sys.exit(1)

    # Set up logging
    args.log_dir.mkdir(parents=True, exist_ok=True)
    ralph_id = generate_ralph_id()
    log_file = args.log_dir / f"ralph-{ralph_id}.log"
    feedback_log = args.log_dir / f"feedback-{ralph_id}.log"
    print(f"Logging to: {log_file}")

    last_project: str | None = None

    for i in range(max_iterations):
        # Check if we should stop
        if stop_marker.exists():
            log(f"{config.stop_file} found, stopping after {i} iteration(s)", log_file)
            break

        # Select project for this iteration
        if args.project:
            selected = args.project
        else:
            selected = select_project(config, last_project)

        project_cwd = Path(selected).resolve()

        log(
            f"=== Iteration {i + 1}/{max_iterations} [{selected}] === {get_timestamp()}",
            log_file,
            newline_before=True,
        )

        # Fetch feedback for the selected project
        project_cfg = config.projects[selected]
        if project_cfg.feedback_script:
            fetch_feedback(log_file, script_path=Path(project_cfg.feedback_script))

        # Build the prompt scoped to the selected project
        prompt_content = build_prompt(config, project_filter=selected)

        # Log feedback before Claude processes it
        feedback_file = Path(selected) / "FEEDBACK.md"
        git_before = log_feedback_before(feedback_log, feedback_file)

        last_log = {}
        try:
            last_line = run_claude(
                prompt_content, config.model, args.verbose, log_file, cwd=project_cwd
            )
            if last_line is not None:
                last_log = json.loads(last_line)
        except json.JSONDecodeError:
            log(f"Last line failed to parse as JSON: {last_line}", log_file)
            last_project = selected
            continue
        except FileNotFoundError:
            print("Error: 'claude' command not found", file=sys.stderr)
            sys.exit(1)
        except KeyboardInterrupt:
            print("\nInterrupted by user")
            sys.exit(130)

        # Log git state after feedback was processed
        log_feedback_after(feedback_log, git_before)

        last_project = selected

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
                    retry_last_line = run_claude(
                        prompt_content,
                        config.model,
                        args.verbose,
                        log_file,
                        cwd=project_cwd,
                    )
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
