#!/usr/bin/env python3
"""
Add a new concept to the Peekaboo media database via the admin API.

Secrets resolution order:
  1. Environment variables (PROD_HOST, API_SESSION_ID)
  2. .env file in the project root (KEY=VALUE format, gitignored)

Usage:
  python3 scripts/add-concept.py horse "Horse"
  python3 scripts/add-concept.py sea_turtle "Sea Turtle"

Exit codes:
  0 - Concept created successfully
  1 - Concept already exists (409 conflict)
  2 - Error (auth failure, network, missing config, validation)
"""

import argparse
import json
import sys
import urllib.error
import urllib.request
from pathlib import Path

PROJECT_DIR = Path(__file__).resolve().parent.parent
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
        value = value.strip().strip("'\"")
        env[key.strip()] = value
    return env


def resolve_secrets() -> dict[str, str]:
    """Resolve secrets from env vars and .env file."""
    import os

    secrets = load_env_file(ENV_FILE)
    for key in REQUIRED_KEYS:
        val = os.environ.get(key)
        if val:
            secrets[key] = val
    return secrets


def create_concept(host: str, session_id: str, concept_id: str, name: str) -> int:
    """Call POST /api/admin/concepts and return exit code."""
    url = f"{host}/api/admin/concepts"
    payload = json.dumps({"id": concept_id, "name": name}).encode("utf-8")

    req = urllib.request.Request(url, data=payload, method="POST")
    req.add_header("Authorization", f"Bearer {session_id}")
    req.add_header("Content-Type", "application/json")
    req.add_header("User-Agent", "peekaboo-admin/1.0")

    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            body = resp.read().decode("utf-8")
            data = json.loads(body)
            print(f"Created concept: {data.get('id')} ({data.get('name')})")
            return 0
    except urllib.error.HTTPError as e:
        body = ""
        if e.fp:
            body = e.fp.read().decode("utf-8", errors="replace")

        if e.code == 401:
            print("Error: Authentication failed (401). Check API_SESSION_ID.", file=sys.stderr)
            return 2
        if e.code == 403:
            print("Error: Forbidden (403). User is not in TRUSTED_USERS.", file=sys.stderr)
            return 2
        if e.code == 409:
            print(f"Concept '{concept_id}' already exists.", file=sys.stderr)
            return 1
        if e.code == 400:
            try:
                data = json.loads(body)
                print(f"Error: {data.get('error', 'bad request')}", file=sys.stderr)
            except json.JSONDecodeError:
                print(f"Error: Bad request (400): {body}", file=sys.stderr)
            return 2

        print(f"Error: API returned HTTP {e.code}", file=sys.stderr)
        if body:
            print(body, file=sys.stderr)
        return 2
    except urllib.error.URLError as e:
        print(f"Error: Network error: {e.reason}", file=sys.stderr)
        return 2


def main(argv: list[str] | None = None) -> int:
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("concept_id", help="Concept ID (lowercase, underscores, e.g. 'horse')")
    parser.add_argument("name", help="Display name (e.g. 'Horse')")
    args = parser.parse_args(argv)

    secrets = resolve_secrets()
    missing = [k for k in REQUIRED_KEYS if k not in secrets or not secrets[k]]
    if missing:
        print(f"Error: Missing required secrets: {', '.join(missing)}", file=sys.stderr)
        print("Provide them via environment variables or .env file", file=sys.stderr)
        return 2

    host = secrets["PROD_HOST"].rstrip("/")
    session_id = secrets["API_SESSION_ID"]

    return create_concept(host, session_id, args.concept_id, args.name)


if __name__ == "__main__":
    sys.exit(main())
