# Ralph Optimizer

## Overview

`ralph_optimizer.py` analyzes Ralph session logs to identify expensive behaviors, token waste, and optimization opportunities. It reads Claude Code's stream-json logs and produces actionable reports.

## Goals

1. Identify expensive tool call patterns (high token cost operations)
2. Find repeated/redundant file reads that could be cached or pre-computed
3. Detect sessions where breakages weren't caught early (tests should run sooner)
4. Suggest RALPH.md prompt improvements based on behavioral patterns
5. Track cost trends across iterations

## Log File Structure

### Ralph log files

**Location:** `~/.ralph/logs/ralph-<id>.log`

**Format:** Mixed text and JSON lines. Each iteration starts with:
```
=== Iteration N/M === <timestamp>
```

Contains a `session_id` that maps to Claude Code session logs.

### Claude Code session logs

**Location:** `~/.claude/projects/<ralph_runtime_dir>/<session_id>.jsonl`

Where `ralph_runtime_dir` is typically `-home-trevor-pub-musings-subtitler` (path with dashes replacing slashes).

**Format:** JSONL with entries like:
```json
{"type": "assistant", "message": {"content": [{"type": "tool_use", "name": "Read", "input": {"file_path": "..."}}]}}
{"type": "tool_result", "content": "..."}
{"type": "assistant", "message": {"usage": {"input_tokens": N, "output_tokens": M}}}
```

### Agent sub-session logs

**Location:** `~/.claude/projects/<ralph_runtime_dir>/<session_id>/<agent_id>.jsonl`

Same JSONL format. The `agent_id` appears in the parent session's tool_result for Task tool calls.

## Architecture

```
ralph_optimizer.py
├── LogParser          # Parse ralph logs, session logs, agent logs
│   ├── parse_ralph_log(path) -> list[Iteration]
│   ├── parse_session(path) -> Session
│   └── parse_agent(path) -> AgentSession
├── CostAnalyzer       # Token counting and cost estimation
│   ├── estimate_cost(session) -> CostBreakdown
│   └── find_expensive_patterns(sessions) -> list[Pattern]
├── PatternDetector    # Find repeated/wasteful behaviors
│   ├── find_redundant_reads(session) -> list[RedundantRead]
│   ├── find_late_test_runs(session) -> list[LateTest]
│   └── find_agent_overhead(session) -> list[AgentOverhead]
└── Reporter           # Generate reports
    ├── summary_report(sessions) -> str
    └── detailed_report(session) -> str
```

## Data Models

```python
@dataclass
class ToolCall:
    name: str           # "Read", "Edit", "Bash", "Task", etc.
    input: dict         # Tool input parameters
    output_tokens: int  # Approximate tokens in result
    timestamp: str      # When it was called

@dataclass
class Iteration:
    number: int
    session_id: str
    timestamp: str
    result: str         # Success/error summary
    is_error: bool

@dataclass
class Session:
    session_id: str
    tool_calls: list[ToolCall]
    total_input_tokens: int
    total_output_tokens: int
    agents: list[AgentSession]

@dataclass
class AgentSession:
    agent_id: str
    agent_type: str     # "Explore", "general-purpose", etc.
    tool_calls: list[ToolCall]
    total_input_tokens: int
    total_output_tokens: int

@dataclass
class CostBreakdown:
    input_tokens: int
    output_tokens: int
    estimated_cost_usd: float
    by_tool: dict[str, int]  # token count per tool type

@dataclass
class RedundantRead:
    file_path: str
    read_count: int
    first_read_index: int
    total_wasted_tokens: int

@dataclass
class Pattern:
    name: str
    description: str
    occurrences: int
    estimated_waste_tokens: int
    suggestion: str
```

## Analysis Patterns to Detect

### 1. Redundant File Reads
Files read multiple times in the same session without intervening edits.

**Detection:** Track (file_path, content_hash) pairs. If same file read twice with no Edit/Write between, flag it.

