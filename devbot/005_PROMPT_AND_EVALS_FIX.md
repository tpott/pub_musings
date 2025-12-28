# Plan: Improve Eval Suite and System Prompt for Tool Call Robustness

## Decisions Made
- **Context depth**: Include last 1-2 messages per session
- **Enforcement**: Use `tool_choice: "any"` at API level (guarantees tool use)
- **Multi-tool eval**: Accept any valid tool from acceptable set

---

## Phase 1: Add tool_choice Support (anthropic_client.py)

### 1.1 Add tool_choice parameter to anthropic_completion

```python
def anthropic_completion(
    messages: list[dict[str, Any]],
    context: dict[str, Any],
    tools: Optional[list[dict]] = None,
    system_prompt: Optional[str] = None,
    tool_choice: Optional[str] = None,  # Add this
) -> CompletionResult:
```

Add to kwargs building (after line 78):
```python
if tools is not None:
    kwargs["tools"] = tools
if tool_choice is not None and tools is not None:
    kwargs["tool_choice"] = {"type": tool_choice}  # "any", "auto", or "tool"
```

**File**: `anthropic_client.py:50-79`

---

## Phase 2: Build Dynamic Session Context (dev_agent.py)

### 2.1 Add build_dynamic_context function

```python
def build_dynamic_context(session_manager: SessionManager) -> str:
    """Build dynamic context including sessions and recent messages."""
    current = session_manager.get_current_session_name()
    sessions = session_manager.list_sessions()

    lines = [f"Current session: {current}"]

    if sessions:
        lines.append("\nActive sessions:")
    for name, info in sessions.items():
        marker = " (current)" if name == current else ""
        hostname = info.get("hostname", "unknown")
        msg_count = info.get("message_count", 0)
        lines.append(f"  {name}{marker}: {hostname} ({msg_count} msgs)")

        # Include last 1-2 messages for context
        recent = session_manager.get_recent_messages(2, name)
        if len(recent) == 0:
            continue
        for msg in recent[-2:]:
            role = msg.get("role", "?")
            content = msg.get("content", "")
            if len(content) > 0:
                lines.append(f"    [{role}]: {content}...")

    return "\n".join(lines)
```

**Location**: Add after TOOLS definition (~line 127)

### 2.2 Update SYSTEM_PROMPT

```python
SYSTEM_PROMPT_TEMPLATE = """You are a developer assistant managing remote Claude sessions.

CRITICAL: You MUST use a tool for every response. NEVER respond with text alone.
- For development questions, coding help, or any unclear request: use proxy_message
- For session management (create/switch/list): use the appropriate session tool
- When in doubt, use proxy_message to forward to the current session

{dynamic_context}

Available tools:
- proxy_message: Forward ANY message to the current session's remote Claude CLI
- get_sessions: List all active sessions with their stats
- switch_session: Switch to a different session by hashtag name
- new_session: Create a new session (optionally specify hostname)
- info: Get detailed info for current or specified session
- get_hosts: List configured remote hosts
- compact_session: Summarize the last N messages into a single context summary

ALWAYS call a tool. Direct text responses are NOT allowed."""
```

**File**: `dev_agent.py:16-29`

### 2.3 Update process_input to use dynamic prompt and tool_choice

In the agent loop (line 161-166):
```python
# Build dynamic system prompt
dynamic_context = build_dynamic_context(self.session_manager)
system_prompt = SYSTEM_PROMPT_TEMPLATE.format(dynamic_context=dynamic_context)

result = anthropic_completion(
    messages=messages,
    context=self.context,
    tools=TOOLS,
    system_prompt=system_prompt,
    tool_choice="any",  # Force tool use
)
```

**File**: `dev_agent.py:156-166`

---

## Phase 3: Update Eval Test Cases (evals/test_cases.py)

### 3.1 Add STANDARD_CASES (proxy_message expected)

```python
STANDARD_CASES = [
    {"input": "How do I write a for loop in Python?", "expected_tool": "proxy_message"},
    {"input": "Explain async/await", "expected_tool": "proxy_message"},
    {"input": "Review this code: def foo(): pass", "expected_tool": "proxy_message"},
    {"input": "What's the difference between let and const?", "expected_tool": "proxy_message"},
    {"input": "Help me debug this error", "expected_tool": "proxy_message"},
    {"input": "Write a function to sort a list", "expected_tool": "proxy_message"},
    {"input": "Can you explain recursion?", "expected_tool": "proxy_message"},
    {"input": "How do I use git rebase?", "expected_tool": "proxy_message"},
]
```

### 3.2 Update SESSION_TOOL_CASES with acceptable_tools

