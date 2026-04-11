"""Pipeline DAG runner: execute stages in order with state persistence."""

from datetime import datetime


def run_pipeline(stages, conf, state, save_fn):
    """Run pipeline stages in order, skipping completed ones.

    Each stage is a tuple of (name, dependencies, fn).
    fn signature: fn(conf, state) -> state
    save_fn is called after each stage transition.
    On failure, marks the stage as failed and re-raises.
    """
    for name, deps, fn in stages:
        stage_info = state["stages"].get(name, {})
        if stage_info.get("status") == "complete":
            continue

        state["stages"][name] = {"status": "running"}
        save_fn(state)

        try:
            state = fn(conf, state)
            state["stages"][name] = {
                "status": "complete",
                "completed_at": datetime.now().isoformat(),
            }
            save_fn(state)
        except Exception:
            state["stages"][name] = {
                "status": "failed",
                "completed_at": datetime.now().isoformat(),
            }
            state["status"] = "failed"
            save_fn(state)
            raise

    state["status"] = "complete"
    save_fn(state)
    return state
