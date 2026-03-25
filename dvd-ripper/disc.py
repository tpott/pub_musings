"""Disc label parsing and episode numbering."""

import re
from pathlib import Path


# Patterns for parsing season/disc info from disc labels
_SEASON_RE = re.compile(
    r'[_\s](?:S|Season[_\s]?|Book[_\s]?)(\d+)', re.IGNORECASE)
_DISC_RE = re.compile(
    r'[_\s](?:D|Disc[_\s]?)(\d+)', re.IGNORECASE)


def parse_disc_label(label, media_type="movie"):
    """Parse a disc label into structured metadata.

    For TV: extracts show_name, season, disc from labels like
        "Avatar_Book_1_Disc_1" or "BREAKING_BAD_S3_D2"
    For movies: returns movie_name as-is.
    """
    if media_type == "movie":
        return {"movie_name": label}

    season_match = _SEASON_RE.search(label)
    disc_match = _DISC_RE.search(label)

    season = int(season_match.group(1)) if season_match else 1
    disc = int(disc_match.group(1)) if disc_match else 1

    # Strip season/disc suffixes to get the show name
    name = label
    # Remove from the earliest match onward
    cut_positions = []
    if season_match:
        cut_positions.append(season_match.start())
    if disc_match:
        cut_positions.append(disc_match.start())
    if cut_positions:
        name = label[:min(cut_positions)]

    # Clean up: underscores to spaces, strip trailing separators
    name = name.replace("_", " ").strip(" -")

    return {"show_name": name, "season": season, "disc": disc}


def compute_episode_start(output_dir, disc, ep_count):
    """Compute the starting episode number for a TV disc.

    For disc 1, always starts at 1. For disc N>1, requires that all
    previous discs have been ripped (sequential order enforced).

    Looks at existing .mp4 files named like "Show S01E03.mp4" in output_dir
    to determine how many episodes have already been ripped.
    """
    if disc == 1:
        return 1

    existing = list(Path(output_dir).glob("*.mp4"))
    ep_numbers = []
    for f in existing:
        m = re.search(r'S\d+E(\d+)\.mp4$', f.name)
        if m:
            ep_numbers.append(int(m.group(1)))

    if not ep_numbers:
        raise RuntimeError(
            f"Disc {disc} cannot be ripped before disc 1. "
            f"No existing episodes found in {output_dir}. "
            f"Please insert discs in order starting from disc 1."
        )

    expected_prior = disc - 1
    max_ep = max(ep_numbers)
    num_existing = len(ep_numbers)

    # Check that the existing episodes form a contiguous range 1..N
    expected_set = set(range(1, num_existing + 1))
    if set(ep_numbers) != expected_set:
        raise RuntimeError(
            f"Disc {disc}: expected contiguous episodes 1-{num_existing} "
            f"but found gaps in {output_dir}. "
            f"Please re-rip missing discs first."
        )

    return max_ep + 1
