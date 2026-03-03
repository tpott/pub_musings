# Ralph: Multi-Project Orchestrator

Ralph is an autonomous development agent that manages multiple projects from a
single long-lived process. It runs in tmux, repeatedly invoking Claude with a
multi-project-aware prompt.

## Quick Start

```bash
# From pub_musings/ repo root:
python -m ralph -n 5

# Preview status without invoking Claude:
python -m ralph --dry-run
```

## Installation

```bash
# From repo root (editable install):
pip install -e ralph/
```

After installing, `python -m ralph` works from any directory — Ralph
auto-detects the repo root and `--project` from your CWD.

## Usage

```
python -m ralph [OPTIONS]

Options:
  -v, --verbose             Stream all JSON output (default: dots for progress)
  -n, --max-iterations N    Maximum iterations (default: from config or 10)
  --config PATH             Path to projects.json (default: ralph/projects.json)
  --project NAME            Focus on a single project
  --log-dir DIR             Log directory (default: ~/.ralph/logs/)
  --dry-run                 Show project status and readiness, then exit
```

## Running from Project Directories

After `pip install -e ralph/`, you can run Ralph from inside a project
directory and it will auto-detect the project:

```bash
cd peekaboo/
python -m ralph --dry-run    # auto-detects --project peekaboo
```

Ralph finds the repo root via `git rev-parse --show-toplevel` (with a
fallback of walking up from CWD looking for `ralph/projects.json`), then
infers `--project` from the first path component of your CWD relative to
the repo root.

## Configuration

Projects are registered in `ralph/projects.json`:

```json
{
  "projects": {
    "peekaboo": {
      "feedback_script": "peekaboo/scripts/fetch-feedback.py",
      "implementation_plan": "peekaboo/plan.md",
      "lint_commands": ["cd peekaboo && ./scripts/lint.sh"],
      "test_commands": ["cd peekaboo/backend && go test ./..."]
    }
  },
  "defaults": {
    "model": "opus",
    "max_iterations": 10,
    "stop_file": "STOP_RALPH"
  }
}
```

## Stopping Ralph

Create `STOP_RALPH` at the repo root:

```bash
touch STOP_RALPH
```

Ralph checks for this file before each iteration and stops gracefully.

## Logs

Logs are written to `~/.ralph/logs/` by default:
- `ralph-<ID>.log` — Full session log with JSON output
- `feedback-<ID>.log` — Feedback content before/after processing

## Contributing

From the `ralph/` directory with the venv activated:

```bash
# Run tests
python -m unittest discover tests/ -v

# Format code
python -m black src/ tests/

# Type check
python -m mypy --strict src/ tests/
```

All three must pass before submitting changes.

## Architecture

```
ralph/
├── pyproject.toml          — Package config (src layout)
├── projects.json           — Project registry
├── RALPH.md                — Prompt template sent to Claude each iteration
├── src/ralph/
│   ├── __main__.py         — Entry point for `python -m ralph`
│   ├── loop.py             — Main orchestration loop
│   ├── config.py           — Configuration loading (ProjectConfig, RalphConfig)
│   ├── ralph_optimizer.py  — Log analyzer for cost/pattern analysis
│   └── eval_ralph.py       — Behavioral evals for prompt compliance
├── tests/                  — Unit tests
└── docs/                   — Situation-specific guidance
```

## How It Works

Each iteration:
1. Check for `STOP_RALPH`
2. Fetch feedback for all projects
3. Build prompt from `RALPH.md` + project context table
4. Invoke Claude with the prompt
5. Handle errors (rate limits, API errors with backoff)
6. Repeat
