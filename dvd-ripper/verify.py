"""Post-rip verification: episode count, duration anomaly, file integrity.

Optionally invokes Claude CLI for intelligent analysis when checkers flag issues.
"""

import re
import statistics
from pathlib import Path

import claude


# Disc label patterns that suggest TV content, not a movie
_TV_LABEL_RE = re.compile(
    r'[_\s](?:S\d|Season|Series|Book|Disc[_\s]?\d|Vol)', re.IGNORECASE)



class VerificationError(Exception):
    """Raised when verification finds errors that should halt the pipeline.

    Carries structured recommendation/fix_command attributes so the caller
    can render them as separate notifications.
    """

    def __init__(self, message, recommendation=None, fix_command=None):
        super().__init__(message)
        self.recommendation = recommendation or message
        self.fix_command = fix_command


VERIFY_SYSTEM_PROMPT = """\
You are a DVD ripping verification assistant. You analyze rip results to detect \
issues like missing episodes, incorrect title selection, or combined episodes.

For TV shows: you MUST use the /title-frame-scanner skill on EVERY episode mp4 \
file to visually verify episode identity. Pass each mp4_path from the plan. \
Report all results in the title_frame_check field, including the timestamp \
(in seconds from the start of the file) where the title card appeared. \
If the skill is unavailable or a file cannot be scanned, set attempted=true \
with an error note.

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
  "fix_command": "complete shell command to fix the issue, or null",
  "title_frame_check": {
    "attempted": true,
    "results": [
      {"file": "path",
       "title_text": "detected text or null",
       "timestamp_secs": "seconds into the file where the title card was seen, or null if not detected"}
    ],
    "error": "error message if scan failed, or null"
  }
}

Guidelines:
- verdict="fail" if episodes are missing or incorrectly selected
- verdict="warn" if suspicious but not definitively wrong
- verdict="pass" if everything looks correct
- When recommending a fix_command, use this format (substitute the \
absolute RIP_DIR path shown in the prompt — do NOT emit the literal \
string "$RIP_DIR", since the copy-pasted command runs outside any shell \
where that variable is defined):
  systemd-run --user --unit="dvd-rip-$(date +%s)" \\
    --setenv=TITLES="0,1,2,3,4,5,6" \\
    --setenv=SHOW_NAME="Show Name" \\
    --setenv=SEASON=N \\
    --setenv=DISC=N \\
    "/absolute/path/to/rip.py" --force
- Always include --force because the previous failed state file still exists
- List TITLES= IDs in intended episode order based on the makemkv info
- Analyze the raw makemkv segment data to determine correct title ordering
- If all episode titles share the same segments (dedup bug), recommend all title IDs
- Include notes about the code bug if you can identify one from the data
- Use the "All existing episodes for this show" section to determine the correct \
SEASON and DISC values. The next disc should continue from the last existing \
episode. If Season N is complete and the disc has new episodes, set SEASON=N+1 \
and DISC=1. Compare file sizes to detect potential duplicates across seasons.\
"""


def check_episode_count(state):
    """Compare episode-length disc titles against selected episode count.

    Counts titles in the 5-65 minute range on the disc, excluding bumper
    duplicates (titles whose segments are a strict superset of another
    title's segments). Compares against the plan's episode count.
    """
    if state.get("media_type") == "movie":
        return []

    titles = state.get("titles", [])
    episodes = state.get("plan", {}).get("episodes", [])

    # Episode-length titles on disc (5-65 min)
    ep_titles = [t for t in titles if 300 <= t.get("duration_secs", 0) <= 3900]
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


def check_is_movie(state):
    """Sanity-check movie classification with heuristics.

    For items classified as "movie", warn if the disc label contains TV
    indicators or the disc has 3+ similar-duration titles in the episode
    range — signs that it might actually be a TV disc.
    """
    if state.get("media_type") != "movie":
        return []

    issues = []
    disc_label = state.get("disc_label", "")

    # Check disc label for TV indicators
    if _TV_LABEL_RE.search(disc_label):
        issues.append({
            "type": "suspect_movie",
            "severity": "warning",
            "detail": (
                f"Classified as movie but disc label '{disc_label}' "
                f"contains TV indicators (season/series/book/disc/vol)"
            ),
        })

    # Check for 3+ similar-duration titles in episode range (5-65 min)
    titles = state.get("titles", [])
    ep_titles = [t for t in titles if 300 <= t.get("duration_secs", 0) <= 3900]
    if len(ep_titles) >= 3:
        durations = [t["duration_secs"] for t in ep_titles]
        median = statistics.median(durations)
        if median > 0:
            cluster = [d for d in durations
                       if abs(d - median) / median <= 0.30]
            if len(cluster) >= 3:
                issues.append({
                    "type": "suspect_movie",
                    "severity": "warning",
                    "detail": (
                        f"Classified as movie but disc has {len(cluster)} "
                        f"similar-duration titles in episode range "
                        f"(median {median:.0f}s)"
                    ),
                })

    return issues


