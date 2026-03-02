# Ralph Improvements

Suggestions for improving your custom Ralph implementation based on our analysis of Ralph, GSD, and what worked/didn't work.

## Problem Summary

| Issue | Root Cause |
|-------|------------|
| Dependencies added without research | No guardrails in prompt |
| No docs after decisions | Not required by prompt |
| GSD slash commands don't work with ralph.py | AskUserQuestion blocks without human input |
| RALPH.md could grow too large | Need conditional loading pattern |

---

## 1. Lean RALPH.md with Situation Links

Keep the core loop tight (~400 tokens). Link to detailed docs that are only read when relevant.

```markdown
# RALPH

You are Ralph. You work autonomously, one task at a time.

## Every Iteration

1. **Feedback** → If `FEEDBACK.md` exists: read, address, delete
2. **Orient** → Read `STATUS.md`, check recent `LEARNINGS.md` entries
3. **Check** → Does it build? Tests pass? Fix before new work
4. **Pick** → One task from `TASKS.jsonl`, mark `in_progress`
5. **Execute** → Implement, verify, update memory files
6. **Commit** → Atomic commit, clear message

## Situation Links

Follow these ONLY when the situation applies:

| Situation | Read First |
|-----------|------------|
| Adding a dependency | [docs/ralph/DEPENDENCIES.md](docs/ralph/DEPENDENCIES.md) |
| Architecture decision | [docs/ralph/DECISIONS.md](docs/ralph/DECISIONS.md) |
| Something surprised you | [docs/ralph/LEARNINGS-FORMAT.md](docs/ralph/LEARNINGS-FORMAT.md) |
| Need external research | [docs/ralph/RESEARCH.md](docs/ralph/RESEARCH.md) |
| Stuck > 2 attempts | [docs/ralph/ESCALATION.md](docs/ralph/ESCALATION.md) |
| Starting a new phase | [docs/ralph/PHASES.md](docs/ralph/PHASES.md) |

## Memory Files

| File | Purpose | Update When |
|------|---------|-------------|
| `STATUS.md` | What exists, what works | After features complete |
| `LEARNINGS.md` | Hard-won lessons | Failures, surprises, decisions |
| `TASKS.jsonl` | Work queue | Pick/complete tasks |

## Rules

- One task per iteration
- Commit after each task
- Spawn subagents for searches/reads (preserve main context)
- If uncertain, bias toward action — fix in next iteration
```

---

## 2. Situation Documents

### docs/ralph/DEPENDENCIES.md

```markdown
# Adding Dependencies

## Before Adding ANY Dependency

1. **WebSearch** for: `"{package}" alternatives comparison 2026`
2. **Evaluate** at least 2 alternatives:
   - Maintenance status (last commit, open issues, bus factor)
   - Size impact (bundle size, binary size, transitive deps)
   - Security (known CVEs, audit history)
   - API ergonomics (does it fit our patterns?)

3. **Document** in `LEARNINGS.md` using Decisions format (see LEARNINGS-FORMAT.md)

4. **Only then** add to package.json / go.mod / requirements.txt

## Red Flags — Require Extra Scrutiny

- Last commit > 6 months ago
- < 100 GitHub stars for core functionality
- No TypeScript types (for JS packages)
- Excessive transitive dependencies
- Known security issues in last 12 months

## Approved Patterns

Prefer these known-good choices when applicable:

| Need | Go | TypeScript |
|------|-----|------------|
| HTTP router | stdlib `net/http` | - |
| Database | `mattn/go-sqlite3` | - |
| Encryption | `filippo.io/age` | - |
| Testing | stdlib `testing` | `vitest` |
| Validation | - | `zod` |
```

### docs/ralph/LEARNINGS-FORMAT.md

```markdown
# LEARNINGS.md Entry Formats

## When to Add Entries

MANDATORY for:
- New dependency added
- Architecture decision made
- Non-obvious workaround discovered
- Something that surprised you
- Bug that took > 1 attempt to fix

## Lesson Format (bugs, surprises, workarounds)

### YYYY-MM-DD: Brief title

**Problem:** What happened (symptoms, not causes)

**Solution:** How you fixed it (be specific)

**Lesson:** What future Ralphs should know to avoid this

## Decision Format (dependencies, architecture, design)

### YYYY-MM-DD: Brief title

**Context:** What situation required a decision

**Options considered:**
- Option A: pros/cons
- Option B: pros/cons

**Decision:** What was chosen and why

**Sources:**
- URL 1
- URL 2

**Outcome:** (update later) How it worked out
```

### docs/ralph/RESEARCH.md

