# dvd-ripper: Agent-Based Post-Rip Verification & Error Recovery

## Context

The dvd-ripper pipeline currently runs linearly: udev → rip → transcode → sync → notify. There's no verification that the right episodes were ripped and no detection of combined/missing episodes. This plan adds automated post-transcode verification (episode count, duration anomaly, title-frame OCR via Claude vision) and restructures `rip.py` into an explicit stage DAG so verification can be slotted in cleanly between transcode and sync — preventing bad rips from ever reaching Jellyfin.

## Pipeline DAG

The current `rip.py:main()` is a linear sequence of side-effecting calls. We make the dependencies explicit (Airflow-style) so verification can slot in as one new node and a `--resume` runner can re-enter the DAG at any point.

```python
# pipeline.py
STAGES = [
    ("scan_disc",   [],              stage_scan_disc),    # blkid + makemkvcon info
    ("plan",        ["scan_disc"],   stage_plan),         # detect media_type, build jobs
    ("rip",         ["plan"],        stage_rip),          # makemkvcon mkv per title → .mkv
    ("transcode",   ["rip"],         stage_transcode),    # HandBrake per job → local .mp4
    ("verify",      ["transcode"],   stage_verify),       # NEW: count + duration + OCR
    ("sync",        ["verify"],      stage_sync),         # rsync to backup host
    ("finalize",    ["sync"],        stage_finalize),     # eject + single notify
]
```

Each stage is `fn(conf, state) -> state`. A small toposort runner walks the list, persists `state.json` between stages, and on failure leaves state on disk for `--resume`. Note: `verify` slots in **between** `transcode` and `sync` so a bad rip never reaches Jellyfin. This requires splitting today's `transcode_and_sync` (transcode.py:36) into two stages — the per-job rsync at the bottom of the loop moves into `stage_sync`.

### Resume flow
`rip.py --resume <state-file>` loads state.json, finds the first incomplete stage, and runs from there. No disc access needed if `rip` is already done.

## Phase 1: State Management — `state.py` + `tests/test_state.py`

New file `state.py` (~100 lines). Handles saving/loading pipeline state for resume.

### State file location

- **Active:** `RIP_DIR/.state/<disc-label>.json` — one file per disc, keyed on the blkid label so `rip.py --resume` (no args) can re-discover it from the inserted disc.
- **Completed:** archived on success to `RIP_DIR/.state/history/<disc-label>-<ISO-timestamp>.json`. Kept indefinitely so we can mine past rips for test fixtures alongside the journald logs.
- **Failed:** stays in `.state/<disc-label>.json` until a `--resume` clears it.
- **Collision guard:** starting a fresh rip when `.state/<label>.json` already exists is refused unless `--force` is passed (prevents clobbering an in-progress investigation when the user re-inserts the disc).

