"""Claude-powered analysis of pipeline errors that escape without a verify verdict.

Used for failures raised outside stage_verify (e.g. plan-stage overwrite guards,
subprocess failures during rip/transcode/sync). Produces a recommendation and
an optional fix_command the user can copy-paste.
"""

from pathlib import Path

import claude


ERROR_SYSTEM_PROMPT = """\
You are a DVD rip error analyst. Given an exception from a ripping pipeline, \
disc metadata, and the current output directory state, recommend a fix.

Your response must be ONLY a valid JSON object (no markdown fences, no \
explanation before or after) with this exact schema:
{
  "recommendation": "human-readable description of what went wrong and how to fix it",
  "fix_command": "complete shell command to fix the issue, or null if manual intervention is needed"
}

Guidelines:
- When recommending a re-run, use this format (substitute the absolute \
RIP_DIR path shown in the prompt — do NOT emit the literal string \
"$RIP_DIR", since the copy-pasted command runs outside any shell where \
that variable is defined):
  systemd-run --user --unit="dvd-rip-$(date +%s)" --setenv=SHOW_NAME="Show Name" --setenv=SEASON=N --setenv=DISC=N "/absolute/path/to/rip.py" --force
- Include --force if a state file likely exists from the failed attempt.
- DISC is the physical disc number in the set (1, 2, 3, ...), not an \
episode index. For a standard 4-disc season with 13 episodes, valid values \
are DISC=1..4. Do not set DISC to the episode count or starting episode \
number — the pipeline computes episode numbering automatically from the \
existing files in the output directory.
- For "Would overwrite" errors: the disc was planned with wrong SEASON/DISC. \
Examine the existing-episodes listing to determine correct values. If \
Season N is complete, the disc is likely Season N+1 Disc 1. If Season N is \
partially ripped (E01..EK exist), this is likely the next physical disc of \
Season N — set SEASON=N and DISC=(number of discs already ripped for that \
season)+1. Typical TV seasons are split across 3-5 discs.
- For subprocess failures during rip/transcode/sync stages, recommend \
`--resume <label>` using the absolute rip.py path, e.g.:
  systemd-run --user --unit="dvd-rip-$(date +%s)" "/absolute/path/to/rip.py" --resume LABEL
- Use `--resume LABEL` when the state file exists and the failure was \
mid-pipeline (rip/transcode/sync). This continues the pipeline without \
restarting from scratch.
- Use `--approve LABEL` when artifacts are on disk and only verify failed. \
This bypasses verify and finishes sync, e.g.:
  systemd-run --user --unit="dvd-rip-$(date +%s)" "/absolute/path/to/rip.py" --approve LABEL\
"""


def build_error_prompt(state, exc, conf):
    """Build the prompt sent to Claude for error analysis."""
    parts = [
        "## Error to Analyze\n",
        f"Disc label: {state.get('disc_label', 'unknown')}",
        f"Media type: {state.get('media_type', 'unknown')}",
        f"RIP_DIR: {conf.get('RIP_DIR', 'unknown')}",
        "",
        "### Exception",
        f"{type(exc).__name__}: {exc}",
    ]

    plan = state.get("plan", {})
    if plan:
        parts.extend([
            "",
            "### Plan (may be partial)",
            f"  show_name:  {plan.get('show_name', '?')}",
            f"  season:     {plan.get('season', '?')}",
            f"  disc:       {plan.get('disc', '?')}",
            f"  output_dir: {plan.get('output_dir', '?')}",
            f"  episodes planned: {len(plan.get('episodes', []))}",
        ])
        for ep in plan.get("episodes", []):
            parts.append(
                f"    - Title {ep.get('title_id')}: {ep.get('ep_name', '?')} "
                f"({ep.get('duration_secs', 0)}s, "
                f"segments={ep.get('segments', '?')})"
            )

    show_name = plan.get("show_name")
    if show_name:
        show_dir = Path(conf.get("RIP_DIR", "")) / "TV" / show_name
        if show_dir.is_dir():
            parts.extend(["", "### All existing episodes for this show"])
            for season_path in sorted(show_dir.iterdir()):
                if not season_path.is_dir():
                    continue
                mp4s = sorted(season_path.glob("*.mp4"))
                if mp4s:
                    parts.append(f"  {season_path.name}/")
                    for mp4 in mp4s:
                        parts.append(
                            f"    - {mp4.name} ({mp4.stat().st_size} bytes)"
                        )

    makemkv_info = state.get("scan_disc", {}).get("makemkv_info")
    if makemkv_info:
        parts.extend(["", "### Raw makemkvcon info", makemkv_info])

    return "\n".join(parts)


def analyze_error(conf, state, exc):
    """Ask Claude to diagnose a pipeline exception.

    Returns {"recommendation": str, "fix_command": str|None}. Falls back to
    the raw exception text when Claude is disabled or unavailable.
    """
    fallback = {"recommendation": str(exc), "fix_command": None}

    if conf.get("ERROR_CLAUDE_ENABLED", "true").lower() == "false":
        return fallback
    if not claude.is_available(conf):
        return fallback

    prompt = build_error_prompt(state, exc, conf)
    return claude.run(prompt, ERROR_SYSTEM_PROMPT, conf, fallback=fallback)
