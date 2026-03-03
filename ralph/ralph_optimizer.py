#!/usr/bin/env python3
"""
Ralph Optimizer: Analyze Ralph session logs to identify expensive behaviors,
token waste, and optimization opportunities.

See specs/ralph-optimizer.md for the full specification.
"""

import argparse
import json
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path


# --- Data Models ---

@dataclass
class ToolCall:
    name: str
    input: dict = field(default_factory=dict)
    output_tokens: int = 0
    index: int = 0  # position in session


@dataclass
class Iteration:
    number: int
    total: int
    session_id: str
    timestamp: str
    is_error: bool = False
    result: str = ""


@dataclass
class AgentSession:
    agent_id: str
    agent_type: str
    tool_calls: list[ToolCall] = field(default_factory=list)
    total_input_tokens: int = 0
    total_output_tokens: int = 0


@dataclass
class Session:
    session_id: str
    tool_calls: list[ToolCall] = field(default_factory=list)
    total_input_tokens: int = 0
    total_output_tokens: int = 0
    agents: list[AgentSession] = field(default_factory=list)


@dataclass
class CostBreakdown:
    input_tokens: int = 0
    output_tokens: int = 0
    estimated_cost_usd: float = 0.0
    by_tool: dict[str, int] = field(default_factory=dict)


@dataclass
class RedundantRead:
    file_path: str
    read_count: int
    first_read_index: int
    total_wasted_tokens: int


@dataclass
class LargeFileRead:
    file_path: str
    lines_read: int
    read_count: int


@dataclass
class LateTestRun:
    edits_before_test: int
    first_test_index: int
    total_tool_calls: int


@dataclass
class AgentOverhead:
    agent_id: str
    agent_type: str
    tool_call_count: int


@dataclass
class Pattern:
    name: str
    description: str
    occurrences: int
    estimated_waste_tokens: int
    suggestion: str


# --- Pricing ---

# Claude Opus 4.5 pricing (per token)
OPUS_INPUT_PRICE = 15.0 / 1_000_000   # $15 per 1M input tokens
OPUS_OUTPUT_PRICE = 75.0 / 1_000_000  # $75 per 1M output tokens
HAIKU_INPUT_PRICE = 0.80 / 1_000_000  # $0.80 per 1M input tokens
HAIKU_OUTPUT_PRICE = 4.0 / 1_000_000  # $4 per 1M output tokens


# --- LogParser ---