```markdown
# Research Protocol

When facing unknowns or making significant decisions, research before implementing.

## When to Research

- New domain/technology you haven't used before
- Performance-critical code paths
- Security-sensitive implementations
- Integration with external services

## Research Steps

1. **WebSearch** with specific queries:
   - `"{topic}" best practices 2026`
   - `"{technology}" vs "{alternative}" comparison`
   - `"{problem}" common pitfalls`

2. **Document findings** with:
   - Key insights (bullet points)
   - Code examples (if applicable)
   - Source URLs (for verification)
   - Confidence level: HIGH / MEDIUM / LOW

3. **Store in appropriate location:**
   - Project-wide knowledge → `.planning/research/` or `specs/`
   - Implementation-specific → `LEARNINGS.md`

## Research Output Template

### Topic: {topic}

**Researched:** YYYY-MM-DD
**Confidence:** HIGH / MEDIUM / LOW

**Summary:**
- Key point 1
- Key point 2

**Recommended approach:** {description}

**Alternatives considered:**
- {alt1}: {why not}
- {alt2}: {why not}

**Sources:**
- {url1}
- {url2}
```

### docs/ralph/ESCALATION.md

```markdown
# Escalation Protocol

When stuck after multiple attempts, escalate rather than thrash.

## Signs You Should Escalate

- Same error after 3+ different fix attempts
- Unclear requirements (spec is ambiguous)
- Need access/permissions you don't have
- Architectural question with no clear answer

## Escalation Actions

1. **Document the blocker** in HELP.md:
   ```markdown
   ## Blocked: {brief description}

   **Task:** {task from TASKS.jsonl}

   **Attempts:**
   1. Tried X → failed because Y
   2. Tried A → failed because B

   **Need:** {what would unblock you}
   ```

2. **Mark task** as `blocked` in TASKS.jsonl (add `blocked_reason` field)

3. **Pick different task** — don't thrash on the same problem

4. **Human will address** HELP.md and update you
```

### docs/ralph/PHASES.md

```markdown
# Working with GSD Phase Structure

Ralph can work directly with GSD's `.planning/` files without using slash commands.

## Phase Workflow

When starting work on a phase:

1. **Read current state:**
   - `.planning/STATE.md` — where are we?
   - `.planning/ROADMAP.md` — what phases exist?

2. **Check phase directory** (`.planning/phases/NN-name/`):

   | Missing File | Action |
   |--------------|--------|
   | `NN-CONTEXT.md` | Read ROADMAP.md, write CONTEXT.md with implementation decisions |
   | `NN-RESEARCH.md` | WebSearch the domain, write RESEARCH.md with sources |
   | `NN-01-PLAN.md` | Write PLAN.md based on research |
   | All plans done | Write VERIFICATION.md, update STATE.md |

3. **If PLAN.md exists:** Execute next incomplete task, commit atomically

## File Formats

Follow templates in `.claude/get-shit-done/templates/`:
- `context.md` — phase boundaries and decisions
- `research.md` — domain research with sources
- `phase-prompt.md` — plan structure (XML tasks)
- `summary.md` — completion report
- `verification-report.md` — phase verification

## Commit Format

```
{type}({phase}-{plan}): {description}

type: feat, fix, docs, chore, refactor, test
phase: 01, 02, etc.
plan: 01, 02, etc. (omit for phase-level commits)
```

Examples:
- `feat(01-02): add content API endpoints`
- `docs(02): write RESEARCH.md for audio pipeline`
- `chore(01): update STATE.md after phase completion`
```

---

## 3. Mode-Specific Prompts

Instead of one RALPH.md, use two prompts and switch manually:

### RALPH_plan.md

```markdown
# RALPH — Planning Mode

You are Ralph in PLANNING mode. You create and update plans, not code.

## This Iteration

1. **Read** `STATUS.md`, `LEARNINGS.md`, specs/*.md
2. **Gap analysis:** What do specs require that code doesn't have?
3. **Update** `TASKS.jsonl` with prioritized work items
4. **If using GSD structure:** Update `.planning/` files (ROADMAP, PLAN.md)
5. **Commit** planning docs only

## Output

- Updated `TASKS.jsonl` with clear `done_when` criteria
- Updated planning docs if applicable
- NO code changes, NO implementation

## When to Exit Planning Mode

- TASKS.jsonl has 5+ well-defined tasks
- Each task has clear acceptance criteria
- Dependencies between tasks are noted
- Human switches to RALPH_build.md
```

### RALPH_build.md

```markdown
# RALPH — Building Mode

You are Ralph in BUILDING mode. You implement from the plan.

## This Iteration

1. **Feedback** → If `FEEDBACK.md` exists: read, address, delete
2. **Orient** → Read `STATUS.md`, recent `LEARNINGS.md`
3. **Check** → Build passes? Tests pass? Fix first
4. **Pick** → ONE task from `TASKS.jsonl`, mark `in_progress`
5. **Execute** → Implement, run tests, verify
6. **Update** → Memory files, mark task `done`
7. **Commit** → Atomic commit

## Situation Links

(same as main RALPH.md)

## When to Exit Building Mode

- Stuck on same issue 2+ iterations
- TASKS.jsonl is empty or stale
- Major architectural question emerges
- Human switches to RALPH_plan.md
```

### Switching Modes

