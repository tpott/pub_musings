#!/usr/bin/env python3
"""Automated DVD/Blu-ray ripping pipeline.

Rips discs with MakeMKV, transcodes with HandBrake on a remote host,
syncs the result to a Jellyfin media server, and sends a notification.
"""

import argparse
import os
import re
import shlex
import subprocess
import sys
from pathlib import Path

from disc import compute_episode_start, normalize_disc_label, parse_disc_label
from error_analysis import analyze_error
from pipeline import run_pipeline
from state import (
    approve_state,
    archive_existing_state_file,
    archive_state,
    discover_active_state,
    load_state,
    new_state,
    resolve_state_arg,
    save_state,
    state_path_for_label,
)
from titles import (
    detect_media_type,
    parse_makemkv_info,
    select_episode_titles,
    select_movie_title,
)
from transcode import find_mkv_for_title, sync_only, transcode_only
from verify import VerificationError, stage_verify


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


def run_capture(cmd, **kwargs):
    """Run a command, printing it first, and return captured output."""
    print(f"+ {cmd}", flush=True)
    return subprocess.run(
        cmd, shell=True, check=True, capture_output=True, text=True, **kwargs
    )


# --- Stage functions ---

def _read_disc_label():
    """Read disc label from blkid and normalize its token casing."""
    try:
        label = subprocess.run(
            ["blkid", "-o", "value", "-s", "LABEL", "/dev/sr0"],
            capture_output=True, text=True, check=True,
        ).stdout.strip()
    except subprocess.CalledProcessError:
        label = "UnknownDisc"
    return normalize_disc_label(label)


def stage_scan_disc(conf, state):
    """Read disc label and scan titles with MakeMKV."""
    disc_label = _read_disc_label()

    info_output = subprocess.run(
        ["makemkvcon", "--robot", "info", "disc:0"],
        capture_output=True, text=True,
    ).stdout
    print(info_output, flush=True)

    titles = parse_makemkv_info(info_output)
    media_type = detect_media_type(titles)

    state["disc_label"] = disc_label
    state["scan_disc"] = {"makemkv_info": info_output}
    state["titles"] = titles
    state["media_type"] = media_type
    return state


def stage_plan(conf, state):
    """Detect media type and plan rip jobs (no ripping yet)."""
    titles = state["titles"]
    disc_label = state["disc_label"]
    media_type = state["media_type"]
    rip_dir = conf["RIP_DIR"]

    if os.environ.get("TITLES"):
        media_type = "tv"
        state["media_type"] = "tv"

    if media_type == "movie":
        movie_name = os.environ.get("MOVIE_NAME", disc_label)
        output_dir = Path(rip_dir) / "Movies" / movie_name
        output_dir.mkdir(parents=True, exist_ok=True)
        backup_dest = conf["BACKUP_DEST"]

        title = select_movie_title(titles)
        state["plan"] = {
            "output_dir": str(output_dir),
            "episodes": [{
                "title_id": title["id"],
                "duration_secs": title["duration_secs"],
                "segments": title.get("segments", ""),
                "ep_name": movie_name,
                "remote_dir": f"{backup_dest}/{movie_name}",
            }],
            "notify_msg": f"Rip complete: {movie_name} is ready in Jellyfin",
        }
    else:
        titles_override = os.environ.get("TITLES")
        if titles_override:
            title_ids = [int(x) for x in titles_override.split(",")]
            id_to_title = {t["id"]: t for t in titles}
            episodes_raw = []
            for tid in title_ids:
                if tid not in id_to_title:
                    raise RuntimeError(f"Title ID {tid} not found on disc")
                episodes_raw.append(id_to_title[tid])
        else:
            episodes_raw = select_episode_titles(titles)

        meta = parse_disc_label(disc_label, media_type="tv")
        show_name = os.environ.get("SHOW_NAME", meta["show_name"])
        season = int(os.environ.get("SEASON", meta.get("season", 1)))
        disc = int(os.environ.get("DISC", meta.get("disc", 1)))

        season_dir = f"Season {season:02d}"
        output_dir = Path(rip_dir) / "TV" / show_name / season_dir
        output_dir.mkdir(parents=True, exist_ok=True)
        backup_dest = conf.get("BACKUP_DEST_TV", conf["BACKUP_DEST"])

        ep_start = compute_episode_start(output_dir, disc, len(episodes_raw))

        plan_episodes = []
        for i, ep in enumerate(episodes_raw):
            ep_num = ep_start + i
            ep_name = f"{show_name} S{season:02d}E{ep_num:02d}"
            plan_episodes.append({
                "title_id": ep["id"],
                "duration_secs": ep["duration_secs"],
                "segments": ep.get("segments", ""),
                "filename": ep.get("filename", ""),
                "ep_name": ep_name,
                "remote_dir": f"{backup_dest}/{show_name}/{season_dir}",
            })

        # Check for mp4 files that would be overwritten
        existing_mp4s = []
        for ep in plan_episodes:
            mp4 = output_dir / f"{ep['ep_name']}.mp4"
            if mp4.exists():
                existing_mp4s.append(mp4.name)
        if existing_mp4s:
            names = ", ".join(existing_mp4s)
            if conf.get("_force"):
                print(f"WARNING: --force overwriting {len(existing_mp4s)} "
                      f"existing files: {names}", file=sys.stderr)
            else:
                raise RuntimeError(
                    f"Would overwrite {len(existing_mp4s)} existing files: "
                    f"{names}. Use --force to override, or set SEASON=/DISC= "
                    f"to target the correct season and disc."
                )

        ep_end = ep_start + len(plan_episodes) - 1
        state["plan"] = {
            "show_name": show_name,
            "season": season,
            "disc": disc,
            "output_dir": str(output_dir),
            "episodes": plan_episodes,
            "notify_msg": (
                f"Rip complete: {show_name} S{season:02d}"
                f"E{ep_start:02d}-E{ep_end:02d} ready in Jellyfin"
            ),
        }
    return state


