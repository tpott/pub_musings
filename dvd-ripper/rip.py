#!/usr/bin/env python3
"""Automated DVD/Blu-ray ripping pipeline.

Rips discs with MakeMKV, transcodes with HandBrake on a remote host,
syncs the result to a Jellyfin media server, and sends a notification.
"""

import os
import re
import subprocess
import sys
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


def main():
    script_dir = Path(__file__).resolve().parent
    conf_path = script_dir / "rip.conf"
    if not conf_path.exists():
        print(f"Error: {conf_path} not found. Copy rip.conf.example to rip.conf and edit it.", file=sys.stderr)
        sys.exit(1)

    conf = parse_conf(conf_path)
    rip_dir = conf["RIP_DIR"]
    handbrake_host = conf["HANDBRAKE_HOST"]
    backup_host = conf["BACKUP_HOST"]
    handbrake_work_dir = conf["HANDBRAKE_WORK_DIR"]
    backup_dest = conf["BACKUP_DEST"]
    openclaw_bin = conf["OPENCLAW_BIN"]
    openclaw_target = conf["OPENCLAW_TARGET"]
    handbrake_encoder = conf["HANDBRAKE_ENCODER"]
    handbrake_preset = conf["HANDBRAKE_PRESET"]
    handbrake_quality = conf["HANDBRAKE_QUALITY"]
    handbrake_audio_bitrate = conf["HANDBRAKE_AUDIO_BITRATE"]

    # Read disc label
    try:
        disc_name = subprocess.run(
            ["blkid", "-o", "value", "-s", "LABEL", "/dev/sr0"],
            capture_output=True, text=True, check=True,
        ).stdout.strip()
    except subprocess.CalledProcessError:
        disc_name = "UnknownDisc"

    # MOVIE_NAME can be overridden via environment
    movie_name = os.environ.get("MOVIE_NAME", disc_name)
    safe_movie_name = re.sub(r'[^a-zA-Z0-9._-]', '_', movie_name)
    output_dir = Path(rip_dir) / "Movies" / movie_name
    remote_dir = f"{backup_dest}/{movie_name}"
    output_dir.mkdir(parents=True, exist_ok=True)

    # Rip the longest title (by duration) to avoid grabbing extras
    # Field 9 = duration string "H:MM:SS"
    # Use subprocess.run directly to capture stdout without echoing the command
    info_output = subprocess.run(
        ["makemkvcon", "-r", "info", "disc:0"],
        capture_output=True, text=True,
    ).stdout

    max_secs = 0
    largest_title = "0"
    for line in info_output.splitlines():
        m = re.match(r'^TINFO:(\d+),9,0,"(\d+:\d{2}:\d{2})"', line)
        if not m:
            continue
        title_id = m.group(1)
        parts = m.group(2).split(":")
        secs = int(parts[0]) * 3600 + int(parts[1]) * 60 + int(parts[2])
        if secs > max_secs:
            max_secs = secs
            largest_title = title_id

    run(f"makemkvcon mkv disc:0 {largest_title} {output_dir}")

    # Find the largest MKV file
    mkvs = sorted(output_dir.glob("*.mkv"), key=lambda p: p.stat().st_size, reverse=True)
    if not mkvs:
        print("Error: no MKV files found after ripping", file=sys.stderr)
        sys.exit(1)
    largest_mkv = mkvs[0]
    mkv_basename = largest_mkv.name
    safe_mkv_basename = re.sub(r'[^a-zA-Z0-9._-]', '_', mkv_basename)

    # Transcode on the HandBrake host via SSH
    run(f"ssh {handbrake_host} 'mkdir -p {handbrake_work_dir}'")
    run(f"scp -O {largest_mkv} {handbrake_host}:{handbrake_work_dir}/{safe_mkv_basename}")
    run(
        f"ssh {handbrake_host} '"
        f"HandBrakeCLI"
        f" -i {handbrake_work_dir}/{safe_mkv_basename}"
        f" -o {handbrake_work_dir}/{safe_movie_name}.mp4"
        f" -e {handbrake_encoder}"
        f" --encoder-preset {handbrake_preset}"
        f" -q {handbrake_quality}"
        f" -B {handbrake_audio_bitrate}'"
    )
    run(f"scp -O {handbrake_host}:{handbrake_work_dir}/{safe_movie_name}.mp4 {output_dir}/{movie_name}.mp4")
    run(f"ssh {handbrake_host} 'rm {handbrake_work_dir}/{safe_mkv_basename} {handbrake_work_dir}/{safe_movie_name}.mp4'")
    largest_mkv.unlink()

    # Sync to backup host
    run(f"rsync --mkpath -avz {output_dir}/{movie_name}.mp4 {backup_host}:{remote_dir}/{movie_name}.mp4")

    # Eject disc
    run("eject /dev/sr0")

    # Notify via OpenClaw
    run(
        f"{openclaw_bin} message send"
        f" --channel matrix --target {openclaw_target}"
        f" --message 'Rip complete: {movie_name} is ready in Jellyfin'"
    )


if __name__ == "__main__":
    main()
