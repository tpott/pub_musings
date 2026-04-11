"""Post-rip verification: episode count, duration anomaly, file integrity.

Optionally invokes Claude CLI for intelligent analysis when checkers flag issues.
"""

import json
import os
import shutil
import statistics
import subprocess
from pathlib import Path


class VerificationError(Exception):
    """Raised when verification finds errors that should halt the pipeline."""
    pass


VERIFY_SYSTEM_PROMPT = """\
You are a DVD ripping verification assistant. You analyze rip results to detect \
issues like missing episodes, incorrect title selection, or combined episodes.

Your response must be ONLY a valid JSON object (no markdown fences, no explanation \
before or after) with this exact schema:
{
  "verdict": "pass" or "fail" or "warn",
  "confidence": 0.0 to 1.0,
  "issues": [
    {"type": "count_mismatch or missing_episode or combined_episode or duration_anomaly",
     "detail": "human-readable description",
     "severity": "error or warning"}
  ],
  "recommendation": "human-readable recommendation for the user",
  "fix_command": "complete shell command to fix the issue, or null"
}

Guidelines:
- verdict="fail" if episodes are missing or incorrectly selected
- verdict="warn" if suspicious but not definitively wrong
- verdict="pass" if everything looks correct
- When recommending a fix_command, use this format:
  systemd-run --user --unit="dvd-rip-$(date +%s)" \\
    --setenv=TITLES="0,1,2,3,4,5,6" \\
    --setenv=SHOW_NAME="Show Name" \\
    --setenv=SEASON=N \\
    --setenv=DISC=N \\
    "$RIP_DIR/rip.py" --force
- Always include --force because the previous failed state file still exists
- List TITLES= IDs in intended episode order based on the makemkv info
- Analyze the raw makemkv segment data to determine correct title ordering
- If all episode titles share the same segments (dedup bug), recommend all title IDs
- Include notes about the code bug if you can identify one from the data\
"""


def check_episode_count(state):
    """Compare episode-length disc titles against selected episode count.

    Counts titles in the 15-65 minute range on the disc, excluding bumper
    duplicates (titles whose segments are a strict superset of another
    title's segments). Compares against the plan's episode count.
    """
    if state.get("media_type") == "movie":
        return []

    titles = state.get("titles", [])
    episodes = state.get("plan", {}).get("episodes", [])

    # Episode-length titles on disc (15-65 min)
    ep_titles = [t for t in titles if 900 <= t.get("duration_secs", 0) <= 3900]
    seg_sets = [frozenset(t.get("segments", "").split(",")) for t in ep_titles]

    # Exclude bumper duplicates: titles whose segments are a strict superset
    # of another episode-length title's segments
    disc_ep_count = 0
    for i in range(len(ep_titles)):
        is_superset = any(
            seg_sets[i] > seg_sets[j]
            for j in range(len(ep_titles)) if i != j
        )
        if not is_superset:
            disc_ep_count += 1

    selected_count = len(episodes)

    if disc_ep_count > selected_count:
        return [{
            "type": "count_mismatch",
            "severity": "error",
            "detail": (
                f"{disc_ep_count} episode-length titles on disc "
                f"but only {selected_count} selected"
            ),
        }]
    return []


def check_duration_anomaly(state):
    """Flag episodes with durations far from the median.

    - >40% deviation from median -> duration_anomaly
    - ~2x median -> combined_episode (potential two-in-one)
    """
    episodes = state.get("plan", {}).get("episodes", [])
    if len(episodes) < 2:
        return []

    durations = [ep.get("duration_secs", 0) for ep in episodes]
    median = statistics.median(durations)
    if median == 0:
        return []

    issues = []
    for ep in episodes:
        dur = ep.get("duration_secs", 0)
        deviation = abs(dur - median) / median

        if dur > 0 and 1.7 <= dur / median <= 2.3:
            issues.append({
                "type": "combined_episode",
                "severity": "warning",
                "detail": (
                    f"Title {ep.get('title_id')} duration {dur}s "
                    f"is ~2x median {median:.0f}s — possible combined episode"
                ),
            })
        elif deviation > 0.40:
            issues.append({
                "type": "duration_anomaly",
                "severity": "warning",
                "detail": (
                    f"Title {ep.get('title_id')} duration {dur}s "
                    f"deviates {deviation:.0%} from median {median:.0f}s"
                ),
            })

    return issues


def check_file_integrity(state):
    """Verify all planned episode files exist and are non-empty."""
    episodes = state.get("plan", {}).get("episodes", [])
    issues = []

    for ep in episodes:
        mp4_path = ep.get("mp4_path")
        if not mp4_path:
            continue
        path = Path(mp4_path)
        if not path.exists():
            issues.append({
                "type": "missing_episode",
                "severity": "error",
                "detail": f"Expected file missing: {mp4_path}",
            })
        elif path.stat().st_size == 0:
            issues.append({
                "type": "missing_episode",
                "severity": "error",
                "detail": f"Empty file: {mp4_path}",
            })

    return issues


