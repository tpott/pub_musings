#!/usr/bin/env python3
"""Convert whisper-formatted subtitles to SRT format."""

import argparse
import re
import sys
from typing import Optional, Tuple


def parse_whisper_line(line: str) -> Optional[Tuple[str, str, str]]:
    """Parse a whisper subtitle line and return (start, end, text) or None."""
    # Pattern: [HH:MM:SS.mmm --> HH:MM:SS.mmm]   text
    pattern = r'\[(\d{2}:\d{2}:\d{2}\.\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}\.\d{3})\]\s*(.*)'
    match = re.match(pattern, line.strip())
    if match is not None:
        start, end, text = match.groups()
        return start, end, text.strip()
    return None


def convert_timestamp(ts: str) -> str:
    """Convert whisper timestamp (periods) to SRT format (commas)."""
    return ts.replace('.', ',')


def convert_whisper_to_srt(input_file: str, output_file: Optional[str]=None):
    """Convert whisper subtitle file to SRT format."""
    with open(input_file, 'r', encoding='utf-8') as f:
        lines = f.readlines()

    srt_entries = []
    seq = 1

    for line in lines:
        parsed = parse_whisper_line(line)
        if parsed is None:
            continue
        start, end, text = parsed
        if len(text) > 0:  # Only include entries with text
            srt_entry = f"{seq}\n{convert_timestamp(start)} --> {convert_timestamp(end)}\n{text}\n"
            srt_entries.append(srt_entry)
            seq += 1

    output = '\n'.join(srt_entries)

    if output_file is not None:
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write(output)
    else:
        print(output)


def main():
    parser = argparse.ArgumentParser(description='Convert whisper subtitles to SRT format')
    parser.add_argument('input', help='Input whisper subtitle file')
    parser.add_argument('output', nargs='?', help='Output SRT file (stdout if not specified)')
    args = parser.parse_args()

    convert_whisper_to_srt(args.input, args.output)


if __name__ == '__main__':
    main()
