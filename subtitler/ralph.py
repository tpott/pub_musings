#!/usr/bin/env python3
"""
Ralph loop: repeatedly runs claude with RALPH.md until STOP_RALPH exists.
"""

import argparse
import json
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

CLAUDE_MODEL = "opus"
PROMPT_FILE = "RALPH.md"
STOP_FILE = "STOP_RALPH"


def get_timestamp():
    """Return formatted timestamp string with local, UTC, and epoch times."""
    now = datetime.now()
    utc_now = datetime.now(timezone.utc)
    epoch_ms = time.time()

    local_str = now.strftime("%Y-%m-%d %H:%M:%S %Z").strip()
    utc_str = utc_now.strftime("%Y-%m-%d %H:%M:%S UTC")

    return f"{local_str} | {utc_str} | {epoch_ms:.3f}"


def run_claude(prompt_content: str, verbose: bool) -> str | None:
    """
    Run claude and return the last line of output.

    If verbose, streams all JSON output.
    If not verbose, prints a '.' for each line received.
    """
    cmd = [
        "claude",
        "--print",
        "--dangerously-skip-permissions",
        "--output-format=stream-json",
        "--verbose",
        "--model", CLAUDE_MODEL,
    ]

    last_line = None

    with subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    ) as proc:
        # Send prompt and close stdin
        proc.stdin.write(prompt_content)
        proc.stdin.close()

        # Read output line by line
        for line in proc.stdout:
            last_line = line.rstrip('\n')
            if verbose:
                print(line, end='', flush=True)
            else:
                print('.', end='', flush=True)

        # Wait for process to complete
        proc.wait()

        if proc.returncode != 0:
            print(f"Process exited with code {proc.returncode}")

        stderr_output = proc.stderr.read()
        if stderr_output:
            print(f"STDERR: {stderr_output}", file=sys.stderr)

        if not verbose:
            print()  # Newline after dots

    return last_line


def main():
    parser = argparse.ArgumentParser(
        description="Ralph loop: repeatedly runs claude until STOP_RALPH exists"
    )
    parser.add_argument(
        "-v", "--verbose",
        action="store_true",
        help="Pass through all streamed JSON output (default: show dots for progress)"
    )
    args = parser.parse_args()

    max_iterations = 10
    prompt_file = Path(PROMPT_FILE)
    complete_marker = Path(STOP_FILE)

    for i in range(max_iterations):
        # Check if we should stop
        if complete_marker.exists():
            print(f"{STOP_FILE} found, stopping after {i} iteration(s)")
            break

        print(f"=== Iteration {i}/{max_iterations} === {get_timestamp()}")

        # Read the prompt file
        if not prompt_file.exists():
            print(f"Error: {prompt_file} not found", file=sys.stderr)
            sys.exit(1)

        prompt_content = prompt_file.read_text()

        try:
            last_line = run_claude(prompt_content, args.verbose)
            if last_line is not None:
                print(f"Result: {json.loads(last_line)['result']}")
        except json.JSONDecodeError:
            print(f"Last line failed to parse as JSON: {last_line}")
            continue
        except KeyError:
            print(f"Last line missing \"result\": {last_line}")
            continue
        except FileNotFoundError:
            print("Error: 'claude' command not found", file=sys.stderr)
            sys.exit(1)
        except KeyboardInterrupt:
            print("\nInterrupted by user")
            sys.exit(130)

    else:
        print(f"Completed {max_iterations} iterations")


if __name__ == "__main__":
    main()
