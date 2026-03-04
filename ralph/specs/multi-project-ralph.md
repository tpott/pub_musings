# Plan: Extract ralph into a multi-project orchestrator

## Context

ralph.py (~500 lines), ralph_optimizer.py (~850 lines), eval_ralph.py, and their tests are **duplicated** between peekaboo/ and subtitler/. The peekaboo versions are the superset (more tests, newer fetch-feedback cursor logic, situation links in RALPH.md). diy-ralph/ is the upstream reference.

This plan creates `ralph/` in pub_musings/ as a **single long-lived orchestrator** that runs from repo root in tmux, manages multiple projects, and lets Claude decide which project to prioritize each iteration.

---

## Phase 1: Create ralph/ directory structure

Create these new files:

```
ralph/
├── __init__.py              # Empty (makes tests importable)
├── ralph.py                 # Core loop (extracted from peekaboo, modified)
├── ralph_optimizer.py       # Log analyzer (copied from peekaboo, unchanged)
├── eval_ralph.py            # Behavioral evals (updated paths)
├── config.py                # NEW: ProjectConfig dataclass + load_config()
├── projects.json            # NEW: Project registry
├── RALPH.md                 # Shared prompt template (multi-project aware)
├── test_ralph.py            # Merged superset (peekaboo version)
├── test_ralph_optimizer.py  # Copied from peekaboo
├── test_config.py           # NEW: Tests for config loading
├── requirements-ralph.txt   # black, mypy
├── docs/
│   ├── DEPENDENCIES.md      # Moved from peekaboo/docs/ralph/
│   ├── ESCALATION.md
│   ├── LEARNINGS-FORMAT.md
│   └── RESEARCH.md
└── README.md                # Usage docs
```

**Source files:**
- `peekaboo/ralph.py` → basis for `ralph/ralph.py` (superset)
- `peekaboo/ralph_optimizer.py` → `ralph/ralph_optimizer.py` (direct copy)
- `peekaboo/eval_ralph.py` → `ralph/eval_ralph.py` (update paths)
- `peekaboo/test_ralph.py` → `ralph/test_ralph.py` (superset with backoff/error tests)
- `peekaboo/test_ralph_optimizer.py` → `ralph/test_ralph_optimizer.py`
- `peekaboo/docs/ralph/*` → `ralph/docs/*`

---

## Phase 2: Create config system (`ralph/config.py` + `ralph/projects.json`) / project registry

**Config format: JSON** (stdlib in Python 3.10+, no deps)

`ralph/projects.json`:
```json
{
  "projects": {
    "peekaboo": {
      "feedback_script": "peekaboo/scripts/fetch-feedback.py",
      "implementation_plan": "peekaboo/plan.md",
      "lint_commands": ["cd peekaboo && ./scripts/lint.sh"],
      "test_commands": ["cd peekaboo/backend && go test ./..."],
    },
    "subtitler": {
      "feedback_script": "subtitler/scripts/fetch-feedback.py",
      "implementation_plan": "subtitler/specs/subtitler.md", // used to be subtitler/001_RALPH_SUBTITLER.md
      "lint_commands": ["cd subtitler && ./scripts/lint.sh"],
      "test_commands": ["cd subtitler/backend && go test ./..."],
    }
  },
  "defaults": {
    "model": "opus",
    "max_iterations": 10,
    "stop_file": "STOP_RALPH"
  }
}
```

`ralph/config.py`: `ProjectConfig` and `RalphConfig` dataclasses with `load_config(path)`.

All paths relative to repo root. `status_file`, `tasks_file`, `learnings_file` relative to project `path`. Lint/test commands are free-form strings (bazel-ready in future).

---

## Phase 3: Modify ralph.py for multi-project orchestration

Key changes to `peekaboo/ralph.py` → `ralph/ralph.py`:

1. **Replace hardcoded constants** with config-driven values
   - `PROMPT_FILE` → `ralph/RALPH.md`
   - `FEEDBACK_FILE` → per-project from config
   - `FETCH_FEEDBACK_SCRIPT` → per-project from config

2. **New CLI args:**
   - `--config PATH` (default: `ralph/projects.json`)
   - `--project NAME` (optional: focus on single project)

3. **`build_prompt(config)`** — reads `ralph/RALPH.md` template, appends:
   - Active Projects table (name, path, pending task count, feedback waiting Y/N)
   - Per-project details block (status file, tasks, lint/test commands, specs dir, etc.)

4. **`fetch_all_feedback(config)`** — loops over all projects, runs each project's feedback script (30s timeout each, failures don't block)

5. **`count_pending_tasks(project)`** — reads project's TASKS.jsonl, counts `status=todo`

6. **`check_feedback_waiting(project)`** — checks if project's FEEDBACK.md exists

7. **Single `STOP_RALPH` at repo root** — one process, one stop mechanism

8. **CWD stays at repo root** — Claude invoked from pub_musings/, all paths project-prefixed

9. **Better defaults** - ralph.py should have `--log-dir` default to `~/.ralph/logs/`. If the log dir doesn't exist, ralph.py should effectively `mkdir -p` it first.

---

## Phase 4: Create shared RALPH.md template

`ralph/RALPH.md` — project-agnostic template based on peekaboo's version:

- Step 0: Check feedback across all projects (feedback files listed in appended context)
- Step 1: Pick a project (prioritize feedback, then highest-priority tasks)
- Step 2: Study chosen project's specs (status file, learnings, README, AGENTS.md)
- Step 3: Check build/tests using project's configured commands
- Step 4: Pick ONE task from project's TASKS.jsonl, claim it
- Step 5: Verify done — run tests, run documented commands
- Step 6: Commit — update memory files, then commit

Rules section emphasizes CWD is repo root, all paths project-prefixed. Situation links point to `ralph/docs/`.

---

## Phase 5: Remove duplicated files from projects

**Remove from peekaboo/:**
- `ralph.py`, `ralph_optimizer.py`, `eval_ralph.py`
- `test_ralph.py`, `test_ralph_optimizer.py`
- `requirements-ralph.txt`
- `RALPH.md`
- `docs/ralph/` directory (moved to `ralph/docs/`)

**Remove from subtitler/:**
- Same files as peekaboo (minus `docs/ralph/` which didn't exist there)
- `RALPH.md`

**Keep in each project** (project-specific):
- `TASKS.jsonl`, `STATUS.md`/`PROGRESS.md`, `LEARNINGS.md`
- `AGENTS.md`, `CLAUDE.md`
- `scripts/fetch-feedback.py` (project-specific API logic)
- `specs/` directory
- `.feedback-cursor`, `FEEDBACK.md` (gitignored)

---

## Phase 6: Tests and verification

1. Run: `python -m pytest ralph/test_ralph.py ralph/test_ralph_optimizer.py ralph/test_config.py`
2. Smoke test: `python ralph/ralph.py --config ralph/projects.json -n 1 -v`
3. Verify feedback fetching works for both projects

If needed: `source ralph/.venv/bin/activate && python -m pip install -r ralph/requirements.txt`

---

## Design decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Config format | JSON | stdlib 3.10+, no deps |
| Prompt construction | Template + appended project context | Keeps RALPH.md clean |
| Feedback | Per-project files, not aggregated | Scripts/formats differ per project |
| Stop mechanism | Single STOP_RALPH at repo root | One process |
| CWD | Always repo root | All paths project-prefixed |
| Project selection | Claude decides based on context | User preference: ralph prioritizes itself |
| Linting | Config-declared commands | Flexible, bazel-ready |
