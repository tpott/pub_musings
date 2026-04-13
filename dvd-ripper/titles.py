"""MakeMKV output parsing and title selection logic."""

import re
import statistics
from collections import defaultdict


def parse_makemkv_info(output):
    """Parse makemkvcon --robot info output into a list of title dicts.

    Each dict contains:
        id (int), name (str), duration_secs (int), segment_count (int),
        segments (str), filename (str)
    """
    # TINFO field IDs: 2=name, 9=duration, 25=segment_count, 26=segments, 27=filename
    raw = defaultdict(dict)
    for line in output.splitlines():
        m = re.match(r'^TINFO:(\d+),(\d+),\d+,"(.*)"', line)
        if not m:
            continue
        title_id = int(m.group(1))
        field_id = int(m.group(2))
        value = m.group(3)
        raw[title_id][field_id] = value

    titles = []
    for tid in sorted(raw):
        fields = raw[tid]
        duration_str = fields.get(9, "0:00:00")
        parts = duration_str.split(":")
        secs = int(parts[0]) * 3600 + int(parts[1]) * 60 + int(parts[2])
        titles.append({
            "id": tid,
            "name": fields.get(2, ""),
            "duration_secs": secs,
            "segment_count": int(fields.get(25, "1")),
            "segments": fields.get(26, ""),
            "filename": fields.get(27, ""),
        })
    return titles


def detect_media_type(titles):
    """Detect whether a disc is a movie or TV show based on title durations.

    Returns "tv" if 3+ titles cluster in the episode range (5-65 min)
    within 30% of the median duration. Otherwise returns "movie".
    """
    episode_range = [t for t in titles if 300 <= t["duration_secs"] <= 3900]
    if len(episode_range) < 3:
        return "movie"
    durations = [t["duration_secs"] for t in episode_range]
    median = statistics.median(durations)
    cluster = [d for d in durations if abs(d - median) / median <= 0.30]
    if len(cluster) >= 3:
        return "tv"
    return "movie"


def select_movie_title(titles):
    """Select the longest title (by duration) for movie ripping."""
    return max(titles, key=lambda t: t["duration_secs"])


def _discover_from_play_all(titles):
    """Discover episodes using the play-all title for segment identification.

    If a multi-segment play-all title exists (4+ segments), use it to
    identify episode segments. This avoids the duration filter excluding
    two-part episodes that are double the typical episode length.

    Returns episodes in play-all order, or None if no usable play-all found.
    """
    multi = [t for t in titles if t["segment_count"] > 1]
    if not multi:
        return None

    # Play-all: the multi-segment title with the most segments
    play_all = max(multi, key=lambda t: t["segment_count"])
    if play_all["segment_count"] < 4:
        return None

    # Bumper segments appear as the first segment in 2+ multi-segment titles
    first_seg_counts = {}
    for t in multi:
        first_seg = t["segments"].split(",")[0]
        first_seg_counts[first_seg] = first_seg_counts.get(first_seg, 0) + 1
    bumper_segs = {seg for seg, count in first_seg_counts.items() if count >= 2}

    # Episode segments: play-all segments minus bumpers
    play_all_segs = play_all["segments"].split(",")
    episode_segs = [s for s in play_all_segs if s not in bumper_segs]

    # Map segments to single-segment titles
    seg_to_title = {}
    for t in titles:
        if t["segment_count"] == 1:
            seg_to_title[t["segments"]] = t

    # Build result in play-all order, skipping segments without standalone titles
    result = [seg_to_title[seg] for seg in episode_segs if seg in seg_to_title]

    return result if len(result) >= 3 else None


def _find_play_all_order(titles, episode_segments):
    """Extract episode order from a 'play all' title if one exists.

    The play-all title is a multi-segment title whose segments are a
    superset of all episode segments (with bumpers/recaps interspersed).
    Returns episode segments in play-all order, or None if not found.
    """
    ep_seg_set = set(episode_segments)

    # Find multi-segment titles that contain all episode segments
    candidates = []
    for t in titles:
        if t["segment_count"] <= 1:
            continue
        title_segs = set(t["segments"].split(","))
        if ep_seg_set.issubset(title_segs):
            candidates.append(t)

    if not candidates:
        return None

    # Use the one with the most segments (most likely the full play-all)
    play_all = max(candidates, key=lambda t: t["segment_count"])

    # Extract episode segments in play-all order
    play_all_segs = play_all["segments"].split(",")
    return [s for s in play_all_segs if s in ep_seg_set]


def select_episode_titles(titles):
    """Select deduplicated episode titles from a TV disc.

    Filters to titles in the episode duration range, prefers single-segment
    titles (no intro bumper), and deduplicates by checking for shared segments.
    """
    # Try play-all-based discovery first — avoids duration filter excluding
    # two-part episodes that are double the typical episode length
    play_all_result = _discover_from_play_all(titles)
    if play_all_result:
        return play_all_result

    # Fall back to duration-based approach
    episode_range = [t for t in titles if 300 <= t["duration_secs"] <= 3900]
    if not episode_range:
        return []
    durations = [t["duration_secs"] for t in episode_range]
    median = statistics.median(durations)
    candidates = [t for t in episode_range
                  if abs(t["duration_secs"] - median) / median <= 0.30]
    if not candidates:
        return []

    # Separate single-segment (clean) and multi-segment (bumper) candidates
    single = [t for t in candidates if t["segment_count"] == 1]
    multi = [t for t in candidates if t["segment_count"] > 1]

    if single and multi:
        # Use multi-segment bumper titles to identify real episode segments.
        # The last segment in a bumper title is the episode content.
        # This filters out raw m2ts streams that match the duration range
        # but are duplicates with the intro baked in.
        episode_segs = set()
        for t in multi:
            parts = t["segments"].split(",")
            episode_segs.add(parts[-1])

        verified = [t for t in single if t["segments"] in episode_segs]
        candidates = verified if verified else single
    elif single:
        candidates = single

    # Deduplicate by segments: if one title's segments are a subset of
    # another's, keep the one with fewer segments
    seen_segments = set()
    result = []
    for t in sorted(candidates, key=lambda t: t["segment_count"]):
        seg_set = frozenset(t["segments"].split(","))
        if not any(seg_set & existing == seg_set for existing in seen_segments):
            result.append(t)
            seen_segments.add(seg_set)

    # Try to derive order from a "play all" title on the disc.
    # Play-all titles contain all episode segments interspersed with
    # bumpers/recaps, and their segment order reflects the intended
    # viewing order — which can differ from m2ts stream numbering.
    ep_segs = {t["segments"] for t in result}
    play_all_order = _find_play_all_order(titles, ep_segs)

    if play_all_order:
        seg_to_pos = {seg: i for i, seg in enumerate(play_all_order)}
        return sorted(result,
                      key=lambda t: seg_to_pos.get(t["segments"], float('inf')))

    # Fall back: sort by segment number (m2ts stream ID).
    # Title IDs reflect playlist discovery order, which can be scrambled.
    def _seg_key(t):
        seg = t["segments"].split(",")[-1]
        try:
            return int(seg)
        except (ValueError, IndexError):
            return t["id"]

    return sorted(result, key=_seg_key)
