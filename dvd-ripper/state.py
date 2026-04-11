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


def discover_active_state(rip_dir):
    """Find active state file for the currently inserted disc via blkid."""
    try:
        label = subprocess.run(
            ["blkid", "-o", "value", "-s", "LABEL", "/dev/sr0"],
            capture_output=True, text=True, check=True,
        ).stdout.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return None
    path = state_path_for_label(rip_dir, label)
    if path.exists():
        return load_state(path)
    return None