class LogParser:
    """Parse ralph logs, session logs, and agent logs."""

    # Default location for Claude Code session logs
    CLAUDE_PROJECTS_DIR = Path.home() / ".claude" / "projects"
    RALPH_LOGS_DIR = Path.home() / ".ralph" / "logs"

    def __init__(self):
        self.runtime_dir: str | None = None

    def _cwd_to_runtime_dir(self, cwd: str) -> str:
        """Convert a cwd path to Claude Code's runtime directory format.

        e.g. /home/trevor/pub_musings/peekaboo -> -home-trevor-pub-musings-peekaboo
        """
        return cwd.replace("/", "-").replace("_", "-")

    def parse_ralph_log(self, path: Path) -> list[Iteration]:
        """Parse a ralph log file and extract iterations."""
        iterations: list[Iteration] = []
        current_session_id = ""
        current_timestamp = ""
        current_number = 0
        current_total = 0

        iter_pattern = re.compile(
            r"=== Iteration (\d+)/(\d+) === (.+)"
        )

        with open(path, "r", errors="replace") as f:
            for line in f:
                line = line.rstrip("\n")

                # Check for iteration header
                match = iter_pattern.match(line)
                if match:
                    current_number = int(match.group(1))
                    current_total = int(match.group(2))
                    current_timestamp = match.group(3)
                    current_session_id = ""
                    continue

                # Try to parse as JSON
                if not line.startswith("{"):
                    # Check for "Result:" lines
                    if line.startswith("Result:"):
                        if iterations:
                            iterations[-1].result = line[7:].strip()
                    continue

                try:
                    data = json.loads(line)
                except json.JSONDecodeError:
                    continue

                # Extract session_id and cwd from init message
                if data.get("type") == "system" and data.get("subtype") == "init":
                    current_session_id = data.get("session_id", "")
                    # Capture cwd for runtime directory (use first one found)
                    if self.runtime_dir is None:
                        cwd = data.get("cwd", "")
                        if cwd:
                            self.runtime_dir = self._cwd_to_runtime_dir(cwd)
                    iterations.append(Iteration(
                        number=current_number,
                        total=current_total,
                        session_id=current_session_id,
                        timestamp=current_timestamp,
                    ))

                # Check for error result
                if data.get("type") == "result" and data.get("is_error"):
                    if iterations:
                        iterations[-1].is_error = True
                        iterations[-1].result = data.get("result", "")

        return iterations

    def parse_session(self, session_id: str) -> Session:
        """Parse a Claude Code session log by session ID."""
        session = Session(session_id=session_id)

        if not self.runtime_dir:
            return session

        session_dir = self.CLAUDE_PROJECTS_DIR / self.runtime_dir
        session_file = session_dir / f"{session_id}.jsonl"

        if not session_file.exists():
            return session

        tool_index = 0
        with open(session_file, "r", errors="replace") as f:
            for line in f:
                line = line.rstrip("\n")
                if not line.startswith("{"):
                    continue
                try:
                    data = json.loads(line)
                except json.JSONDecodeError:
                    continue

                if data.get("type") == "assistant":
                    msg = data.get("message", {})
                    usage = msg.get("usage", {})

                    # Accumulate tokens
                    input_tokens = usage.get("input_tokens", 0)
                    cache_creation = usage.get("cache_creation_input_tokens", 0)
                    cache_read = usage.get("cache_read_input_tokens", 0)
                    output_tokens = usage.get("output_tokens", 0)
                    session.total_input_tokens += input_tokens + cache_creation + cache_read
                    session.total_output_tokens += output_tokens

                    # Extract tool calls
                    content = msg.get("content", [])
                    for item in content:
                        if isinstance(item, dict) and item.get("type") == "tool_use":
                            tc = ToolCall(
                                name=item.get("name", ""),
                                input=item.get("input", {}),
                                output_tokens=output_tokens,
                                index=tool_index,
                            )
                            # Only count top-level tool calls (no parent_tool_use_id)
                            parent = data.get("parent_tool_use_id")
                            if parent is None:
                                session.tool_calls.append(tc)
                            tool_index += 1

        # Parse agent sub-sessions
        agent_dir = session_dir / session_id
        if agent_dir.is_dir():
            for agent_file in sorted(agent_dir.glob("*.jsonl")):
                agent_id = agent_file.stem
                agent_session = self._parse_agent_file(agent_file, agent_id)
                session.agents.append(agent_session)

        return session

    def _parse_agent_file(self, path: Path, agent_id: str) -> AgentSession:
        """Parse an agent sub-session log file."""
        agent = AgentSession(agent_id=agent_id, agent_type="unknown")
        tool_index = 0

        with open(path, "r", errors="replace") as f:
            for line in f:
                line = line.rstrip("\n")
                if not line.startswith("{"):
                    continue
                try:
                    data = json.loads(line)
                except json.JSONDecodeError:
                    continue

                if data.get("type") == "system" and data.get("subtype") == "init":
                    # Try to detect agent type from model
                    model = data.get("model", "")
                    if "haiku" in model:
                        agent.agent_type = "Explore/Haiku"
                    elif "sonnet" in model:
                        agent.agent_type = "Sonnet"
                    elif "opus" in model:
                        agent.agent_type = "Opus"

                if data.get("type") == "assistant":
                    msg = data.get("message", {})
                    usage = msg.get("usage", {})

                    input_tokens = usage.get("input_tokens", 0)
                    cache_creation = usage.get("cache_creation_input_tokens", 0)
                    cache_read = usage.get("cache_read_input_tokens", 0)
                    output_tokens = usage.get("output_tokens", 0)
                    agent.total_input_tokens += input_tokens + cache_creation + cache_read
                    agent.total_output_tokens += output_tokens

                    content = msg.get("content", [])
                    for item in content:
                        if isinstance(item, dict) and item.get("type") == "tool_use":
                            tc = ToolCall(
                                name=item.get("name", ""),
                                input=item.get("input", {}),
                                output_tokens=output_tokens,
                                index=tool_index,
                            )
                            agent.tool_calls.append(tc)
                            tool_index += 1

        return agent

    def find_latest_ralph_log(self) -> Path | None:
        """Find the most recently modified ralph log file."""
        if not self.RALPH_LOGS_DIR.exists():
            return None
        logs = sorted(
            self.RALPH_LOGS_DIR.glob("ralph-*.log"),
            key=lambda p: p.stat().st_mtime,
            reverse=True,
        )
        return logs[0] if logs else None


