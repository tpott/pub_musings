#!/usr/bin/env python3
"""
Query and filter security events from subtitler logs.

Reads log lines from stdin or journalctl and filters security events by
event type, IP address, user (email or user_id), date range, and log level.

Exit codes:
  0 - Matching events found and printed
  1 - No matching events found
  2 - Error (invalid arguments, etc.)
"""

import argparse
import re
import sys
from datetime import datetime, timezone


# Regex to parse slog TextHandler logfmt output.
# Handles both quoted and unquoted values.
# Examples:
#   key=value
#   key="quoted value"
#   key="value with \"escapes\""
LOGFMT_PAIR = re.compile(
    r'(\w+)='
    r'(?:"((?:[^"\\]|\\.)*)"|(\S*))'
)


def parse_logfmt(line: str) -> dict[str, str]:
    """Parse a logfmt line into a dict of key-value pairs."""
    fields: dict[str, str] = {}
    for match in LOGFMT_PAIR.finditer(line):
        key = match.group(1)
        # Group 2 is quoted value, group 3 is unquoted
        value = match.group(2) if match.group(2) is not None else match.group(3)
        fields[key] = value
    return fields


def is_security_event(fields: dict[str, str]) -> bool:
    """Check if a parsed log line is a security event."""
    msg = fields.get("msg", "")
    return msg in ("Security event", "Security warning")


def parse_timestamp(ts: str) -> datetime | None:
    """Parse a slog timestamp string to datetime."""
    if not ts:
        return None
    # slog TextHandler uses RFC3339 / ISO 8601
    # Examples: 2024-01-29T10:30:00.000Z, 2024-01-29T10:30:00.000+00:00
    try:
        # Python 3.11+ handles Z suffix in fromisoformat
        return datetime.fromisoformat(ts)
    except ValueError:
        pass
    # Fallback: replace Z with +00:00
    try:
        return datetime.fromisoformat(ts.replace("Z", "+00:00"))
    except ValueError:
        return None


def parse_date_arg(value: str) -> datetime:
    """
    Parse a date/datetime argument.

    Accepts:
      - "2024-01-29" (date only, treated as start of day UTC)
      - "2024-01-29T10:30:00" (datetime, treated as UTC)
      - ISO 8601 with timezone
    """
    # Date only
    if re.match(r"^\d{4}-\d{2}-\d{2}$", value):
        return datetime.fromisoformat(value + "T00:00:00+00:00")
    # Datetime without timezone - assume UTC
    try:
        dt = datetime.fromisoformat(value)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt
    except ValueError:
        raise argparse.ArgumentTypeError(
            f"Invalid date/datetime: {value!r}. "
            f"Use YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS"
        )


def matches_filters(
    fields: dict[str, str],
    event_filter: str | None,
    ip_filter: str | None,
    user_filter: str | None,
    level_filter: str | None,
    since: datetime | None,
    until: datetime | None,
) -> bool:
    """Check if a security event matches all specified filters."""
    # Event type filter (prefix match: "auth" matches "auth.login.success")
    if event_filter is not None:
        event = fields.get("event", "")
        if not event.startswith(event_filter):
            return False

    # IP filter (exact match)
    if ip_filter is not None:
        if fields.get("ip", "") != ip_filter:
            return False

    # User filter (matches email or user_id)
    if user_filter is not None:
        email = fields.get("email", "")
        user_id = fields.get("user_id", "")
        admin_user_id = fields.get("admin_user_id", "")
        if user_filter not in (email, user_id, admin_user_id):
            return False

    # Level filter (exact, case-insensitive)
    if level_filter is not None:
        if fields.get("level", "").upper() != level_filter.upper():
            return False

    # Date range filters
    if since is not None or until is not None:
        ts = parse_timestamp(fields.get("time", ""))
        if ts is None:
            return False
        if since is not None and ts < since:
            return False
        if until is not None and ts > until:
            return False

    return True


def format_event(fields: dict[str, str], output_format: str) -> str:
    """Format a security event for output."""
    if output_format == "raw":
        # Reconstruct logfmt line
        parts = []
        for key, value in fields.items():
            if " " in value or '"' in value or "=" in value:
                value = '"' + value.replace("\\", "\\\\").replace('"', '\\"') + '"'
            parts.append(f"{key}={value}")
        return " ".join(parts)

    if output_format == "short":
        ts = fields.get("time", "")[:19]  # Trim to seconds
        level = fields.get("level", "")
        event = fields.get("event", "")
        ip = fields.get("ip", "")
        # Build a concise summary
        extra_parts = []
        for key in ("email", "user_id", "endpoint", "resource", "reason",
                     "attempt_count", "limit_name", "filename"):
            if key in fields:
                extra_parts.append(f"{key}={fields[key]}")
        extra = " ".join(extra_parts)
        return f"{ts} {level:<4} {event:<40} ip={ip:<15} {extra}"

    # Default: table-like
    return format_event(fields, "short")