**Suggestion:** "Pre-load this file content into the prompt" or "Use a subagent to read once and summarize."

### 2. Large File Full Reads
Reading large files when only a small section is needed.

**Detection:** Read calls without offset/limit on files > 500 lines.

**Suggestion:** "Use offset/limit parameters" or "Use Grep to find relevant sections first."

### 3. Late Test Execution
Tests run only at the end of a session, after many edits. Earlier test runs could catch issues sooner.

**Detection:** Count edits before first Bash test command.

**Suggestion:** "Run tests after every N edits" or "Run relevant test file immediately after editing."

### 4. Agent Overhead
Subagents launched for tasks that could be done with direct tool calls.

**Detection:** Agent sessions with < 3 tool calls (could have been done directly).

**Suggestion:** "Use direct Grep/Read instead of launching an Explore agent for simple lookups."

### 5. Repeated Search Patterns
Same or very similar Grep/Glob patterns run multiple times.

**Detection:** Fuzzy match on search patterns within a session.

**Suggestion:** "Cache search results" or "Broaden the initial search."

### 6. Failed-Then-Retry Loops
Tool calls that fail and are retried with minor modifications.

**Detection:** Sequential tool calls to same tool with similar inputs where first returns error.

**Suggestion:** Document the correct invocation in LEARNINGS.md or AGENTS.md.

## CLI Interface

```bash
# Analyze most recent ralph log
python ralph_optimizer.py

# Analyze specific log file
python ralph_optimizer.py ~/.ralph/logs/ralph-abc12345.log

# Analyze last N iterations
python ralph_optimizer.py --last 5

# Show detailed per-session breakdown
python ralph_optimizer.py --detailed

# Output as JSON for programmatic use
python ralph_optimizer.py --json
```

## Output Format

### Summary Report (default)

```
Ralph Optimizer Report
======================
Log: ~/.ralph/logs/ralph-abc12345.log
Sessions analyzed: 5
Total estimated cost: $X.XX

Top Expensive Patterns:
  1. Redundant reads of PROGRESS.md (8 times across 5 sessions) - ~12K wasted tokens
     → Pre-compute a summary and include in prompt
  2. Full reads of upload.astro (3200 lines × 4 reads) - ~50K wasted tokens
     → Use targeted Grep + Read with offset/limit
  3. Explore agents for simple lookups (12 agents, avg 2.1 tool calls) - ~30K tokens overhead
     → Use direct Grep/Read for files with known paths

Cost by Tool Type:
  Read:   45% (120K tokens)
  Task:   25% (67K tokens)
  Bash:   15% (40K tokens)
  Edit:   10% (27K tokens)
  Other:   5% (13K tokens)

Recommendations:
  1. Add PROGRESS.md summary to RALPH.md prompt (saves ~12K tokens/iteration)
  2. Index large files in specs/ so Ralph knows where to look (saves ~50K tokens)
  3. Set minimum tool-call threshold for agent launches (saves ~30K tokens)
```

## Research References

The feedback mentions checking:
- **Geoff Huntley** - Known for AI coding agent optimization work
- **Steve Yegge** - Has written about AI coding assistants and token efficiency
- **https://github.com/glittercowboy/get-shit-done** - Interesting patterns for AI agent efficiency

These should be researched when implementing to see if they have specific techniques for:
- Prompt caching strategies
- Context window management
- Agent orchestration patterns
- Pre-computation of frequently accessed data

## Testing

```bash
# Unit tests for log parsing
python -m pytest test_ralph_optimizer.py -v

# Integration test with real log
python ralph_optimizer.py ~/.ralph/logs/ralph-<latest>.log
```

## Files to Create

| File | Purpose |
|------|---------|
| `ralph_optimizer.py` | Main script |
| `test_ralph_optimizer.py` | Unit tests |

## Dependencies

- Python 3.11+ (already available)
- No external packages (use stdlib json, pathlib, dataclasses, argparse)
