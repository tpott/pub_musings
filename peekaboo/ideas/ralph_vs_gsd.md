# Ralph vs GSD: A Comparative Analysis

## Executive Summary

**Ralph** and **GSD (Get Shit Done)** are two distinct approaches to AI-assisted autonomous software development. Both solve the same core problem — harnessing Claude Code for reliable, high-quality autonomous coding — but take fundamentally different philosophical approaches.

| Aspect | Ralph | GSD |
|--------|-------|-----|
| **Philosophy** | Minimal infrastructure, trust the agent | Structured orchestration, constrain for quality |
| **Complexity** | ~100 lines of Python + prompt files | ~30 commands, 11 agents, 80+ template files |
| **Context management** | Fresh context each iteration (stateless) | Sophisticated state tracking (stateful) |
| **Planning** | Emergent from iteration | Explicit phases, plans, verification |
| **Human involvement** | Outside the loop (observe & tune) | Inside the loop (discuss, verify, approve) |

---

## The Ralph Pattern

### Origins

Ralph Wiggum is a methodology popularized by [Geoffrey Huntley](https://ghuntley.com/ralph/) in late 2024/early 2025. The name references the Simpsons character — ironically calling the approach "dumb" while it produces sophisticated results.

### Core Mechanism

```bash
while :; do cat RALPH.md | claude -p --dangerously-skip-permissions ; done
```

That's it. A bash loop that feeds a prompt file to Claude, lets it execute autonomously, and repeats.

### File Structure

```
project/
├── loop.sh                    # The bash loop (or ralph.py for more control)
├── RALPH.md                   # Core instructions (the "brain")
├── PROGRESS.md                # High-level status summary
├── LEARNINGS.md               # Hard-won lessons (mandatory!)
├── TASKS.jsonl                # Work items with status
├── FEEDBACK.md                # User feedback (created/deleted per cycle)
├── specs/                     # Requirement specifications
│   ├── feature-a.md
│   └── feature-b.md
└── src/                       # Implementation
```

### Key Principles

**1. Context Is Everything**
- LLMs degrade with context accumulation
- Each iteration starts fresh (context isolation)
- Main agent is a scheduler — spawn subagents for actual work
- "Up to 500 parallel Sonnet subagents" for searches/reads

**2. Memory Files ARE the State**
- `PROGRESS.md` — what's done, what exists
- `LEARNINGS.md` — MANDATORY entries when anything fails or surprises
- `TASKS.jsonl` — work queue with "in_progress"/"done"/"wontfix"
- `specs/*.md` — source of truth for requirements

**3. Let Ralph Ralph**
- Trust the agent to self-identify, self-correct, self-improve
- Don't micromanage task order or implementation details
- Eventual consistency through iteration
- "Deterministically bad in an undeterministic world"

**4. Steering via Signals, Not Commands**
- **Upstream:** Existing code patterns shape what gets generated
- **Downstream:** Tests, builds, lints create backpressure
- **Environment:** Add utilities/patterns that Ralph discovers and follows
- **Tuning:** Watch failures, add guardrails reactively ("tune like a guitar")

**5. Plan Is Disposable**
- If trajectory is wrong, delete `IMPLEMENTATION_PLAN.md` and regenerate
- Cost is one planning iteration — cheap vs. going in circles

### Real-World Evidence: Subtitler Project

The subtitler project in this repo shows Ralph in action:

- **482 tasks completed** with this pattern
- **16,100 lines** of Go backend, **1,505+ tests**
- Full-featured app: auth, encryption, video processing, i18n
- `LEARNINGS.md` contains ~100 hard-won lessons
- Git history shows consistent, atomic progress

Example from subtitler's `RALPH.md`:
```markdown
0. **Check for FEEDBACK** - If `FEEDBACK.md` exists then read it, address it, delete it
1. **Study your specs** - Read `PROGRESS.md`, `LEARNINGS.md`, `README.md` and specs/*.md
2. **Check current behavior** - Does the project build, do tests pass. Fix before new work.
3. **Pick a task** - ONE task from `TASKS.jsonl`, mark "in_progress"
4. **Verify task is done** - Tests pass, feature works. Run every command you wrote.
5. **Commit** - Update memory files, then commit.
```

---

## The GSD Pattern

### Origins

GSD was created by TÂCHES (glittercowboy on GitHub) as a "context engineering layer that makes Claude Code reliable." It evolved from frustration with enterprise-style spec tools (BMAD, SpecKit) being overcomplicated.

### Core Mechanism

A sophisticated command system with specialized agents:

```bash
/gsd:new-project        # Initialize with deep questioning
/gsd:discuss-phase 1    # Capture implementation preferences
/gsd:plan-phase 1       # Research + plan + verify
/gsd:execute-phase 1    # Parallel execution with fresh contexts
/gsd:verify-work 1      # User acceptance testing
```

### File Structure

```
project/
├── .planning/
│   ├── PROJECT.md           # Core vision and requirements
│   ├── REQUIREMENTS.md      # Versioned requirements with traceability
│   ├── ROADMAP.md           # Phase structure with progress
│   ├── STATE.md             # Current position, decisions, blockers
│   ├── config.json          # Workflow settings
│   ├── research/            # Domain research documents
│   │   ├── SUMMARY.md
│   │   ├── ARCHITECTURE.md
│   │   ├── FEATURES.md
│   │   ├── STACK.md
│   │   └── PITFALLS.md
│   ├── phases/
│   │   └── 01-name/
│   │       ├── 01-CONTEXT.md      # User decisions (locked!)
│   │       ├── 01-RESEARCH.md     # Phase-specific research
│   │       ├── 01-01-PLAN.md      # Executable plan (XML tasks)
│   │       ├── 01-01-SUMMARY.md   # Completion report
│   │       └── 01-VERIFICATION.md # Phase verification
│   └── codebase/            # Existing codebase analysis
└── .claude/
    ├── commands/gsd/        # 27 GSD slash commands
    ├── agents/              # 11 specialized agent specs
    └── get-shit-done/
        ├── templates/       # Document templates
        ├── workflows/       # Execution workflows
        └── references/      # Planning references
```

### Key Principles

**1. Context Engineering**
- Quality degrades as context fills: 0-30% = PEAK, 70%+ = POOR
- Plans should complete within ~50% context
- Each plan: 2-3 tasks maximum
- Fresh context per plan (subagent isolation)

**2. Plans Are Prompts**
- `PLAN.md` is not a document that becomes a prompt — it IS the prompt
- XML structure optimized for Claude parsing:
```xml
<task type="auto">
  <name>Create login endpoint</name>
  <files>src/app/api/auth/login/route.ts</files>
  <action>Use jose for JWT (not jsonwebtoken - CommonJS issues)...</action>
  <verify>curl -X POST localhost:3000/api/auth/login returns 200</verify>
  <done>Valid credentials return cookie, invalid return 401</done>
</task>
```

**3. Goal-Backward Verification**
- Start with outcome: "What must be TRUE for the goal to be achieved?"
- Derive observable truths (user perspective)
- Derive required artifacts (specific files)
- Identify key links (where it breaks)
- Verify codebase delivers outcomes, not just task completion

**4. Multi-Agent Orchestration**
- Thin orchestrator spawns specialized agents:
  - `gsd-planner` — creates executable plans
  - `gsd-executor` — implements with atomic commits
  - `gsd-verifier` — checks goal achievement
  - `gsd-researcher` — investigates domains
  - `gsd-debugger` — systematic problem solving
- Main context stays at 30-40% while agents do heavy lifting

**5. Wave-Based Parallel Execution**
- Plans grouped by dependency waves
- Wave 1: Independent plans run in parallel
- Wave 2: Plans depending on Wave 1
- Maximizes parallelism, respects dependencies

### Real-World Evidence: Peekaboo Project

The peekaboo project in this directory uses GSD:

- **Phase 1:** Content foundation (crypto + database + API)
- 3 plans completed with atomic commits
- Research documents produced before implementation
- Verification caught API design gap → gap closure plan created
- Git history shows structured progression

Example from peekaboo's verification finding:
```markdown
### API Design Gap (Photo + Audio)
- **Problem:** API returns random item, but Phase 4 needs photo AND audio
- **Solution:** Add `GetPhotoAndAudioForConcept` query
- **Status:** Gap closure plan 01-03 created
```

---

## Head-to-Head Comparison

### Philosophy

| Ralph | GSD |
|-------|-----|
| Embrace chaos, iterate to convergence | Structure prevents chaos from starting |
| Trust agent judgment | Constrain agent to prevent degradation |
| Simple primitives, emergent behavior | Complex scaffolding, predictable behavior |
| "Let Ralph Ralph" | "The complexity is in the system, not your workflow" |

### Planning

| Ralph | GSD |
|-------|-----|
| Optional IMPLEMENTATION_PLAN.md | Mandatory phase/plan hierarchy |
| Plan generated during PLANNING loop | Plan created by gsd-planner agent |
| Ralph picks "most important" task | Plans execute in dependency order |
| Regenerate when wrong | Revise based on checker feedback |

### Task Granularity

| Ralph | GSD |
|-------|-----|
| One task per loop iteration | 2-3 tasks per plan |
| Task scope: whatever Ralph judges | Task scope: 15-60 minutes Claude execution |
| No explicit task format | XML structure with <action>, <verify>, <done> |

### Human Involvement

| Ralph | GSD |
|-------|-----|
| **Outside the loop** | **Inside the loop** |
| Watch, observe patterns, tune prompts | Discuss phase, verify work, approve plans |
| FEEDBACK.md for async input | AskUserQuestion during execution |
| Regenerate plan if trajectory wrong | Fix plans via checker/planner iteration |

### State Management

| Ralph | GSD |
|-------|-----|
| Memory files (PROGRESS, LEARNINGS, TASKS) | STATE.md + frontmatter graphs |
| State is markdown files Claude reads | Structured YAML tracking position/decisions |
| No explicit "where am I" tracking | `/gsd:progress` shows exact position |

### Verification

| Ralph | GSD |
|-------|-----|
| Implicit via backpressure (tests/builds) | Explicit verification phase |
| LEARNINGS.md captures failures | VERIFICATION.md captures gaps |
| Run commands you document | Goal-backward methodology |

### Commits

| Ralph | GSD |
|-------|-----|
| Per-iteration (after task done) | Per-task atomic commits |
| Simple commit messages | Structured format: `{type}({phase}-{plan}): {desc}` |
| Memory files updated in same commit | Planning docs committed separately |

---

## Strengths and Weaknesses

### Ralph Strengths

1. **Extreme simplicity** — A bash loop and some markdown files
2. **Low overhead** — No installation, no dependencies beyond Claude CLI
3. **Self-correcting** — Errors get fixed in subsequent iterations
4. **Scales via subagents** — Main context stays clean
5. **Battle-tested** — 482+ tasks completed on subtitler project
6. **Lightweight persistence** — Memory files are human-readable state
7. **Adaptable** — Works with any Claude CLI-compatible tool

### Ralph Weaknesses

1. **No guardrails** — Ralph can go in circles if prompts aren't tuned
2. **Requires observation** — Someone should watch early iterations
3. **Task selection is probabilistic** — "Most important" is subjective
4. **No explicit planning verification** — Plans validated through execution
5. **Learning curve for tuning** — Knowing what signals to add takes experience

### GSD Strengths

1. **Context quality preservation** — Plans sized to stay in "smart zone"
2. **Explicit verification** — Goal-backward methodology catches gaps
3. **Parallelization** — Wave-based execution maximizes throughput
4. **Rich state tracking** — Know exactly where you are
5. **User decision capture** — CONTEXT.md locks in preferences
6. **Research integration** — Domain knowledge gathered before planning
7. **UAT workflow** — Human verification built into process

### GSD Weaknesses

1. **High complexity** — 30 commands, 11 agents, 80+ files
2. **Installation friction** — `npx get-shit-done-cc` setup required
3. **Overhead for small tasks** — `/gsd:quick` exists but still heavier than Ralph
4. **Learning curve** — Understanding the full system takes time
5. **Rigidity** — Structure can feel constraining for exploratory work
6. **Coupling** — Tied to specific file structures and conventions

---

## When to Use Which

### Use Ralph When:

- Building a single focused product over time (like subtitler)
- You want to observe AI behavior and tune prompts interactively
- The project will run for many iterations (100+ tasks)
- You prefer minimal tooling overhead
- You're comfortable with probabilistic task selection
- You want maximum flexibility in how work is structured

### Use GSD When:

- Starting a new project with clear phases
- You want structured handoffs between work sessions
- The project has multiple distinct milestones
- You prefer explicit verification checkpoints
- You want parallel execution of independent work
- You value explicit planning and user decision capture

### Hybrid Approach (Peekaboo's Pattern)

Interestingly, this peekaboo project shows a hybrid:

- Uses **GSD structure** (`.planning/` with phases, plans, verification)
- Has **RALPH.md** in the root (Ralph-style instructions)
- Combines structured planning with iteration-friendly memory files
- Gets benefits of both: phase structure + self-correction

The commit `9b86f08 "Lets try adding ralph to GSD"` shows explicit experimentation with combining the approaches.

---

## Key Takeaways

### Common Ground

Both patterns recognize:
1. **Context degrades with accumulation** — Fresh contexts are essential
2. **Subagents enable scale** — Spawn workers, keep main context clean
3. **Tests/builds create backpressure** — Automated checks catch mistakes
4. **Memory persists in files** — The filesystem is the state store
5. **Claude can self-direct** — Given good prompts, it makes good decisions

### Core Difference

**Ralph:** "Create an environment where the AI can succeed through iteration"

**GSD:** "Create a structure that prevents the AI from failing"

Both work. Ralph is more organic and adaptable. GSD is more predictable and verifiable. The choice depends on your preference for emergence vs. structure.

### The Meta-Lesson

The existence of both patterns validates a key insight: **Claude Code is powerful enough that multiple meta-approaches work**. The bottleneck isn't Claude's capability — it's context management and human oversight.

Whether you prefer Ralph's dumb loop or GSD's sophisticated orchestration, you're solving the same problem: keeping Claude in its quality zone while letting it do the work.

---

## Sources

### Ralph
- [ghuntley.com/ralph](https://ghuntley.com/ralph/) — Original methodology post
- [how-to-ralph-wiggum](https://github.com/ghuntley/how-to-ralph-wiggum) — Detailed playbook
- `pub_musings/subtitler/` — 482-task production example
- `pub_musings/subtitler/RALPH.md` — Live implementation

### GSD
- [get-shit-done](https://github.com/glittercowboy/get-shit-done) — Source repo
- `pub_musings/peekaboo/.planning/` — Production implementation
- `~/.claude/commands/gsd/` — Command definitions
- `~/.claude/agents/gsd-*.md` — Agent specifications

---

*Generated: 2026-02-03*
*Methodology: Parallel exploration agents + manual deep-dive of both codebases*
