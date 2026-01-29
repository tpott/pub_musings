#!/usr/bin/env python3
"""
Fetch new user feedback from production and write to FEEDBACK.md.

Secrets resolution order:
  1. Environment variables (PROD_HOST, API_SESSION_ID)
  2. .env file in the project root (KEY=VALUE format, gitignored)
  3. secrets.enc.yaml decrypted via sops (git-tracked, encrypted with age)

Uses .feedback-cursor (gitignored) to track the last-fetched timestamp
so repeated runs only fetch new items.

Exit codes:
  0 - New feedback written to FEEDBACK.md
  1 - No new feedback found
  2 - Error (auth failure, network, missing config)
"""

import json
import os
import shutil
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

PROJECT_DIR = Path(__file__).resolve().parent.parent
CURSOR_FILE = PROJECT_DIR / ".feedback-cursor"
OUTPUT_FILE = PROJECT_DIR / "FEEDBACK.md"
ENV_FILE = PROJECT_DIR / ".env"
SECRETS_ENC_FILE = PROJECT_DIR / "secrets.enc.yaml"

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


def load_sops_secrets(path: Path) -> dict[str, str]:
    """Decrypt secrets.enc.yaml via sops and return as dict."""
    sops_bin = shutil.which("sops")
    if sops_bin is None:
        return {}
    if not path.is_file():
        return {}
    try:
        result = subprocess.run(
            [sops_bin, "--decrypt", str(path)],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode != 0:
            print(f"Warning: sops decrypt failed: {result.stderr.strip()}", file=sys.stderr)
            return {}
    except subprocess.TimeoutExpired:
        print("Warning: sops decrypt timed out", file=sys.stderr)
        return {}

    # Parse YAML manually (key: value format, no nesting expected)
    secrets: dict[str, str] = {}
    for line in result.stdout.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if ":" not in line:
            continue
        key, _, value = line.partition(":")
        key = key.strip()
        value = value.strip().strip("'\"")
        # Map yaml keys to env var names
        env_key = key.upper()
        secrets[env_key] = value
    return secrets


def resolve_secrets() -> dict[str, str]:
    """
    Resolve secrets from multiple sources in priority order:
      1. Environment variables
      2. .env file
      3. secrets.enc.yaml via sops
    """
    # Start with sops (lowest priority)
    secrets = load_sops_secrets(SECRETS_ENC_FILE)

    # Layer .env file on top
    env_vars = load_env_file(ENV_FILE)
    secrets.update(env_vars)

    # Layer actual environment on top (highest priority)
    for key in REQUIRED_KEYS:
        val = os.environ.get(key)
        if val:
            secrets[key] = val

    return secrets


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
        fb_type = item.get("feedback_type", "general")
        text = item.get("feedback_text", "").replace("\n", " ").strip()
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


def main(argv: list[str] | None = None) -> int:
    """Main entry point. Returns exit code."""
    if argv is None:
        argv = sys.argv[1:]

    if "--help" in argv or "-h" in argv:
        print(__doc__.strip())
        print()
        print("Usage: fetch-feedback.py [--help]")
        print()
        print("Secrets can be provided via:")
        print("  1. Environment variables: PROD_HOST, API_SESSION_ID")
        print("  2. .env file in the project root (KEY=VALUE)")
        print("  3. secrets.enc.yaml (decrypted via sops)")
        return 0

    secrets = resolve_secrets()

    missing = [k for k in REQUIRED_KEYS if k not in secrets or not secrets[k]]
    if missing:
        print(f"Error: Missing required secrets: {', '.join(missing)}", file=sys.stderr)
        print("Provide them via environment variables, .env, or secrets.enc.yaml", file=sys.stderr)
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