def stage_rip(conf, state):
    """Rip titles from disc using MakeMKV."""
    output_dir = state["plan"]["output_dir"]
    for ep in state["plan"]["episodes"]:
        run(f"makemkvcon mkv disc:0 {ep['title_id']} '{output_dir}'")
    return state


def stage_transcode(conf, state):
    """Transcode ripped MKV files via remote HandBrake host."""
    output_dir = Path(state["plan"]["output_dir"])
    jobs = []
    for ep in state["plan"]["episodes"]:
        mkv = find_mkv_for_title(
            output_dir, ep["title_id"], ep.get("filename")
        )
        if mkv is None:
            print(f"Warning: no MKV found for title {ep['title_id']}, skipping",
                  file=sys.stderr)
            continue
        jobs.append({
            "mkv_path": mkv,
            "output_name": ep["ep_name"],
            "output_dir": output_dir,
            "remote_dir": ep["remote_dir"],
        })

    transcode_only(conf, jobs, run)

    # Record mp4 paths in state for verify
    for ep in state["plan"]["episodes"]:
        ep["mp4_path"] = str(output_dir / f"{ep['ep_name']}.mp4")

    # Store jobs for sync stage
    state["_jobs"] = [{
        "output_name": j["output_name"],
        "output_dir": str(j["output_dir"]),
        "remote_dir": j["remote_dir"],
    } for j in jobs]
    return state


# stage_verify is imported from verify.py


def stage_sync(conf, state):
    """Sync transcoded files to backup host."""
    jobs = []
    for j in state.get("_jobs", []):
        jobs.append({
            "output_name": j["output_name"],
            "output_dir": Path(j["output_dir"]),
            "remote_dir": j["remote_dir"],
        })
    sync_only(conf, jobs, run)
    return state


def stage_finalize(conf, state):
    """Eject disc and send success notification."""
    if conf.get("AUTO_EJECT", "false").lower() == "true":
        run("eject /dev/sr0")

    notify_msg = state["plan"].get("notify_msg", "Rip complete")

    # Append verification notes if any issues were auto-fixed or warned
    issues = state.get("verification", {}).get("issues", [])
    warnings = [i for i in issues if i["severity"] == "warning"]
    if warnings:
        notes = "; ".join(w["detail"] for w in warnings)
        notify_msg += f" ({notes})"

    if state.get("stages", {}).get("verify", {}).get("approved"):
        notify_msg += " (verification manually approved)"

    notify(conf, notify_msg)
    return state


def notify(conf, message):
    """Send a notification via OpenClaw."""
    openclaw_bin = conf["OPENCLAW_BIN"]
    openclaw_target = conf["OPENCLAW_TARGET"]
    run(
        f"{openclaw_bin} message send"
        f" --channel matrix --target {openclaw_target}"
        f" --message {shlex.quote(message)}"
    )


