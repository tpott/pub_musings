"""Pipeline state management: save/load/archive for resume support."""

import json
import subprocess
from datetime import datetime
from pathlib import Path


def state_path_for_label(rip_dir, label):
    """Return the state file path for a given disc label."""
    return Path(rip_dir) / ".state" / f"{label}.json"


def new_state(rip_dir, disc_label):
    """Create a new state dict for a fresh pipeline run."""
    path = state_path_for_label(rip_dir, disc_label)
    now = datetime.now().isoformat()
    return {
        "version": 1,
        "status": "running",
        "created_at": now,
        "updated_at": now,
        "disc_label": disc_label,
        "_state_path": str(path),
        "stages": {},
        "plan": {},
        "verification": {"issues": []},
    }


def save_state(state):
    """Atomically write state to disk via tmp+rename."""
    path = Path(state["_state_path"])
    path.parent.mkdir(parents=True, exist_ok=True)
    state["updated_at"] = datetime.now().isoformat()
    tmp = path.with_suffix(".tmp")
    with open(tmp, "w") as f:
        json.dump(state, f, indent=2)
    tmp.rename(path)


def load_state(path):
    """Load state from a JSON file."""
    path = str(path)
    with open(path) as f:
        state = json.load(f)
    state["_state_path"] = path
    return state


def archive_state(state):
    """Move completed state file to history directory."""
    src = Path(state["_state_path"])
    ts = datetime.now().strftime("%Y%m%dT%H%M%S")
    dest = src.parent / "history" / f"{state['disc_label']}-{ts}.json"
    dest.parent.mkdir(parents=True, exist_ok=True)
    src.rename(dest)
    state["_state_path"] = str(dest)
    return state


def archive_existing_state_file(rip_dir, label):
    """If a state file already exists for this label, move it to history.

    Used before starting a fresh rip so previous state is preserved instead
    of being clobbered. Checks for the exact label first, then falls back to a
    case-insensitive scan to catch old files written before label normalization.
    Returns the archived path, or None if no file existed.
    """
    src = state_path_for_label(rip_dir, label)
    archive_label = label
    if not src.exists():
        state_dir = Path(rip_dir) / ".state"
        if state_dir.is_dir():
            label_lower = label.lower()
            for p in state_dir.glob("*.json"):
                if p.stem.lower() == label_lower:
                    src = p
                    archive_label = p.stem
                    break
    if not src.exists():
        return None
    ts = datetime.now().strftime("%Y%m%dT%H%M%S")
    dest = src.parent / "history" / f"{archive_label}-{ts}.json"
    dest.parent.mkdir(parents=True, exist_ok=True)
    src.rename(dest)
    return dest


def approve_state(state):
    """Mark verify stage as manually approved and reset pipeline to running."""
    now = datetime.now().isoformat()
    state["stages"]["verify"] = {
        "status": "complete",
        "completed_at": now,
        "approved": True,
        "approved_at": now,
    }
    state["status"] = "running"


def resolve_state_arg(rip_dir, arg):
    """Resolve a --resume/--approve argument to a loaded state dict.

    arg=True  → auto-detect from inserted disc via blkid
    arg=str (existing path) → load that file directly
    arg=str (label) → load .state/<label>.json

    Raises RuntimeError with a user-facing message on failure.
    """
    if arg is True:
        state, reason, label = discover_active_state(rip_dir)
        if state is None:
            if reason == "no_disc":
                raise RuntimeError(
                    "No disc in drive (/dev/sr0). Insert a disc and retry."
                )
            raise RuntimeError(
                f"No state file matches inserted disc {label}."
            )
        return state
    path_arg = Path(arg)
    if path_arg.exists():
        return load_state(str(path_arg))
    path = state_path_for_label(rip_dir, arg)
    if path.exists():
        return load_state(path)
    # Case-insensitive fallback for labels written before normalization
    state_dir = Path(rip_dir) / ".state"
    if state_dir.is_dir():
        arg_lower = arg.lower()
        lower_to_paths: dict[str, list] = {}
        for p in state_dir.glob("*.json"):
            lower_to_paths.setdefault(p.stem.lower(), []).append(p)
        if arg_lower in lower_to_paths:
            matched = lower_to_paths[arg_lower]
            if len(matched) == 1:
                return load_state(matched[0])
            candidates = ", ".join(str(p) for p in sorted(matched))
            raise RuntimeError(
                f"Ambiguous label {arg!r}: multiple case variants found: {candidates}"
            )
    raise RuntimeError(f"No state file for {arg!r}: {path}")


def discover_active_state(rip_dir):
    """Find active state file for the currently inserted disc via blkid.

    Returns a 3-tuple (state, reason, label):
      - ("ok", loaded_state_dict, label) when the disc is present and a
        matching state file was found.
      - (None, "no_state", label) when the disc is present and readable
        but no matching state file exists.
      - (None, "no_disc", None) when blkid reports no media in the drive
        (exit code 2) or blkid is unavailable.

    Detection relies on blkid's exit code rather than stderr parsing.
    """
    try:
        label = subprocess.run(
            ["blkid", "-o", "value", "-s", "LABEL", "/dev/sr0"],
            capture_output=True, text=True, check=True,
        ).stdout.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return (None, "no_disc", None)
    path = state_path_for_label(rip_dir, label)
    if path.exists():
        return (load_state(path), "ok", label)
    # Case-insensitive fallback for old non-normalized state files
    state_dir = Path(rip_dir) / ".state"
    if state_dir.is_dir():
        label_lower = label.lower()
        lower_to_paths: dict[str, list] = {}
        for p in state_dir.glob("*.json"):
            lower_to_paths.setdefault(p.stem.lower(), []).append(p)
        if label_lower in lower_to_paths:
            matched = lower_to_paths[label_lower]
            if len(matched) == 1:
                return (load_state(matched[0]), "ok", label)
    return (None, "no_state", label)
