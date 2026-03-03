# Ralph: Multi-Project Orchestrator

Ralph is an autonomous development agent that manages multiple projects from a
single long-lived process. It runs from the repo root in tmux, repeatedly
invoking Claude with a multi-project-aware prompt.

## Quick Start

```bash
# From pub_musings/ repo root:
python ralph/ralph.py -n 5 -v
```

## Usage

```
python ralph/ralph.py [OPTIONS]

Options:
  -v, --verbose             Stream all JSON output (default: dots for progress)
  -n, --max-iterations N    Maximum iterations (default: from config or 10)
  --config PATH             Path to projects.json (default: ralph/projects.json)
  --project NAME            Focus on a single project
  --log-dir DIR             Log directory (default: ~/.ralph/logs/)
```

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

## Running Tests

```bash
cd ralph
python -m pytest test_ralph.py test_ralph_optimizer.py test_config.py
```

## Architecture

- `ralph.py` — Main orchestration loop
- `config.py` — Configuration loading (ProjectConfig, RalphConfig)
- `projects.json` — Project registry
- `RALPH.md` — Prompt template sent to Claude each iteration
- `ralph_optimizer.py` — Log analyzer for cost/pattern analysis
- `eval_ralph.py` — Behavioral evals for prompt compliance
- `docs/` — Situation-specific guidance (dependencies, escalation, etc.)

## How It Works

Each iteration:
1. Check for `STOP_RALPH`
2. Fetch feedback for all projects
3. Build prompt from `RALPH.md` + project context table
4. Invoke Claude with the prompt
5. Handle errors (rate limits, API errors with backoff)
6. Repeat