```python
SESSION_TOOL_CASES = [
    # new_session triggers
    {"input": "Create a new session called debug", "acceptable_tools": ["new_session"]},
    {"input": "Start a new session #refactor on devbox", "acceptable_tools": ["new_session"]},
    {"input": "I need a fresh session for this project", "acceptable_tools": ["new_session", "proxy_message"]},

    # switch_session triggers (may need get_sessions first)
    {"input": "Switch to the refactor session", "acceptable_tools": ["switch_session", "get_sessions"]},
    {"input": "Go back to #debug", "acceptable_tools": ["switch_session"]},
    {"input": "Change to my other session", "acceptable_tools": ["switch_session", "get_sessions"]},

    # get_sessions triggers
    {"input": "What sessions do I have?", "acceptable_tools": ["get_sessions"]},
    {"input": "List all my sessions", "acceptable_tools": ["get_sessions"]},

    # get_hosts triggers
    {"input": "What hosts are available?", "acceptable_tools": ["get_hosts"]},
    {"input": "Show me the configured servers", "acceptable_tools": ["get_hosts"]},
]
```

**File**: `evals/test_cases.py`

---

## Phase 4: Update Eval Script (evals/tool_call_eval.py)

### 4.1 Add mock session context for eval

```python
MOCK_SESSION_CONTEXT = """Current session: #default

Active sessions:
  #default (current): devbox (12 msgs)
    [user]: How do I implement a binary search?...
    [assistant]: Here's a binary search implementation...
  #refactor: devbox (5 msgs)
    [user]: Let's refactor the auth module...
    [assistant]: I'll help refactor the authentication...
  #debug: prod (3 msgs)
    [user]: There's a bug in the checkout flow...
    [assistant]: Let me investigate the checkout..."""

def get_eval_system_prompt() -> str:
    """Build system prompt for eval with mock context."""
    from dev_agent import SYSTEM_PROMPT_TEMPLATE
    return SYSTEM_PROMPT_TEMPLATE.format(dynamic_context=MOCK_SESSION_CONTEXT)
```

### 4.2 Add tool_choice="any" to eval API calls

Update `run_single_case`:
```python
result = anthropic_completion(
    messages=messages,
    context=context,
    tools=TOOLS,
    system_prompt=get_eval_system_prompt(),
    tool_choice="any",  # Force tool use
)
```

### 4.3 Add run_standard_eval function

```python
def run_standard_eval(context: dict[str, Any], verbose: bool = False) -> dict[str, Any]:
    """Run standard (non-adversarial) test cases expecting proxy_message."""
    results = []
    failures = []

    print("\nRunning standard cases...")
    for case in STANDARD_CASES:
        if verbose:
            print(f"\n[standard]", end="")

        result = run_single_case(case["input"], context, verbose)
        result["expected_tool"] = case["expected_tool"]
        results.append(result)

        if result["tool_called"] != case["expected_tool"]:
            failures.append(result)

    passed = len(results) - len(failures)
    total = len(results)
    success_rate = (passed / total * 100) if total > 0 else 0

    return {
        "total": total,
        "passed": passed,
        "failed": len(failures),
        "success_rate": success_rate,
        "failures": failures,
        "results": results,
    }
```

### 4.4 Update session eval to use acceptable_tools

```python
def run_session_tool_eval(context: dict[str, Any], verbose: bool = False) -> dict[str, Any]:
    results = []
    failures = []

    print("\nRunning session tool selection cases...")
    for case in SESSION_TOOL_CASES:
        if verbose:
            print(f"\n[session]", end="")

        result = run_single_case(case["input"], context, verbose)
        result["acceptable_tools"] = case["acceptable_tools"]
        results.append(result)

        # Check if called tool is in acceptable set
        if result["tool_called"] not in case["acceptable_tools"]:
            failures.append(result)

    # ... rest of function
```

### 4.5 Update report to include standard cases

Add STANDARD_CASES section to `print_report()`.

**File**: `evals/tool_call_eval.py`

---

## Implementation Order

1. `anthropic_client.py` - Add tool_choice parameter
2. `dev_agent.py` - Add build_dynamic_context() function
3. `dev_agent.py` - Update SYSTEM_PROMPT to template with strict wording
4. `dev_agent.py` - Wire dynamic prompt + tool_choice into process_input
5. `evals/test_cases.py` - Add STANDARD_CASES
6. `evals/test_cases.py` - Update SESSION_TOOL_CASES with acceptable_tools
7. `evals/tool_call_eval.py` - Add mock context and eval system prompt
8. `evals/tool_call_eval.py` - Add tool_choice to API calls
9. `evals/tool_call_eval.py` - Add run_standard_eval()
10. `evals/tool_call_eval.py` - Update session eval for acceptable_tools
11. `evals/tool_call_eval.py` - Update print_report()

---

## Files to Modify

| File | Lines | Changes |
|------|-------|---------|
| `anthropic_client.py` | 50-79 | Add tool_choice parameter |
| `dev_agent.py` | 16-29 | SYSTEM_PROMPT → SYSTEM_PROMPT_TEMPLATE |
| `dev_agent.py` | ~127 | Add build_dynamic_context() |
| `dev_agent.py` | 156-166 | Use dynamic prompt + tool_choice |
| `evals/test_cases.py` | all | Add STANDARD_CASES, update SESSION_TOOL_CASES |
| `evals/tool_call_eval.py` | all | Mock context, tool_choice, new eval function |