```bash
# Planning mode
cp RALPH_plan.md RALPH.md && python ralph.py -n 3

# Building mode
cp RALPH_build.md RALPH.md && python ralph.py -n 10
```

Or modify ralph.py to accept a mode flag:

```python
# ralph.py additions
parser.add_argument(
    "--mode",
    choices=["plan", "build"],
    default="build",
    help="Which prompt to use (RALPH_plan.md or RALPH_build.md)"
)

# In main():
prompt_file = Path(f"RALPH_{args.mode}.md") if args.mode else Path("RALPH.md")
```

---

## 4. Enhanced ralph.py

### Add Feedback Fetching

Your ralph.py already has `fetch_feedback()`. Make sure it's wired up:

```python
# Before each iteration
fetch_feedback(log_file, args.auto_fetch_feedback_script)
```

### Add Iteration Summary

Log what happened each iteration for easier debugging:

```python
def summarize_iteration(last_log: dict, log_file: Path | None) -> None:
    """Extract and log key actions from the iteration."""
    result = last_log.get("result", "")

    # Look for patterns
    patterns = {
        "committed": r"committed|commit [a-f0-9]{7}",
        "task_done": r"marked.*done|status.*done",
        "blocked": r"blocked|stuck|failed",
        "researched": r"WebSearch|researched",
    }

    actions = [name for name, pattern in patterns.items()
               if re.search(pattern, result, re.IGNORECASE)]

    if actions:
        log(f"Actions: {', '.join(actions)}", log_file)
```

### Add Stop Conditions

Stop early if Ralph is thrashing:

```python
# Track repeated failures
failure_count = 0
MAX_CONSECUTIVE_FAILURES = 3

for i in range(max_iterations):
    # ... run iteration ...

    if "blocked" in result.lower() or "failed" in result.lower():
        failure_count += 1
        if failure_count >= MAX_CONSECUTIVE_FAILURES:
            log(f"Stopping: {failure_count} consecutive failures", log_file)
            break
    else:
        failure_count = 0  # Reset on success
```

---

## 5. Git Integration

### Auto-Push to Branch

Add to ralph.py after successful iterations:

```python
def push_if_commits(log_file: Path | None) -> None:
    """Push to current branch if there are unpushed commits."""
    result = subprocess.run(
        ["git", "status", "--porcelain", "--branch"],
        capture_output=True, text=True
    )
    if "[ahead" in result.stdout:
        subprocess.run(["git", "push"], capture_output=True)
        log("Pushed commits to remote", log_file)
```

### PR Gating (Keep Manual)

Your current approach is correct — let Ralph commit and push, but require manual PR creation:

```bash
# After Ralph session
gh pr create --base trunk --head $(git branch --show-current) --fill
```

---

## 6. GSD Integration Without Slash Commands

If you want GSD's structure with Ralph's autonomy, use this hybrid approach:

### RALPH.md (GSD-aware)

```markdown
# RALPH

You are Ralph. You work autonomously using GSD's planning structure.

## Every Iteration

1. **Feedback** → If `FEEDBACK.md` exists: read, address, delete
2. **State** → Read `.planning/STATE.md` to find current phase
3. **Check** → Build passes? Tests pass? Fix first
4. **Phase work:**
   - No CONTEXT.md? → Write it (implementation decisions)
   - No RESEARCH.md? → WebSearch, write with sources
   - No PLAN.md? → Write based on research
   - PLAN.md exists? → Execute next incomplete task
   - All tasks done? → Write VERIFICATION.md, update STATE.md
5. **Commit** → `{type}({phase}-{plan}): {description}`

## Situation Links

| Situation | Read |
|-----------|------|
| Adding dependency | [docs/ralph/DEPENDENCIES.md](docs/ralph/DEPENDENCIES.md) |
| Writing RESEARCH.md | [docs/ralph/RESEARCH.md](docs/ralph/RESEARCH.md) |
| Stuck > 2 attempts | [docs/ralph/ESCALATION.md](docs/ralph/ESCALATION.md) |

## Research Protocol

When writing RESEARCH.md or investigating unknowns:
- WebSearch for current best practices (include "2026" in queries)
- Document sources with URLs
- Note confidence levels (HIGH/MEDIUM/LOW)
- Include code examples where helpful
```

This gives you:
- GSD's phase structure and research rigor
- Ralph's autonomous execution loop
- No interactive prompts blocking progress

---

## Summary of Changes

| Change | Benefit |
|--------|---------|
| Lean RALPH.md + situation links | Saves tokens, loads detail on-demand |
| DEPENDENCIES.md protocol | Prevents blind dependency additions |
| LEARNINGS.md requirements | Forces documentation of decisions |
| Plan/Build mode split | Clearer separation of concerns |
| FEEDBACK.md pattern | Async human input without blocking |
| GSD file integration | Structure without interactive commands |
| Enhanced ralph.py | Better logging, failure detection |

---

*Generated: 2026-02-03*