def build_verify_prompt(state, checker_results, conf):
    """Build the dynamic prompt for Claude CLI verification."""
    parts = [
        "## Rip Results to Verify\n",
        f"Disc label: {state.get('disc_label', 'unknown')}",
        f"Media type: {state.get('media_type', 'unknown')}",
        f"RIP_DIR: {conf.get('RIP_DIR', 'unknown')}",
        "",
        "### Checker Results",
    ]
    if checker_results:
        for issue in checker_results:
            parts.append(
                f"- [{issue['severity']}] {issue['type']}: {issue['detail']}"
            )
    else:
        parts.append("- All checks passed")

    parts.extend([
        "",
        "### Plan",
        f"Episodes selected: {len(state.get('plan', {}).get('episodes', []))}",
    ])
    for ep in state.get("plan", {}).get("episodes", []):
        line = (
            f"  - Title {ep.get('title_id')}: {ep.get('ep_name', '?')} "
            f"({ep.get('duration_secs', 0)}s, segments={ep.get('segments', '?')})"
        )
        if ep.get("mp4_path"):
            line += f" -> {ep['mp4_path']}"
        parts.append(line)

    parts.extend([
        "",
        "### Raw makemkvcon info",
        state.get("scan_disc", {}).get("makemkv_info", "(not available)"),
    ])

    # Output directory listing
    output_dir = state.get("plan", {}).get("output_dir")
    if output_dir and Path(output_dir).exists():
        mp4s = sorted(Path(output_dir).glob("*.mp4"))
        parts.extend(["", f"### Output directory: {output_dir}"])
        for mp4 in mp4s:
            parts.append(f"  - {mp4.name} ({mp4.stat().st_size} bytes)")
        if not mp4s:
            parts.append("  (no mp4 files)")

    return "\n".join(parts)


def run_claude_verify(prompt, conf):
    """Invoke Claude CLI for verification analysis.

    Returns a dict with verdict, confidence, issues, recommendation, fix_command.
    """
    claude_bin = conf.get("CLAUDE_BIN", "claude")
    cmd = [
        claude_bin, "--print",
        "--dangerously-skip-permissions",
        "--system-prompt", VERIFY_SYSTEM_PROMPT,
    ]

    model = conf.get("VERIFY_MODEL")
    if model:
        cmd.extend(["--model", model])

    cwd = conf.get("RIP_DIR")
    if cwd and not Path(cwd).is_dir():
        cwd = None
    print(f"+ claude --print (verification)", flush=True)
    proc = subprocess.Popen(
        cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
        stderr=subprocess.PIPE, text=True, cwd=cwd,
    )
    stdout, stderr = proc.communicate(input=prompt)

    if proc.returncode != 0:
        return {
            "verdict": "warn", "confidence": 0.0,
            "issues": [],
            "recommendation": f"Claude CLI failed (rc={proc.returncode}): {stderr[:500]}",
            "fix_command": None,
        }

    # The system prompt requests JSON-only output. Parse it, stripping
    # any markdown code fences Claude might wrap around it.
    text = stdout.strip()
    if text.startswith("```"):
        lines = text.split("\n")
        end = -1 if lines[-1].strip().startswith("```") else len(lines)
        text = "\n".join(lines[1:end]).strip()

    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return {
            "verdict": "warn", "confidence": 0.0,
            "issues": [],
            "recommendation": f"Could not parse Claude response: {stdout[:500]}",
            "fix_command": None,
        }


def stage_verify(conf, state):
    """Pipeline stage: run all verification checks.

    Runs deterministic checkers first. If any checker flags an issue (or
    VERIFY_CLAUDE_ALWAYS=true), invokes Claude CLI for intelligent analysis.
    Raises VerificationError on failure, preventing sync to Jellyfin.
    """
    if conf.get("VERIFY_ENABLED", "true").lower() == "false":
        return state

    issues = []
    issues.extend(check_episode_count(state))
    issues.extend(check_duration_anomaly(state))
    issues.extend(check_file_integrity(state))

    state["verification"] = {"issues": issues}

    errors = [i for i in issues if i["severity"] == "error"]
    should_invoke_claude = (
        bool(errors)
        or conf.get("VERIFY_CLAUDE_ALWAYS", "false").lower() == "true"
    )

    claude_bin = conf.get("CLAUDE_BIN", "claude")
    claude_available = Path(claude_bin).exists() if os.path.isabs(claude_bin) else shutil.which(claude_bin)
    if should_invoke_claude and claude_available:
        prompt = build_verify_prompt(state, issues, conf)
        claude_result = run_claude_verify(prompt, conf)
        state["verification"]["claude_verdict"] = claude_result

    # Checker errors always halt the pipeline; Claude's verdict enriches
    # the error message with a recommendation and fix_command
    if errors:
        claude_verdict = state.get("verification", {}).get("claude_verdict")
        if claude_verdict and claude_verdict.get("fix_command"):
            recommendation = claude_verdict.get("recommendation", "")
            fix_command = claude_verdict["fix_command"]
            detail = recommendation + f"\nFix: {fix_command}"
        else:
            detail = "; ".join(e["detail"] for e in errors)
        raise VerificationError(detail)

    return state