# --- CostAnalyzer ---

class CostAnalyzer:
    """Token counting and cost estimation."""

    def estimate_cost(self, session: Session) -> CostBreakdown:
        """Estimate cost for a session."""
        breakdown = CostBreakdown(
            input_tokens=session.total_input_tokens,
            output_tokens=session.total_output_tokens,
        )

        # Main session (Opus pricing)
        cost = (
            session.total_input_tokens * OPUS_INPUT_PRICE
            + session.total_output_tokens * OPUS_OUTPUT_PRICE
        )

        # Agent sessions (typically Haiku for Explore agents)
        for agent in session.agents:
            if "Haiku" in agent.agent_type or "Explore" in agent.agent_type:
                cost += (
                    agent.total_input_tokens * HAIKU_INPUT_PRICE
                    + agent.total_output_tokens * HAIKU_OUTPUT_PRICE
                )
            else:
                cost += (
                    agent.total_input_tokens * OPUS_INPUT_PRICE
                    + agent.total_output_tokens * OPUS_OUTPUT_PRICE
                )
            breakdown.input_tokens += agent.total_input_tokens
            breakdown.output_tokens += agent.total_output_tokens

        breakdown.estimated_cost_usd = cost

        # Count tokens by tool type
        for tc in session.tool_calls:
            breakdown.by_tool[tc.name] = breakdown.by_tool.get(tc.name, 0) + 1
        for agent in session.agents:
            for tc in agent.tool_calls:
                tool_name = f"Agent:{tc.name}"
                breakdown.by_tool[tool_name] = breakdown.by_tool.get(tool_name, 0) + 1

        return breakdown


# --- PatternDetector ---

