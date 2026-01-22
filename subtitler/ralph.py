#!/usr/bin/env python3
"""
Ralph loop: repeatedly runs claude with RALPH.md until STOP_RALPH exists.
"""

import argparse
import base64
import json
import os
import subprocess
import sys
import time
from datetime import datetime, timezone
from collections.abc import Iterable
from pathlib import Path

DEFAULT_MAX_ITERATIONS = 10
CLAUDE_MODEL = "opus"
PROMPT_FILE = "RALPH.md"
STOP_FILE = "STOP_RALPH"


def generate_ralph_id() -> str:
    """Generate an 8-character base32 ID (40 bits of entropy)."""
    random_bytes = os.urandom(5)  # 5 bytes = 40 bits
    return base64.b32encode(random_bytes).decode("ascii").lower()


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


def process_claude_output(lines: Iterable[str], verbose: bool) -> str | None:
    """
    Process lines from claude output, return the last line.

    If verbose, streams all output.
    If not verbose, prints session_id once, then '.' for each line received.
    """
    last_line = None
    session_id_printed = False

    for line in lines:
        last_line = line.rstrip("\n")
        if verbose:
            print(line, end="", flush=True)
            continue
        # Print session_id once before the dots
        if not session_id_printed:
            data: dict[str, str] = {}
            try:
                data = json.loads(last_line)
            except json.JSONDecodeError:
                pass
            if "session_id" in data:
                print(f"session_id: {data['session_id']}")
                session_id_printed = True
        print(".", end="", flush=True)

    if not verbose:
        print()  # Newline after dots

    return last_line


def run_claude(prompt_content: str, verbose: bool) -> str | None:
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

        last_line = process_claude_output(proc.stdout, verbose)

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
    args = parser.parse_args()

    max_iterations = args.max_iterations
    prompt_file = Path(PROMPT_FILE)
    stop_marker = Path(STOP_FILE)

    # Set up logging if requested
    log_file = None
    if args.log_dir:
        args.log_dir.mkdir(parents=True, exist_ok=True)
        log_file = args.log_dir / f"ralph-{generate_ralph_id()}.log"
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

        # Read the prompt file
        if not prompt_file.exists():
            print(f"Error: {prompt_file} not found", file=sys.stderr)
            sys.exit(1)

        prompt_content = prompt_file.read_text()

        try:
            last_line = run_claude(prompt_content, args.verbose)
            # If ralph is run with --verbose then skip logging the Result so it's
            # easier to parse with `jq`. Grep for `"type":"result","subtype":"success"`
            if last_line is not None and not args.verbose:
                result_text = json.loads(last_line)["result"]
                log(f"Result: {result_text}", log_file)
        except json.JSONDecodeError:
            log(f"Last line failed to parse as JSON: {last_line}", log_file)
            continue
        except KeyError:
            log(f'Last line missing "result": {last_line}', log_file)
            continue
        except FileNotFoundError:
            print("Error: 'claude' command not found", file=sys.stderr)
            sys.exit(1)
        except KeyboardInterrupt:
            print("\nInterrupted by user")
            sys.exit(130)

    else:
        log(f"Completed {max_iterations} iterations", log_file)


if __name__ == "__main__":
    main()
