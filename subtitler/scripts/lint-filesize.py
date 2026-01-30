#!/usr/bin/env python3
"""Cross-language file size linter.

Checks all source files (Go, TypeScript, Astro, CSS) for line count limits.
Errors on any file exceeding 1000 lines unless explicitly excepted.

Excludes: vendor/, node_modules/, dist/, .git/, and non-source files.

Usage:
    python3 scripts/lint-filesize.py [--list-exceptions]

Exit codes:
    0: All files within limits
    1: One or more files exceed the limit
"""

import os
import sys

# Maximum lines per source file. Files exceeding this fail the lint.
MAX_LINES = 1000

# Source file extensions to check.
SOURCE_EXTENSIONS = {".go", ".ts", ".astro", ".css"}

# Directories to skip entirely.
SKIP_DIRS = {"vendor", "node_modules", "dist", ".git", "venv", "__pycache__"}

# Files excepted from the limit with justification.
# Format: relative path from project root -> reason
EXCEPTIONS = {
    "backend/api_test_helpers_test.go": "Shared test infrastructure (testServer, registerHandlers, helpers) — cannot split without duplicating across test files",
    "backend/api_video_test.go": "84 video tests covering list/delete/transcription/segments/burn/caching/range/thumbnails/align/embedded — cohesive test group",
    "backend/api_auth_test.go": "66 auth tests covering login/register/sessions/TOTP/password-reset/magic-link/email/CSRF/CAPTCHA — cohesive test group",
    "backend/api_system_test.go": "45 system tests covering health/logs/feedback/admin/metrics/scripts — cohesive test group",
    "backend/api_upload_test.go": "27 upload tests (single + chunked) — at 1011 lines, marginally over limit; splitting would break test readability",
}


def find_source_files(project_root):
    """Walk project tree and yield (relative_path, line_count) for source files."""
    for dirpath, dirnames, filenames in os.walk(project_root):
        # Remove skip dirs in-place to prevent os.walk from descending
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]

        for filename in filenames:
            _, ext = os.path.splitext(filename)
            if ext not in SOURCE_EXTENSIONS:
                continue

            full_path = os.path.join(dirpath, filename)
            rel_path = os.path.relpath(full_path, project_root)

            try:
                with open(full_path, "r", encoding="utf-8", errors="replace") as f:
                    line_count = sum(1 for _ in f)
            except OSError:
                continue

            yield rel_path, line_count


def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)

    if "--list-exceptions" in sys.argv:
        print(f"Files excepted from {MAX_LINES}-line limit:\n")
        for path, reason in sorted(EXCEPTIONS.items()):
            print(f"  {path}")
            print(f"    {reason}\n")
        return 0

    errors = []
    excepted = []

    for rel_path, line_count in sorted(find_source_files(project_root)):
        if line_count <= MAX_LINES:
            continue

        if rel_path in EXCEPTIONS:
            excepted.append((rel_path, line_count))
        else:
            errors.append((rel_path, line_count))

    if excepted:
        for path, count in excepted:
            print(f"EXCEPTED: {path} ({count} lines) — {EXCEPTIONS[path]}")

    if errors:
        print()
        for path, count in errors:
            print(f"ERROR: {path} has {count} lines (max {MAX_LINES})")
        print(f"\nFile size lint FAILED — {len(errors)} file(s) exceed {MAX_LINES} lines.")
        print("Split large files or add an exception with justification.")
        return 1

    if excepted:
        print(f"\nFile size lint passed ({len(excepted)} excepted file(s)).")
    else:
        print("File size lint passed — all files within limits.")

    return 0


if __name__ == "__main__":
    sys.exit(main())