class PatternDetector:
    """Find repeated/wasteful behaviors in sessions."""

    def find_redundant_reads(self, session: Session) -> list[RedundantRead]:
        """Find files read multiple times without intervening edits."""
        # Track read counts and edit positions
        reads: dict[str, list[int]] = {}  # file_path -> [indices]
        edits: set[str] = set()  # files edited at any point
        edit_indices: dict[str, list[int]] = {}  # file_path -> [edit indices]

        for tc in session.tool_calls:
            if tc.name in ("Read",):
                fp = tc.input.get("file_path", "")
                if fp:
                    reads.setdefault(fp, []).append(tc.index)
            elif tc.name in ("Edit", "Write"):
                fp = tc.input.get("file_path", "")
                if fp:
                    edits.add(fp)
                    edit_indices.setdefault(fp, []).append(tc.index)

        redundant = []
        for fp, indices in reads.items():
            if len(indices) <= 1:
                continue

            # Count reads without intervening edits
            wasted_reads = 0
            edit_idxs = edit_indices.get(fp, [])
            for i in range(1, len(indices)):
                # Check if there was an edit between this read and the previous one
                prev_idx = indices[i - 1]
                curr_idx = indices[i]
                has_intervening_edit = any(
                    prev_idx < e_idx < curr_idx for e_idx in edit_idxs
                )
                if not has_intervening_edit:
                    wasted_reads += 1

            if wasted_reads > 0:
                # Estimate ~500 tokens per wasted read (rough average)
                redundant.append(RedundantRead(
                    file_path=fp,
                    read_count=len(indices),
                    first_read_index=indices[0],
                    total_wasted_tokens=wasted_reads * 500,
                ))

        return sorted(redundant, key=lambda r: r.total_wasted_tokens, reverse=True)

    def find_large_file_reads(self, session: Session) -> list[LargeFileRead]:
        """Find reads of large files without offset/limit."""
        large_reads: dict[str, int] = {}  # file_path -> count

        for tc in session.tool_calls:
            if tc.name != "Read":
                continue
            inp = tc.input
            fp = inp.get("file_path", "")
            has_offset = "offset" in inp
            has_limit = "limit" in inp
            if fp and not has_offset and not has_limit:
                large_reads[fp] = large_reads.get(fp, 0) + 1

        # We can't know actual line count without reading the file, so
        # just report files read multiple times without offset/limit
        results = []
        for fp, count in large_reads.items():
            if count > 1:
                results.append(LargeFileRead(
                    file_path=fp,
                    lines_read=0,  # unknown
                    read_count=count,
                ))

        return sorted(results, key=lambda r: r.read_count, reverse=True)

    def find_late_test_runs(self, session: Session) -> list[LateTestRun]:
        """Find sessions where tests run late after many edits."""
        edit_count = 0
        first_test_index = -1

        test_patterns = [
            "go test", "npm test", "vitest", "verify-all",
            "test-backend", "test-frontend", "test-e2e",
        ]

        for i, tc in enumerate(session.tool_calls):
            if tc.name in ("Edit", "Write"):
                edit_count += 1
            elif tc.name == "Bash":
                cmd = tc.input.get("command", "")
                is_test = any(p in cmd for p in test_patterns)
                if is_test and first_test_index == -1:
                    first_test_index = i
                    break

        if first_test_index == -1:
            # No test run found - but only report if edits were made
            if edit_count > 0:
                return [LateTestRun(
                    edits_before_test=edit_count,
                    first_test_index=-1,
                    total_tool_calls=len(session.tool_calls),
                )]
            return []

        if edit_count >= 5:
            return [LateTestRun(
                edits_before_test=edit_count,
                first_test_index=first_test_index,
                total_tool_calls=len(session.tool_calls),
            )]

        return []

    def find_agent_overhead(self, session: Session) -> list[AgentOverhead]:
        """Find agents with very few tool calls (could be done directly)."""
        overhead = []
        for agent in session.agents:
            if len(agent.tool_calls) < 3:
                overhead.append(AgentOverhead(
                    agent_id=agent.agent_id,
                    agent_type=agent.agent_type,
                    tool_call_count=len(agent.tool_calls),
                ))
        return overhead

    def detect_all_patterns(self, sessions: list[Session]) -> list[Pattern]:
        """Run all pattern detectors across multiple sessions."""
        patterns: list[Pattern] = []

        total_redundant_reads = 0
        total_redundant_tokens = 0
        total_late_tests = 0
        total_overhead_agents = 0
        top_redundant_files: dict[str, int] = {}

        for session in sessions:
            # Redundant reads
            redundant = self.find_redundant_reads(session)
            for r in redundant:
                total_redundant_reads += r.read_count - 1
                total_redundant_tokens += r.total_wasted_tokens
                top_redundant_files[r.file_path] = (
                    top_redundant_files.get(r.file_path, 0) + r.read_count
                )

            # Late tests
            late = self.find_late_test_runs(session)
            total_late_tests += len(late)

            # Agent overhead
            overhead = self.find_agent_overhead(session)
            total_overhead_agents += len(overhead)

        if total_redundant_reads > 0:
            top_files = sorted(
                top_redundant_files.items(), key=lambda x: x[1], reverse=True
            )[:3]
            file_list = ", ".join(
                f"{Path(fp).name} ({cnt}x)" for fp, cnt in top_files
            )
            patterns.append(Pattern(
                name="Redundant File Reads",
                description=f"{total_redundant_reads} redundant reads across {len(sessions)} sessions. Top: {file_list}",
                occurrences=total_redundant_reads,
                estimated_waste_tokens=total_redundant_tokens,
                suggestion="Pre-load frequently read files into prompt or use subagent summaries",
            ))

        if total_late_tests > 0:
            patterns.append(Pattern(
                name="Late Test Execution",
                description=f"{total_late_tests} sessions ran tests only after 5+ edits",
                occurrences=total_late_tests,
                estimated_waste_tokens=total_late_tests * 5000,
                suggestion="Run tests after every 2-3 edits to catch issues sooner",
            ))

        if total_overhead_agents > 0:
            patterns.append(Pattern(
                name="Low-Value Agent Launches",
                description=f"{total_overhead_agents} agents with <3 tool calls (could use direct tools)",
                occurrences=total_overhead_agents,
                estimated_waste_tokens=total_overhead_agents * 2000,
                suggestion="Use direct Grep/Read instead of launching agents for simple lookups",
            ))

        return sorted(patterns, key=lambda p: p.estimated_waste_tokens, reverse=True)


# --- Reporter ---

