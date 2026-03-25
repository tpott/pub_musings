#!/usr/bin/env python3
"""Automated DVD/Blu-ray ripping pipeline.

Rips discs with MakeMKV, transcodes with HandBrake on a remote host,
syncs the result to a Jellyfin media server, and sends a notification.
"""

import os
import re
import shlex
import subprocess
import sys
from pathlib import Path

from disc import compute_episode_start, parse_disc_label
from titles import (
    detect_media_type,
    parse_makemkv_info,
    select_episode_titles,
    select_movie_title,
)
from transcode import find_mkv_for_title, transcode_and_sync


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
        mkv = find_mkv_for_title(output_dir, ep["id"], ep.get("filename"))
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
        f" --message {shlex.quote(message)}"
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

        transcode_and_sync(conf, jobs, run)

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