`.state/` is a sibling of `TV/` and `Movies/`, gitignored, and never picked up by the rsync to Jellyfin (originally I'd planned to drop it inside `output_dir`, but that path gets synced to the backup host).

### Schema

```python
{
    "version": 1,
    "status": "running|complete|failed",
    "created_at": "ISO-8601",
    "updated_at": "ISO-8601",
    "disc_label": "AVATAR_BOOK_1_DISC_1",
    "media_type": "tv",
    "stages": {
        "scan_disc": {"status": "complete", "completed_at": "..."},
        "rip":       {"status": "complete", "completed_at": "..."},
        "transcode": {"status": "complete", "completed_at": "..."},
        "verify":    {"status": "failed",   "completed_at": "..."},
        "sync":      {"status": "pending"},
        "finalize":  {"status": "pending"}
    },
    "plan": {
        "show_name": "Avatar", "season": 1, "disc": 1,
        "output_dir": "...",
        "episodes": [
            {"title_id": 0, "ep_name": "Avatar S01E01", "mp4_path": "..."}
        ]
    },
    "verification": {
        "issues": [
            {
                "type": "title_mismatch|missing_episode|combined_episode|duration_anomaly",
                "episode": "Avatar S01E03",
                "detail": "Title card reads 'The Chase' but expected 'The Storm'",
                "auto_fix_applied": null
            }
        ]
    }
}
```

### Functions
- `state_path_for_label(rip_dir, label)` → `RIP_DIR/.state/<label>.json`
- `discover_active_state(rip_dir)` — blkid `/dev/sr0`, return matching state file or None
- `save_state(state)` — atomic write via tmp+rename
- `load_state(path)`
- `archive_state(state)` — move to `.state/history/<label>-<timestamp>.json` on completion

## Phase 2: Pipeline DAG runner + `rip.py` refactor

### New file: `pipeline.py` (~60 lines)
A tiny stage runner. Each stage is `fn(conf, state) -> state`. The runner:
1. Topologically sorts `STAGES` (linear today, but explicit so verify can be inserted as one edit).
2. Skips any stage already marked `complete` in `state.stages`.
3. Calls each pending stage, persists `state.json` after each, marks complete on success.
4. On exception, marks the stage `failed`, persists state, re-raises.

### `rip.py` changes
- Add argparse: `--resume [LABEL_OR_PATH]` (no arg → blkid disc and find state), `--force` (override collision guard).
- Break `main()` into the seven stage functions listed in the DAG. Each takes `(conf, state)` and returns the mutated state.
- Split `transcode_and_sync` (transcode.py:36) into `transcode_only` and `sync_only` so verify can run between them.
- Add `run_capture(cmd)` — like `run()` but returns `CompletedProcess` with captured stdout/stderr. Needed for ffprobe output in verify.

### Resume flow
- `rip.py --resume` → blkid `/dev/sr0`, find `.state/<label>.json`, hand to runner.
- `rip.py --resume <label>` → look up by label without needing the disc.
- `rip.py --resume <path>` → explicit path.
- The runner skips stages already marked `complete` and re-runs from the first non-complete stage. No disc access needed for stages downstream of `rip`.

### Notifications
Unchanged in scope: still exactly **one** `openclaw message send` per pipeline run, fired by `stage_finalize` (or by the top-level exception handler on failure). See Phase 6.

## Phase 4: Post-Rip Verification — `verify.py` + `tests/test_verify.py`

New file `verify.py` (~150 lines). Three checks:

### Check 1: Episode count
Compare number of `.mp4` files against expected count from title selection.

### Check 2: Duration anomaly
Use `ffprobe -v quiet -print_format json -show_format <file>` to get duration.
- Flag episodes >40% deviation from median (wider than title selection's 30%)
- Flag episodes at ~2x median as potential combined episodes

### Check 3: Title frame OCR
- Extract frames at configurable offset (default 75s, based on Avatar experiments) using ffmpeg
- `ffmpeg -ss {offset} -i <file> -vframes 10 -r 1 -q:v 2 {tmpdir}/%03d.jpg`
- Send frames to Claude API for OCR (via `ANTHROPIC_API_KEY` in rip.conf, or skip if not configured)
- Compare detected title text against episode filename/metadata
- Log detected titles in state for user audit

### Auto-fix capabilities
- **Title mismatch** → rename file (safe, applied automatically, recorded in state)
- **Combined episode** → leave as-is, mark in state, log warning. Splitting requires human judgment on the cut point; defer to manual `--resume` after the user inspects the file.
- **Missing episode** → halt the pipeline at `verify` (so `sync` doesn't run), leave state as `failed` for manual `--resume` after re-inserting the disc.

The verify stage is intentionally conservative: it auto-fixes only what it's certain about, and otherwise halts the DAG with state on disk. The user then uses normal tools (their editor, the rip directory) to correct things and runs `rip.py --resume` to continue from `sync`.

## Phase 5: Claude Code Project Skill

### `.claude/skills/title-frame-scanner.md`
Teaches Claude Code to:
1. Accept a video file path
2. Run ffmpeg to extract frames from early in the episode
3. Read frames with Claude vision to OCR title card text
4. Return detected title

This is the only skill in this plan — it's the one piece of verification that benefits from being callable interactively (e.g., "Claude, what's the title card on this episode?") in addition to being invoked from `verify.py`.

## Phase 6: Notification posture (unchanged volume)

**Goal: do not increase the number of openclaw messages.** Today the pipeline sends exactly one per run — the final "Rip complete" message (or a "Rip FAILED" message on exception). Verified by checking journald for the most recent rip (`SHE_RA` 2026-04-03): exactly one `openclaw message send` invocation.

The verification work folds into that single message rather than adding new ones:

- **Success with no issues:** unchanged — `Rip complete: SHE RA S01E01-E02 ready in Jellyfin`
- **Success with auto-fixed issues:** same message, with a one-line suffix listing what was auto-fixed (e.g. `(renamed E03 after title-card OCR)`)
- **Verification halt (sync did not run):** message becomes a single failure-style notification with the issue summary and the path to the state file the user can `--resume` after fixing. Still one message.
- **Hard exception anywhere in the DAG:** existing failure path (one message).

No progress messages, no per-stage milestones, no A/B/C interactive prompts. The journald log + state file are the source of detail; the matrix message stays terse.

## Phase 7: Config & Docs

### `rip.conf.example` additions
```bash
# Verification (optional)
VERIFY_ENABLED="true"
VERIFY_FRAME_OFFSET="75"
ANTHROPIC_API_KEY=""  # For title frame OCR, leave empty to skip
```

### `.gitignore` additions
```
.state/
.frames/
```

### `README.md` updates
Document verification, --resume, license recovery, and the Claude Code skills.

## Files Summary

| New File | Purpose | Est. Lines |
|----------|---------|-----------|
| `pipeline.py` | DAG runner: toposort + per-stage state persistence | ~60 |
| `state.py` | State persistence (active + history dirs) | ~100 |
| `verify.py` | Post-rip verification (count + duration + OCR) | ~150 |
| `.claude/skills/title-frame-scanner.md` | Title OCR skill | ~40 |
| `tests/test_pipeline.py` | DAG runner tests | ~80 |
| `tests/test_state.py` | State tests | ~60 |
| `tests/test_verify.py` | Verification tests | ~120 |

| Modified File | Changes |
|---------------|---------|
| `rip.py` | argparse (`--resume`, `--force`), stage functions, run_capture, wired to `pipeline.run()` |
| `transcode.py` | Split `transcode_and_sync` into `transcode_only` and `sync_only` |
| `rip.conf.example` | `VERIFY_ENABLED`, `VERIFY_FRAME_OFFSET`, `ANTHROPIC_API_KEY` |
| `.gitignore` | `.state/` |
| `README.md` | Document `--resume`, verification, the title-frame-scanner skill |

## Implementation Order

1. `state.py` + `tests/test_state.py` (foundation, no deps)
2. `pipeline.py` + `tests/test_pipeline.py` (DAG runner, tested in isolation)
3. `rip.py` refactor: stage functions + argparse, wire to `pipeline.run()`. Split `transcode_and_sync`. No behavior change yet — verify is a no-op stage.
4. `verify.py` + `tests/test_verify.py`. Implement count + duration checks first; OCR last (gated on `ANTHROPIC_API_KEY`).
5. `.claude/skills/title-frame-scanner.md`
6. Config, gitignore, README updates

## Verification / Testing

- Unit tests: `python3 -m unittest discover tests` (all existing + new test files)
- DAG runner test: stub stages with deterministic side effects, confirm topo order, skip-on-complete, and failure-halt behavior
- Manual test: rip a known disc, confirm verification runs and the single notification still fires
- Resume test: manually mark `verify` as failed in a state file, run `--resume`, confirm it re-runs verify and continues to sync + finalize
- Title frame test: run the `title-frame-scanner` skill against a known Avatar episode, confirm OCR returns title text