def check_is_show(state):
    """Sanity-check TV classification with heuristics.

    For items classified as "tv", warn if only 1 episode was selected
    from a disc with only 1 title in the episode range — might be a movie.
    """
    if state.get("media_type") != "tv":
        return []

    episodes = state.get("plan", {}).get("episodes", [])
    if len(episodes) != 1:
        return []

    titles = state.get("titles", [])
    ep_titles = [t for t in titles if 300 <= t.get("duration_secs", 0) <= 3900]
    if len(ep_titles) == 1:
        return [{
            "type": "suspect_show",
            "severity": "warning",
            "detail": (
                "Classified as TV but only 1 episode selected from a disc "
                "with only 1 episode-length title — might be a movie"
            ),
        }]

    return []


def check_expected_duration(state):
    """Flag suspiciously short movies and missed longer titles.

    For movies: warn if the selected title is under 60 minutes, or if
    there are much longer titles on the disc that weren't selected.
    """
    if state.get("media_type") != "movie":
        return []

    episodes = state.get("plan", {}).get("episodes", [])
    if not episodes:
        return []

    issues = []
    selected = episodes[0]
    selected_dur = selected.get("duration_secs", 0)

    # Flag very short movies (under 60 min)
    if 0 < selected_dur < 3600:
        issues.append({
            "type": "short_movie",
            "severity": "warning",
            "detail": (
                f"Movie is only {selected_dur // 60}m{selected_dur % 60}s "
                f"— suspiciously short for a feature film"
            ),
        })

    # Flag if there are much longer titles on disc that weren't selected
    titles = state.get("titles", [])
    selected_id = selected.get("title_id")
    for t in titles:
        if t.get("id") == selected_id:
            continue
        t_dur = t.get("duration_secs", 0)
        if t_dur > selected_dur * 2 and t_dur > 3600:
            issues.append({
                "type": "longer_title_exists",
                "severity": "warning",
                "detail": (
                    f"Title {t.get('id')} is {t_dur // 60}m "
                    f"({t_dur}s) — much longer than selected "
                    f"{selected_dur // 60}m title"
                ),
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

    # Include all existing episodes across all seasons for this show
    plan = state.get("plan", {})
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

    return "\n".join(parts)


def run_claude_verify(prompt, conf):
    """Invoke Claude CLI for verification analysis.

    Returns a dict with verdict, confidence, issues, recommendation, fix_command.
    """
    fallback = {
        "verdict": "warn", "confidence": 0.0,
        "issues": [],
        "recommendation": "",
        "fix_command": None,
    }
    return claude.run(prompt, VERIFY_SYSTEM_PROMPT, conf, fallback=fallback)


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
    issues.extend(check_is_movie(state))
    issues.extend(check_is_show(state))
    issues.extend(check_expected_duration(state))

    state["verification"] = {"issues": issues}

    errors = [i for i in issues if i["severity"] == "error"]
    warnings = [i for i in issues if i["severity"] == "warning"]
    should_invoke_claude = (
        bool(errors) or bool(warnings)
        or conf.get("VERIFY_CLAUDE_ALWAYS", "true").lower() == "true"
    )

    if should_invoke_claude and claude.is_available(conf):
        prompt = build_verify_prompt(state, issues, conf)
        claude_result = run_claude_verify(prompt, conf)
        state["verification"]["claude_verdict"] = claude_result

    # Claude's fail verdict halts the pipeline even without checker errors
    claude_verdict = state.get("verification", {}).get("claude_verdict")
    if claude_verdict and claude_verdict.get("verdict") == "fail":
        recommendation = claude_verdict.get("recommendation", "")
        fix_command = claude_verdict.get("fix_command")
        detail = recommendation
        if fix_command:
            detail += f"\nFix: {fix_command}"
        raise VerificationError(
            detail, recommendation=recommendation, fix_command=fix_command,
        )

    # Checker errors always halt the pipeline; Claude's verdict enriches
    # the error message with a recommendation and fix_command
    if errors:
        claude_verdict = state.get("verification", {}).get("claude_verdict")
        if claude_verdict and claude_verdict.get("fix_command"):
            recommendation = claude_verdict.get("recommendation", "")
            fix_command = claude_verdict["fix_command"]
            detail = recommendation + f"\nFix: {fix_command}"
            raise VerificationError(
                detail, recommendation=recommendation, fix_command=fix_command,
            )
        detail = "; ".join(e["detail"] for e in errors)
        raise VerificationError(detail, recommendation=detail)

    return state