class Reporter:
    """Generate reports from analysis results."""

    def summary_report(
        self,
        log_path: Path,
        iterations: list[Iteration],
        sessions: list[Session],
        costs: list[CostBreakdown],
        patterns: list[Pattern],
    ) -> str:
        """Generate a summary report."""
        lines = [
            "Ralph Optimizer Report",
            "=" * 50,
            f"Log: {log_path}",
            f"Iterations analyzed: {len(iterations)}",
            f"Sessions parsed: {len(sessions)}",
        ]

        # Total cost
        total_cost = sum(c.estimated_cost_usd for c in costs)
        total_input = sum(c.input_tokens for c in costs)
        total_output = sum(c.output_tokens for c in costs)
        lines.append(f"Total estimated cost: ${total_cost:.2f}")
        lines.append(f"Total tokens: {_fmt_tokens(total_input)} input, {_fmt_tokens(total_output)} output")
        lines.append("")

        # Error rate
        error_count = sum(1 for it in iterations if it.is_error)
        if error_count:
            lines.append(f"Error iterations: {error_count}/{len(iterations)}")
            lines.append("")

        # Cost per session
        if costs:
            lines.append("Cost Per Session:")
            for i, (iteration, cost) in enumerate(zip(iterations, costs)):
                session_total = cost.input_tokens + cost.output_tokens
                err = " [ERROR]" if iteration.is_error else ""
                lines.append(
                    f"  {i + 1}. Session {iteration.session_id[:8]}... "
                    f"${cost.estimated_cost_usd:.2f} "
                    f"({_fmt_tokens(session_total)} tokens){err}"
                )
            lines.append("")

        # Tool call distribution
        merged_tools: dict[str, int] = {}
        for cost in costs:
            for tool, count in cost.by_tool.items():
                merged_tools[tool] = merged_tools.get(tool, 0) + count

        if merged_tools:
            total_calls = sum(merged_tools.values())
            lines.append(f"Tool Call Distribution ({total_calls} total):")
            sorted_tools = sorted(
                merged_tools.items(), key=lambda x: x[1], reverse=True
            )
            for tool, count in sorted_tools[:10]:
                pct = count / total_calls * 100
                lines.append(f"  {tool:20s} {count:4d} ({pct:.0f}%)")
            lines.append("")

        # Patterns
        if patterns:
            lines.append("Detected Patterns:")
            for i, p in enumerate(patterns, 1):
                lines.append(f"  {i}. {p.name} ({p.occurrences} occurrences, ~{_fmt_tokens(p.estimated_waste_tokens)} wasted)")
                lines.append(f"     {p.description}")
                lines.append(f"     -> {p.suggestion}")
            lines.append("")

        # Recommendations
        lines.append("Recommendations:")
        if not patterns:
            lines.append("  No significant waste patterns detected.")
        else:
            for i, p in enumerate(patterns, 1):
                lines.append(f"  {i}. {p.suggestion} (saves ~{_fmt_tokens(p.estimated_waste_tokens)} tokens)")
        lines.append("")

        return "\n".join(lines)

    def detailed_report(
        self,
        session: Session,
        cost: CostBreakdown,
    ) -> str:
        """Generate a detailed report for a single session."""
        lines = [
            f"Session: {session.session_id}",
            f"  Input tokens:  {_fmt_tokens(session.total_input_tokens)}",
            f"  Output tokens: {_fmt_tokens(session.total_output_tokens)}",
            f"  Estimated cost: ${cost.estimated_cost_usd:.2f}",
            f"  Tool calls: {len(session.tool_calls)}",
            f"  Agents: {len(session.agents)}",
        ]

        if session.tool_calls:
            lines.append("  Tool call sequence:")
            for tc in session.tool_calls[:50]:  # Cap at 50 for readability
                inp_summary = _summarize_input(tc)
                lines.append(f"    [{tc.index:3d}] {tc.name}: {inp_summary}")
            if len(session.tool_calls) > 50:
                lines.append(f"    ... and {len(session.tool_calls) - 50} more")

        if session.agents:
            lines.append("  Agent sub-sessions:")
            for agent in session.agents:
                lines.append(
                    f"    {agent.agent_id[:8]}... ({agent.agent_type}) - "
                    f"{len(agent.tool_calls)} tool calls, "
                    f"{_fmt_tokens(agent.total_input_tokens + agent.total_output_tokens)} tokens"
                )

        return "\n".join(lines)

    def json_report(
        self,
        log_path: Path,
        iterations: list[Iteration],
        sessions: list[Session],
        costs: list[CostBreakdown],
        patterns: list[Pattern],
    ) -> str:
        """Generate a JSON report."""
        report = {
            "log_path": str(log_path),
            "iterations": len(iterations),
            "sessions": len(sessions),
            "total_cost_usd": sum(c.estimated_cost_usd for c in costs),
            "total_input_tokens": sum(c.input_tokens for c in costs),
            "total_output_tokens": sum(c.output_tokens for c in costs),
            "error_count": sum(1 for it in iterations if it.is_error),
            "per_session": [
                {
                    "session_id": it.session_id,
                    "iteration": it.number,
                    "is_error": it.is_error,
                    "cost_usd": cost.estimated_cost_usd,
                    "input_tokens": cost.input_tokens,
                    "output_tokens": cost.output_tokens,
                    "tool_calls": sum(cost.by_tool.values()),
                }
                for it, cost in zip(iterations, costs)
            ],
            "patterns": [
                {
                    "name": p.name,
                    "description": p.description,
                    "occurrences": p.occurrences,
                    "estimated_waste_tokens": p.estimated_waste_tokens,
                    "suggestion": p.suggestion,
                }
                for p in patterns
            ],
        }
        return json.dumps(report, indent=2)