def count_by_field(events: list[dict[str, str]], field: str) -> dict[str, int]:
    """Count events grouped by a field value."""
    counts: dict[str, int] = {}
    for fields in events:
        value = fields.get(field, "(none)")
        counts[value] = counts.get(value, 0) + 1
    # Sort by count descending
    return dict(sorted(counts.items(), key=lambda x: x[1], reverse=True))


def build_parser() -> argparse.ArgumentParser:
    """Build the argument parser."""
    parser = argparse.ArgumentParser(
        prog="query-security-events",
        description="Query and filter security events from subtitler logs.",
        epilog=(
            "Examples:\n"
            "  # Pipe from journalctl\n"
            "  journalctl -u subtitler --since today | %(prog)s\n"
            "\n"
            "  # Filter failed logins\n"
            "  journalctl -u subtitler | %(prog)s --event auth.login.failed\n"
            "\n"
            "  # Events from a specific IP in the last hour\n"
            '  journalctl -u subtitler --since "1 hour ago" | %(prog)s --ip 192.168.1.100\n'
            "\n"
            "  # All events for a user\n"
            "  journalctl -u subtitler | %(prog)s --user user@example.com\n"
            "\n"
            "  # Count events by type\n"
            "  journalctl -u subtitler --since today | %(prog)s --count-by event\n"
            "\n"
            "  # Warnings only in a date range\n"
            "  journalctl -u subtitler | %(prog)s --level WARN --since 2024-01-01 --until 2024-01-31\n"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )

    # Filters
    parser.add_argument(
        "--event", "-e",
        metavar="TYPE",
        help=(
            "Filter by event type (prefix match). "
            'E.g., "auth" matches all auth events, '
            '"auth.login.failed" matches all failed logins.'
        ),
    )
    parser.add_argument(
        "--ip", "-i",
        metavar="ADDR",
        help="Filter by client IP address (exact match).",
    )
    parser.add_argument(
        "--user", "-u",
        metavar="EMAIL_OR_ID",
        help="Filter by user email or user_id (exact match).",
    )
    parser.add_argument(
        "--level", "-l",
        metavar="LEVEL",
        choices=["INFO", "WARN", "ERROR", "DEBUG"],
        help="Filter by log level (INFO, WARN, ERROR, DEBUG).",
    )
    parser.add_argument(
        "--since", "-s",
        metavar="DATE",
        type=parse_date_arg,
        help="Show events on or after this date (YYYY-MM-DD or ISO 8601).",
    )
    parser.add_argument(
        "--until", "-U",
        metavar="DATE",
        type=parse_date_arg,
        help="Show events on or before this date (YYYY-MM-DD or ISO 8601).",
    )

    # Output options
    parser.add_argument(
        "--format", "-f",
        choices=["short", "raw"],
        default="short",
        help="Output format: short (default) or raw (original logfmt).",
    )
    parser.add_argument(
        "--count-by", "-c",
        metavar="FIELD",
        help=(
            "Instead of listing events, count them grouped by FIELD. "
            "Common fields: event, ip, email, user_id, level, endpoint."
        ),
    )
    parser.add_argument(
        "--tail", "-t",
        metavar="N",
        type=int,
        help="Show only the last N matching events.",
    )

    return parser


def main(argv: list[str] | None = None) -> int:
    """Main entry point. Returns exit code."""
    parser = build_parser()
    args = parser.parse_args(argv)

    if sys.stdin.isatty():
        parser.print_usage(sys.stderr)
        print(
            "\nError: No input. Pipe log data via stdin.\n"
            "Example: journalctl -u subtitler --since today | query-security-events.py\n"
            "\nUse --help for more information.",
            file=sys.stderr,
        )
        return 2

    matched_events: list[dict[str, str]] = []

    for line in sys.stdin:
        line = line.rstrip("\n")
        if not line:
            continue

        # Quick pre-filter before full parse
        if "Security event" not in line and "Security warning" not in line:
            continue

        fields = parse_logfmt(line)
        if not is_security_event(fields):
            continue

        if not matches_filters(
            fields,
            event_filter=args.event,
            ip_filter=args.ip,
            user_filter=args.user,
            level_filter=args.level,
            since=args.since,
            until=args.until,
        ):
            continue

        matched_events.append(fields)

    if not matched_events:
        print("No matching security events found.", file=sys.stderr)
        return 1

    # Apply --tail
    if args.tail is not None and args.tail > 0:
        matched_events = matched_events[-args.tail:]

    # Output
    if args.count_by:
        counts = count_by_field(matched_events, args.count_by)
        # Print header
        print(f"{'Count':>6}  {args.count_by}")
        print(f"{'-----':>6}  {'---' * 10}")
        for value, count in counts.items():
            print(f"{count:>6}  {value}")
        print(f"\nTotal: {sum(counts.values())} events")
    else:
        for fields in matched_events:
            print(format_event(fields, args.format))

    return 0


if __name__ == "__main__":
    sys.exit(main())
