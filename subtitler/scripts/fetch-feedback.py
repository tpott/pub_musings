#!/usr/bin/env python3
"""
Fetch new user feedback from production and write to FEEDBACK.md.

Secrets resolution order:
  1. Environment variables (PROD_HOST, API_SESSION_ID)
  2. .env file in the project root (KEY=VALUE format, gitignored)

Optional filtering:
  TRUSTED_USERS - Comma-separated list of user IDs. When set, only feedback
  from these users is included. Anonymous feedback (no user_id) is excluded.

Uses .feedback-cursor (gitignored) to track the last-fetched timestamp
so repeated runs only fetch new items.

Exit codes:
  0 - New feedback written to FEEDBACK.md
  1 - No new feedback found
  2 - Error (auth failure, network, missing config)
"""

import argparse
import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path

PROJECT_DIR = Path(__file__).resolve().parent.parent
CURSOR_FILE = PROJECT_DIR / ".feedback-cursor"
OUTPUT_FILE = PROJECT_DIR / "FEEDBACK.md"
ENV_FILE = PROJECT_DIR / ".env"

REQUIRED_KEYS = ("PROD_HOST", "API_SESSION_ID")


def load_env_file(path: Path) -> dict[str, str]:
    """Parse a KEY=VALUE file, ignoring comments and blank lines."""
    env: dict[str, str] = {}
    if not path.is_file():
        return env
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            continue
        key, _, value = line.partition("=")
        # Strip optional quotes
        value = value.strip().strip("'\"")
        env[key.strip()] = value
    return env


def resolve_secrets() -> dict[str, str]:
    """
    Resolve secrets from multiple sources in priority order:
      1. Environment variables (highest priority)
      2. .env file
    """
    # Start with .env file (lower priority)
    secrets = load_env_file(ENV_FILE)

    # Layer actual environment on top (highest priority)
    for key in (*REQUIRED_KEYS, "TRUSTED_USERS"):
        val = os.environ.get(key)
        if val:
            secrets[key] = val

    return secrets


def parse_trusted_users(raw: str) -> set[str]:
    """Parse a comma-separated TRUSTED_USERS string into a set of user IDs."""
    return {u.strip() for u in raw.split(",") if u.strip()}


def filter_trusted(items: list[dict], trusted: set[str]) -> list[dict]:
    """Keep only feedback items whose user_id is in the trusted set."""
    return [item for item in items if item.get("user_id") in trusted]


def read_cursor() -> str | None:
    """Read the cursor timestamp from .feedback-cursor, or None if missing."""
    if not CURSOR_FILE.is_file():
        return None
    text = CURSOR_FILE.read_text().strip()
    return text if text else None


def write_cursor(timestamp: str) -> None:
    """Write the newest timestamp to .feedback-cursor."""
    CURSOR_FILE.write_text(timestamp + "\n")


def fetch_feedback_api(host: str, session_id: str, after: str | None) -> tuple[int, str]:
    """
    Call the feedback API and return (status_code, body).

    Uses urllib to avoid adding a requests dependency.
    """
    url = f"{host}/api/admin/feedback?status=new&limit=50"
    if after:
        url += f"&after={after}"

    req = urllib.request.Request(url)
    req.add_header("Authorization", f"Bearer {session_id}")
    req.add_header("User-Agent", "subtitler-feedback/1.0")

    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            return resp.status, resp.read().decode("utf-8")
    except urllib.error.HTTPError as e:
        body = ""
        if e.fp:
            body = e.fp.read().decode("utf-8", errors="replace")
        return e.code, body
    except urllib.error.URLError as e:
        print(f"Error: Network error: {e.reason}", file=sys.stderr)
        sys.exit(2)


def format_feedback(items: list[dict]) -> tuple[list[str], str]:
    """
    Format feedback items as markdown lines.

    Returns (lines, newest_timestamp).
    """
    lines: list[str] = []
    newest_ts = ""

    for item in items:
        fb_type = item.get("type", "general")
        text = item.get("text", "").replace("\n", " ").strip()
        created = item.get("created_at", "")
        page = item.get("page_url", "")
        rating = item.get("rating")

        parts = [f"[{fb_type}]"]
        if rating:
            parts.append(f"Rating: {rating}/5")
        parts.append(f'- "{text}"')
        if created:
            date_part = created[:10] if len(created) >= 10 else created
            suffix = f", page: {page})" if page else ")"
            parts.append(f"({date_part}{suffix}")
        lines.append("* " + " ".join(parts))

        if created > newest_ts:
            newest_ts = created

    return lines, newest_ts


def build_parser() -> argparse.ArgumentParser:
    """Build the CLI argument parser."""
    parser = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "Secrets can be provided via:\n"
            "  1. Environment variables: PROD_HOST, API_SESSION_ID\n"
            "  2. .env file in the project root (KEY=VALUE)\n"
            "\n"
            "Optional environment variables:\n"
            "  TRUSTED_USERS - Comma-separated user IDs to filter by\n"
        ),
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    """Main entry point. Returns exit code."""
    parser = build_parser()
    parser.parse_args(argv)

    secrets = resolve_secrets()

    missing = [k for k in REQUIRED_KEYS if k not in secrets or not secrets[k]]
    if missing:
        print(f"Error: Missing required secrets: {', '.join(missing)}", file=sys.stderr)
        print("Provide them via environment variables or .env file", file=sys.stderr)
        sys.exit(2)

    host = secrets["PROD_HOST"].rstrip("/")
    session_id = secrets["API_SESSION_ID"]
    cursor = read_cursor()

    status, body = fetch_feedback_api(host, session_id, cursor)

    if status == 401:
        print("Error: Authentication failed (401). Update API_SESSION_ID.", file=sys.stderr)
        sys.exit(2)

    if status != 200:
        print(f"Error: API returned HTTP {status}", file=sys.stderr)
        print(body, file=sys.stderr)
        sys.exit(2)

    try:
        data = json.loads(body)
    except json.JSONDecodeError as e:
        print(f"Error: Failed to parse API response: {e}", file=sys.stderr)
        sys.exit(2)

    items = data.get("feedback") or []
    if not items:
        print("No new feedback found.")
        return 1

    trusted_raw = secrets.get("TRUSTED_USERS", "")
    trusted = parse_trusted_users(trusted_raw)
    if len(trusted) > 0:
        total = len(items)
        items = filter_trusted(items, trusted)
        skipped = total - len(items)
        if skipped:
            print(f"Filtered out {skipped} item(s) from non-trusted users.")
        if not items:
            print("No new feedback from trusted users.")
            return 1

    lines, newest_ts = format_feedback(items)

    # Write/append to FEEDBACK.md
    content = "\n".join(lines) + "\n"
    if OUTPUT_FILE.is_file():
        with open(OUTPUT_FILE, "a") as f:
            f.write(content)
    else:
        OUTPUT_FILE.write_text(content)

    # Update cursor
    if newest_ts:
        write_cursor(newest_ts)

    print(f"Wrote {len(items)} feedback item(s) to FEEDBACK.md")
    return 0


if __name__ == "__main__":
    sys.exit(main())
