#!/usr/bin/env python3
"""Automated DVD/Blu-ray ripping pipeline.

Rips discs with MakeMKV, transcodes with HandBrake on a remote host,
syncs the result to a Jellyfin media server, and sends a notification.
"""

import os
import re
import statistics
import subprocess
import sys
from collections import defaultdict
from pathlib import Path


def parse_conf(conf_path):
    """Parse a shell-style KEY="value" config file into a dict.

    Supports:
    - KEY="value" and KEY='value' (quoted)
    - KEY=value (unquoted)
    - $HOME and $VAR expansion from already-parsed keys + os.environ
    - Comments (#) and blank lines are skipped
    """
    config = {}
    with open(conf_path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            match = re.match(r'^([A-Za-z_][A-Za-z0-9_]*)=(.*)$', line)
            if not match:
                continue
            key = match.group(1)
            val = match.group(2).strip()
            # Strip matching quotes
            if (val.startswith('"') and val.endswith('"')) or \
               (val.startswith("'") and val.endswith("'")):
                val = val[1:-1]
            # Expand $VAR references from config and environment
            def expand(m):
                var = m.group(1) or m.group(2)
                return config.get(var, os.environ.get(var, m.group(0)))
            val = re.sub(r'\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)', expand, val)
            config[key] = val
    return config


def run(cmd, **kwargs):
    """Run a command, printing it first (like set -x)."""
    print(f"+ {cmd}", flush=True)
    return subprocess.run(cmd, shell=True, check=True, **kwargs)


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

    Returns "tv" if 3+ titles cluster in the episode range (15-65 min)
    within 30% of the median duration. Otherwise returns "movie".
    """
    episode_range = [t for t in titles if 900 <= t["duration_secs"] <= 3900]
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
    # Find titles in episode range
    episode_range = [t for t in titles if 900 <= t["duration_secs"] <= 3900]
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


def safe_name(name):
    """Replace non-filename-safe characters with underscores."""
    return re.sub(r'[^a-zA-Z0-9._-]', '_', name)


def find_mkv_for_title(output_dir, title_id):
    """Find the MKV file produced by makemkvcon for a given title ID.

    makemkvcon names output files like *_tNN.mkv where NN is the title number.
    """
    pattern = f"*_t{title_id:02d}.mkv"
    matches = list(Path(output_dir).glob(pattern))
    if matches:
        return matches[0]
    # Fallback: try without zero-padding
    pattern = f"*_t{title_id}.mkv"
    matches = list(Path(output_dir).glob(pattern))
    if matches:
        return matches[0]
    return None


def transcode_and_sync(conf, jobs):
    """Transcode MKV files on the HandBrake host and sync to backup.

    Each job is a dict with: mkv_path, output_name, output_dir, remote_dir
    """
    handbrake_host = conf["HANDBRAKE_HOST"]
    backup_host = conf["BACKUP_HOST"]
    handbrake_work_dir = conf["HANDBRAKE_WORK_DIR"]
    handbrake_encoder = conf["HANDBRAKE_ENCODER"]
    handbrake_preset = conf["HANDBRAKE_PRESET"]
    handbrake_quality = conf["HANDBRAKE_QUALITY"]
    handbrake_audio_bitrate = conf["HANDBRAKE_AUDIO_BITRATE"]

    run(f"ssh {handbrake_host} 'mkdir -p {handbrake_work_dir}'")

    for job in jobs:
        mkv_path = job["mkv_path"]
        output_name = job["output_name"]
        output_dir = job["output_dir"]
        remote_dir = job["remote_dir"]
        safe_mkv = safe_name(mkv_path.name)
        safe_out = safe_name(output_name)

        # Upload MKV to HandBrake host
        run(f"scp -O '{mkv_path}' {handbrake_host}:{handbrake_work_dir}/{safe_mkv}")

        # Transcode
        run(
            f"ssh {handbrake_host} '"
            f"HandBrakeCLI"
            f" -i {handbrake_work_dir}/{safe_mkv}"
            f" -o {handbrake_work_dir}/{safe_out}.mp4"
            f" -e {handbrake_encoder}"
            f" --encoder-preset {handbrake_preset}"
            f" -q {handbrake_quality}"
            f" -B {handbrake_audio_bitrate}'"
        )

        # Download transcoded file
        run(f"scp -O {handbrake_host}:{handbrake_work_dir}/{safe_out}.mp4"
            f" '{output_dir}/{output_name}.mp4'")

        # Clean up remote
        run(f"ssh {handbrake_host} 'rm"
            f" {handbrake_work_dir}/{safe_mkv}"
            f" {handbrake_work_dir}/{safe_out}.mp4'")

        # Clean up local MKV
        mkv_path.unlink()

        # Sync to backup host
        run(f"rsync --mkpath -avz"
            f" '{output_dir}/{output_name}.mp4'"
            f" {backup_host}:'{remote_dir}/{output_name}.mp4'")


def prepare_movie(conf, titles, disc_label):
    """Prepare rip jobs for a movie disc.

    Returns (jobs, notify_msg).
    """
    rip_dir = conf["RIP_DIR"]
    movie_name = os.environ.get("MOVIE_NAME", disc_label)
    output_dir = Path(rip_dir) / "Movies" / movie_name
    output_dir.mkdir(parents=True, exist_ok=True)
    backup_dest = conf["BACKUP_DEST"]

    title = select_movie_title(titles)
    run(f"makemkvcon mkv disc:0 {title['id']} '{output_dir}'")

    mkvs = sorted(output_dir.glob("*.mkv"),
                   key=lambda p: p.stat().st_size, reverse=True)
    if not mkvs:
        raise RuntimeError("No MKV files found after ripping")

    jobs = [{
        "mkv_path": mkvs[0],
        "output_name": movie_name,
        "output_dir": output_dir,
        "remote_dir": f"{backup_dest}/{movie_name}",
    }]
    notify_msg = f"Rip complete: {movie_name} is ready in Jellyfin"
    return jobs, notify_msg


def prepare_tv(conf, titles, disc_label):
    """Prepare rip jobs for a TV disc.

    Computes episode start from disc number and existing files.
    Requires discs to be ripped in sequential order.

    Returns (jobs, notify_msg).
    """
    rip_dir = conf["RIP_DIR"]
    titles_override = os.environ.get("TITLES")
    if titles_override:
        title_ids = [int(x) for x in titles_override.split(",")]
        id_to_title = {t["id"]: t for t in titles}
        episodes = []
        for tid in title_ids:
            if tid not in id_to_title:
                raise RuntimeError(f"Title ID {tid} not found on disc")
            episodes.append(id_to_title[tid])
    else:
        episodes = select_episode_titles(titles)
    meta = parse_disc_label(disc_label, media_type="tv")
    show_name = os.environ.get("SHOW_NAME", meta["show_name"])
    season = int(os.environ.get("SEASON", meta.get("season", 1)))
    disc = int(os.environ.get("DISC", meta.get("disc", 1)))

    season_dir = f"Season {season:02d}"
    output_dir = Path(rip_dir) / "TV" / show_name / season_dir
    output_dir.mkdir(parents=True, exist_ok=True)
    backup_dest = conf.get("BACKUP_DEST_TV", conf["BACKUP_DEST"])

    ep_start = compute_episode_start(output_dir, disc, len(episodes))

    # Rip all episode titles
    for ep in episodes:
        run(f"makemkvcon mkv disc:0 {ep['id']} '{output_dir}'")

    # Build transcode jobs
    jobs = []
    for i, ep in enumerate(episodes):
        ep_num = ep_start + i
        ep_name = f"{show_name} S{season:02d}E{ep_num:02d}"
        mkv = find_mkv_for_title(output_dir, ep["id"])
        if mkv is None:
            print(f"Warning: no MKV found for title {ep['id']}, skipping",
                  file=sys.stderr)
            continue
        jobs.append({
            "mkv_path": mkv,
            "output_name": ep_name,
            "output_dir": output_dir,
            "remote_dir": f"{backup_dest}/{show_name}/{season_dir}",
        })

    ep_count = len(jobs)
    ep_end = ep_start + ep_count - 1
    notify_msg = (f"Rip complete: {show_name} S{season:02d}"
                  f"E{ep_start:02d}-E{ep_end:02d} ready in Jellyfin")
    return jobs, notify_msg


def notify(conf, message):
    """Send a notification via OpenClaw."""
    openclaw_bin = conf["OPENCLAW_BIN"]
    openclaw_target = conf["OPENCLAW_TARGET"]
    run(
        f"{openclaw_bin} message send"
        f" --channel matrix --target {openclaw_target}"
        f" --message '{message}'"
    )


def main():
    script_dir = Path(__file__).resolve().parent
    conf_path = script_dir / "rip.conf"
    if not conf_path.exists():
        print(f"Error: {conf_path} not found. Copy rip.conf.example to rip.conf and edit it.", file=sys.stderr)
        sys.exit(1)

    conf = parse_conf(conf_path)

    try:
        # Read disc label
        try:
            disc_label = subprocess.run(
                ["blkid", "-o", "value", "-s", "LABEL", "/dev/sr0"],
                capture_output=True, text=True, check=True,
            ).stdout.strip()
        except subprocess.CalledProcessError:
            disc_label = "UnknownDisc"

        # Scan disc titles
        info_output = subprocess.run(
            ["makemkvcon", "--robot", "info", "disc:0"],
            capture_output=True, text=True,
        ).stdout
        print(info_output, flush=True)
        titles = parse_makemkv_info(info_output)

        media_type = detect_media_type(titles)

        if media_type == "movie":
            jobs, notify_msg = prepare_movie(conf, titles, disc_label)
        else:
            jobs, notify_msg = prepare_tv(conf, titles, disc_label)

        transcode_and_sync(conf, jobs)

        # Eject disc
        run("eject /dev/sr0")

        notify(conf, notify_msg)

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        try:
            notify(conf, f"Rip FAILED: {e}")
        except Exception:
            pass
        sys.exit(1)


if __name__ == "__main__":
    main()
