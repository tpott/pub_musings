"""Post-rip verification: episode count, duration anomaly, file integrity."""

import statistics
from pathlib import Path


class VerificationError(Exception):
    """Raised when verification finds errors that should halt the pipeline."""
    pass


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

    - >40% deviation from median → duration_anomaly
    - ~2x median → combined_episode (potential two-in-one)
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


def stage_verify(conf, state):
    """Pipeline stage: run all verification checks.

    Raises VerificationError if any check returns an error-severity issue,
    preventing the pipeline from proceeding to sync.
    """
    if conf.get("VERIFY_ENABLED", "true").lower() == "false":
        return state

    issues = []
    issues.extend(check_episode_count(state))
    issues.extend(check_duration_anomaly(state))
    issues.extend(check_file_integrity(state))

    state["verification"] = {"issues": issues}

    errors = [i for i in issues if i["severity"] == "error"]
    if errors:
        summary = "; ".join(e["detail"] for e in errors)
        raise VerificationError(f"Verification failed: {summary}")

    return state