def _default_fix_command(state, exception, rip_script_path):
    """Return a fallback systemd-run fix command when Claude returns fix_command=None.

    Decision tree:
      state file on disk + VerificationError  → --approve <label> (artifacts present)
      state file on disk + other exception    → --resume <label>  (retry tail of pipeline)
      no state file + disc_label known        → --force (fresh re-run)
      no state file + disc_label unknown      → None (manual diagnosis)
    """
    state_path = state.get("_state_path", "")
    disc_label = state.get("disc_label", "")
    quoted_script = shlex.quote(str(rip_script_path))

    if state_path and Path(state_path).exists():
        quoted_label = shlex.quote(disc_label) if disc_label else "unknown"
        if isinstance(exception, VerificationError):
            return (
                f'systemd-run --user --unit="dvd-rip-$(date +%s)" '
                f'{quoted_script} --approve {quoted_label}'
            )
        return (
            f'systemd-run --user --unit="dvd-rip-$(date +%s)" '
            f'{quoted_script} --resume {quoted_label}'
        )
    if disc_label:
        return (
            f'systemd-run --user --unit="dvd-rip-$(date +%s)" '
            f'{quoted_script} --force'
        )
    return None


STOP_FILE = "STOP"

STAGES = [
    ("scan_disc",  [],              stage_scan_disc),
    ("plan",       ["scan_disc"],   stage_plan),
    ("rip",        ["plan"],        stage_rip),
    ("transcode",  ["rip"],         stage_transcode),
    ("verify",     ["transcode"],   stage_verify),
    ("sync",       ["verify"],      stage_sync),
    ("finalize",   ["sync"],        stage_finalize),
]


def main():
    parser = argparse.ArgumentParser(
        description="Automated DVD/Blu-ray ripping pipeline"
    )
    exclusive = parser.add_mutually_exclusive_group()
    exclusive.add_argument(
        "--resume", nargs="?", const=True, default=None,
        metavar="LABEL_OR_PATH",
        help="Resume from state file (no arg: auto-detect from disc)",
    )
    exclusive.add_argument(
        "--approve", nargs="?", const=True, default=None,
        metavar="LABEL_OR_PATH",
        help="Approve a failed verification and run remaining stages",
    )
    parser.add_argument(
        "--force", action="store_true",
        help="Override collision guard on existing state files",
    )
    args = parser.parse_args()

    script_dir = Path(__file__).resolve().parent

    stop_path = script_dir / STOP_FILE
    if stop_path.exists():
        print(f"Stop file exists ({stop_path}), skipping rip.", flush=True)
        sys.exit(0)

    conf_path = script_dir / "rip.conf"
    if not conf_path.exists():
        print(f"Error: {conf_path} not found. Copy rip.conf.example "
              f"to rip.conf and edit it.", file=sys.stderr)
        sys.exit(1)

    conf = parse_conf(conf_path)
    conf["_force"] = args.force

    try:
        if args.approve:
            state = resolve_state_arg(conf["RIP_DIR"], args.approve)
            approve_state(state)
            save_state(state)
        elif args.resume:
            if args.resume is True:
                state, reason, label = discover_active_state(conf["RIP_DIR"])
                if state is None:
                    if reason == "no_disc":
                        raise RuntimeError(
                            "No disc in drive (/dev/sr0). "
                            "Insert a disc and retry."
                        )
                    raise RuntimeError(
                        f"No state file matches inserted disc {label}."
                    )
            elif Path(args.resume).exists():
                state = load_state(args.resume)
            else:
                state = resolve_state_arg(conf["RIP_DIR"], args.resume)
        else:
            # Fresh run — read disc label for state file
            disc_label = _read_disc_label()

            state_path = state_path_for_label(conf["RIP_DIR"], disc_label)
            if state_path.exists() and not args.force:
                raise RuntimeError(
                    f"State file already exists: {state_path}. "
                    f"Use --force to override."
                )

            archived = archive_existing_state_file(conf["RIP_DIR"], disc_label)
            if archived is not None:
                print(
                    f"Archived prior state file to {archived}",
                    flush=True,
                )

            state = new_state(conf["RIP_DIR"], disc_label)

        state = run_pipeline(STAGES, conf, state, save_state)
        archive_state(state)

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        try:
            st = locals().get("state", {}) or {}
            rip_script = Path(__file__).resolve()

            # VerificationError carries structured attrs; other exceptions
            # get routed through Claude for diagnosis.
            recommendation = getattr(e, "recommendation", None)
            fix_command = getattr(e, "fix_command", None)
            if recommendation is None:
                verdict = analyze_error(conf, st, e)
                recommendation = verdict.get("recommendation") or str(e)
                fix_command = verdict.get("fix_command")

            if fix_command is None:
                fix_command = _default_fix_command(st, e, rip_script)

            label = st.get("disc_label", "unknown")
            summary = f"Rip FAILED: {label}\n{recommendation}"
            state_file = st.get("_state_path", "")
            if state_file:
                summary += f"\nState: {state_file}"

            notify(conf, summary)
            if fix_command:
                notify(conf, fix_command)
        except Exception:
            pass
        sys.exit(1)


if __name__ == "__main__":
    main()