# --- Helpers ---

def _fmt_tokens(n: int) -> str:
    """Format token count with K/M suffix."""
    if n >= 1_000_000:
        return f"{n / 1_000_000:.1f}M"
    if n >= 1_000:
        return f"{n / 1_000:.1f}K"
    return str(n)


def _summarize_input(tc: ToolCall) -> str:
    """Create a short summary of tool call input."""
    inp = tc.input
    if tc.name == "Read":
        fp = inp.get("file_path", "")
        return Path(fp).name if fp else "(no path)"
    if tc.name in ("Edit", "Write"):
        fp = inp.get("file_path", "")
        return Path(fp).name if fp else "(no path)"
    if tc.name == "Bash":
        cmd = inp.get("command", "")
        return cmd[:60] + "..." if len(cmd) > 60 else cmd
    if tc.name == "Grep":
        pattern = inp.get("pattern", "")
        return f'"{pattern}"'
    if tc.name == "Glob":
        pattern = inp.get("pattern", "")
        return pattern
    if tc.name == "Task":
        desc = inp.get("description", "")
        return desc
    if tc.name == "TodoWrite":
        return "(todo update)"
    return str(inp)[:60]


# --- Main ---

def main() -> None:
    parser = argparse.ArgumentParser(
        description="Analyze Ralph session logs for optimization opportunities"
    )
    parser.add_argument(
        "log_file",
        nargs="?",
        type=Path,
        help="Ralph log file to analyze (default: most recent)",
    )
    parser.add_argument(
        "--last",
        type=int,
        default=0,
        help="Only analyze last N iterations",
    )
    parser.add_argument(
        "--detailed",
        action="store_true",
        help="Show detailed per-session breakdown",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        dest="json_output",
        help="Output as JSON",
    )
    args = parser.parse_args()

    log_parser = LogParser()

    # Find log file
    log_path = args.log_file
    if log_path is None:
        log_path = log_parser.find_latest_ralph_log()
        if log_path is None:
            print("Error: No ralph log files found in ~/.ralph/logs/", file=sys.stderr)
            sys.exit(1)

    if not log_path.exists():
        print(f"Error: Log file not found: {log_path}", file=sys.stderr)
        sys.exit(1)

    # Parse iterations
    iterations = log_parser.parse_ralph_log(log_path)
    if not iterations:
        print(f"Error: No iterations found in {log_path}", file=sys.stderr)
        sys.exit(1)

    # Apply --last filter
    if args.last > 0:
        iterations = iterations[-args.last:]

    # Parse sessions
    cost_analyzer = CostAnalyzer()
    sessions: list[Session] = []
    costs: list[CostBreakdown] = []

    for iteration in iterations:
        if not iteration.session_id:
            continue
        session = log_parser.parse_session(iteration.session_id)
        sessions.append(session)
        costs.append(cost_analyzer.estimate_cost(session))

    # Detect patterns
    detector = PatternDetector()
    patterns = detector.detect_all_patterns(sessions)

    # Generate report
    reporter = Reporter()

    if args.json_output:
        print(reporter.json_report(log_path, iterations, sessions, costs, patterns))
    else:
        print(reporter.summary_report(log_path, iterations, sessions, costs, patterns))

        if args.detailed:
            print()
            print("=" * 50)
            print("DETAILED SESSION BREAKDOWN")
            print("=" * 50)
            for session, cost in zip(sessions, costs):
                print()
                print(reporter.detailed_report(session, cost))


if __name__ == "__main__":
    main()
