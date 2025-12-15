# dev_agent.py Implementation Plan

## Overview

Implement a local Claude-powered agent (`dev_agent.py`) that manages SSH proxy sessions to remote Claude CLI instances. **Fully replaces the existing OpenAI/ChatGPT path** - all messages go through DevAgent via Anthropic API.

## Architecture

```
User Input --> loop.py --> DevAgent (Anthropic Claude)
                              |
                              |--> Tool: proxy_message --> SSH --> Remote Claude CLI
                              |--> Tool: get_sessions/switch_session/new_session/info
                              |--> Tool: get_hosts/compact_session
                              |
                              v
                         anthropic_client.py (token tracking, cost calculation)
```

## File Structure

```
devbot/
├── loop.py              # MODIFY: Replace ChatGPT with DevAgent
├── dev_agent.py         # NEW: Main agent with tool use
├── anthropic_client.py  # NEW: Anthropic SDK wrapper (like chatgpt.py)
├── ssh_proxy.py         # NEW: Secure SSH execution
├── session_manager.py   # NEW: Session state + persistence
├── serve_config.py      # MODIFY: Add hosts config
├── chatgpt.py           # KEEP: May remove later, unused
├── data/
│   ├── config.json      # MODIFY: Add anthropic_api_key + hosts
│   ├── sessions.json    # NEW: Session metadata
│   └── history/         # NEW: Per-session JSONL history
│       ├── default.jsonl
│       └── {hashtag}.jsonl
```

## Implementation Steps

### Phase 1: Configuration Updates

**serve_config.py** - Add new config fields:
```python
# Expected config structure:
{
  "anthropic_api_key": "sk-ant-...",
  "hosts": {
    "devbox": {"hostname": "devbox.example.com", "user": "trevor"},
    "prod": {"hostname": "prod.example.com", "user": "deploy"}
  },
  "default_host": "devbox"
}
```

### Phase 2: Anthropic Client (anthropic_client.py)

Create wrapper similar to `chatgpt.py` for consistency and token tracking:

```python
# anthropic_client.py
from anthropic import Anthropic
from typing import TypedDict, List, Dict, Any

class CompletionResult(TypedDict):
    content: str
    usage: Dict[str, int]  # input_tokens, output_tokens
    cost: Dict[str, float]  # input_cost, output_cost, total

# Anthropic pricing (as of Dec 2024)
MODEL_PRICING = {
    "claude-sonnet-4-5-20250514": {"input": 3.00, "output": 15.00},  # per 1M tokens
    "claude-3-5-haiku-20241022": {"input": 0.80, "output": 4.00},
}

async def anthropic_completion(
    messages: List[Dict[str, Any]],
    context: Dict[str, Any],
    tools: List[Dict] = None
) -> CompletionResult:
    """
    Call Anthropic API with token tracking.
    Returns structured result with content, usage, and cost.
    """
    client = Anthropic(api_key=context["anthropic_api_key"])
    model = context.get("model", "claude-sonnet-4-5-20250514")

    response = client.messages.create(
        model=model,
        max_tokens=context.get("max_tokens", 4096),
        system=context.get("system_prompt", ""),
        messages=messages,
        tools=tools or []
    )

    # Calculate cost
    pricing = MODEL_PRICING.get(model, {"input": 3.00, "output": 15.00})
    input_cost = (response.usage.input_tokens / 1_000_000) * pricing["input"]
    output_cost = (response.usage.output_tokens / 1_000_000) * pricing["output"]

    return {
        "content": _extract_text(response),
        "response": response,  # Full response for tool_use handling
        "usage": {
            "input_tokens": response.usage.input_tokens,
            "output_tokens": response.usage.output_tokens,
        },
        "cost": {
            "input": input_cost,
            "output": output_cost,
            "total": input_cost + output_cost
        }
    }
```

**DevAgent uses this** for all Claude API calls, enabling:
- Consistent token/cost tracking across the application
- Verbose logging (when `context["verbose"] > 0`)
- Easy model switching via context

### Phase 3: Session Manager (session_manager.py)

Create `SessionManager` class with:
- `sessions.json` persistence (file locking for safety)
- JSONL message history per session in `data/history/`
- Methods: `create_session()`, `switch_session()`, `get_current_session()`, `update_session()`, `add_message()`, `list_sessions()`, `get_session_info()`, `get_recent_messages()`, `compact_messages()`

**Sessions.json schema** (metadata only):
```json
{
  "sessions": {
    "#default": {
      "hostname": "devbox",
      "last_message_id": "msg_abc123",
      "session_id": "uuid",
      "created_at": 1702598400.0,
      "updated_at": 1702684800.0,
      "message_count": 42
    }
  },
  "current_session": "#default"
}
```

**History JSONL format** (`data/history/default.jsonl`):
```jsonl
{"role": "user", "content": "How does auth work?", "timestamp": 1702598400.0, "proxied": true}
{"role": "assistant", "content": "The auth system uses JWT tokens...", "timestamp": 1702598405.0, "remote_message_id": "msg_abc123"}
{"role": "user", "content": "Can you show me the middleware?", "timestamp": 1702598500.0, "proxied": true}
{"role": "assistant", "content": "Here's the auth middleware...", "timestamp": 1702598510.0, "remote_message_id": "msg_def456"}
{"role": "system", "content": "[Compacted context summary]: User explored auth system including JWT tokens and middleware implementation.", "timestamp": 1702600000.0, "compacted_count": 4}
{"role": "user", "content": "Now add rate limiting", "timestamp": 1702600100.0, "proxied": true}
```

**Key fields:**
- `proxied: true` = message was sent to remote Claude CLI
- `remote_message_id` = the ID returned by remote Claude (used for `--resume`)
- `compacted_count` = how many messages were summarized into this system message

**When `compact_session(n)` runs:**
1. Read last N messages from JSONL
2. Send to local Claude for summarization
3. **Archive deleted messages** to `data/history/archive/{hashtag}_{timestamp}.jsonl`
4. Remove those N lines from main history file
5. Append summary as `{"role": "system", "content": "[Compacted context summary]: ...", "compacted_count": N}`

**Archive directory structure:**
```
data/history/
├── default.jsonl              # Active history
├── refactor-auth.jsonl        # Active history
└── archive/
    ├── default_1702598400.jsonl    # Archived from compact
    ├── default_1702684800.jsonl    # Another compact
    └── refactor-auth_1702600000.jsonl
```

Each archive file contains the exact messages that were compacted, preserving full context for debugging.

### Phase 4: SSH Proxy (ssh_proxy.py)

Create `SSHProxy` class with secure execution:

**Security approach:**
- Use `subprocess.run()` with `shell=False` (list format)
- Use `shlex.quote()` for remote command strings
- Validate hostnames against config whitelist only

**Claude CLI flags:**
- `-p` / `--print`: Non-interactive mode
- `--output-format stream-json`: Streaming JSON for parsing
- `--resume <session-id>`: Continue existing session

**Execute pattern:**
```python
cmd = ["ssh", f"{user}@{hostname}",
       f"claude -p --output-format stream-json --resume {shlex.quote(resume_id)} {shlex.quote(message)}"]
result = await asyncio.create_subprocess_exec(*cmd, ...)
```

**Parse streaming JSON output:**
- `{"type": "init", "session_id": "..."}` - capture session ID
- `{"type": "content_block_delta", "delta": {"text": "..."}}` - accumulate content
- `{"type": "result", "message_id": "..."}` - capture for next resume

### Phase 5: Dev Agent (dev_agent.py)

Create `DevAgent` class using Anthropic SDK tool use (via `anthropic_client.py`):

**Tools defined:**
| Tool | Description |
|------|-------------|
| `proxy_message` | Forward message to current session's remote Claude |
| `get_sessions` | List active sessions with stats |
| `switch_session` | Switch to #hashtag session |
| `new_session` | Create session (optional hostname param) |
| `info` | Detailed stats for current/specified session |
| `get_hosts` | List configured hostnames |
| `compact_session` | Summarize last N messages to 1 |

**Routing logic:**
1. Slash commands (`/sessions`, `/switch`, etc.) - parse directly, skip LLM
2. Everything else - agent loop decides (default: proxy to current session)

**Agent system prompt:**
```
You are a developer assistant managing remote Claude sessions.
By default, proxy all development messages to the current session.
Use session tools when user explicitly mentions sessions or switching.
```

### Phase 6: Integration (loop.py)

**Fully replace ChatGPT path with DevAgent:**

```python
# loop.py - simplified
from dev_agent import DevAgent
from serve_config import get_config

class ConversationManager:
    def __init__(self):
        self.history = []  # Local history (mirrors what DevAgent tracks)
        self.context = {}
        self._agent = None

    def _get_agent(self) -> DevAgent:
        if self._agent is None:
            config = get_config()
            self._agent = DevAgent(config)
        return self._agent

    async def process_input(self, user_input: str) -> str:
        """All input goes through DevAgent."""
        self.add_message("user", user_input)

        agent = self._get_agent()
        response = await agent.process_input(user_input)

        self.add_message("assistant", response)
        return response
```

**Note:** DevAgent maintains its own persistent history via SessionManager. The local `self.history` in ConversationManager is kept for display/debugging purposes only and mirrors what DevAgent tracks.

**Changes to main():**
- Remove `--model` flag (or repurpose for local agent model selection)
- Remove ChatGPT-specific initialization
- System prompt now lives in DevAgent

## Data Flow Example

**User asks: "How does auth work?"**

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. INPUT                                                        │
│    User: "How does auth work?"                                  │
│    Current session: #default (host: devbox)                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. DEV AGENT REASONING (via anthropic_client.py)                │
│    Local Claude decides: This is a development question,        │
│    should be proxied to remote Claude on devbox.                │
│    → Tool call: proxy_message("How does auth work?")            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. SSH PROXY EXECUTION                                          │
│    SessionManager.get_current_session() → {                     │
│      hostname: "devbox",                                        │
│      last_message_id: "msg_prev123"  # from previous exchange   │
│    }                                                            │
│                                                                 │
│    Execute:                                                     │
│    ssh trevor@devbox "claude -p --output-format stream-json \   │
│                       --resume msg_prev123 \                    │
│                       'How does auth work?'"                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. REMOTE CLAUDE RESPONSE (streamed JSON)                       │
│    {"type": "init", "session_id": "sess_abc"}                   │
│    {"type": "content_block_delta", "delta": {"text": "The..."}} │
│    {"type": "content_block_delta", "delta": {"text": " auth"}}  │
│    ...                                                          │
│    {"type": "result", "message_id": "msg_new456"}               │
│                                                                 │
│    Accumulated: "The auth system uses JWT tokens stored in..."  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. PERSISTENCE                                                  │
│    SessionManager.update_session("#default",                    │
│      last_message_id="msg_new456"  # For next --resume          │
│    )                                                            │
│                                                                 │
│    Append to data/history/default.jsonl:                        │
│    {"role":"user","content":"How does auth work?","proxied":true}│
│    {"role":"assistant","content":"The auth system uses JWT...","remote_message_id":"msg_new456"}│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. OUTPUT                                                       │
│    Return to user: "The auth system uses JWT tokens stored      │
│    in httpOnly cookies. Here's how it works..."                 │
└─────────────────────────────────────────────────────────────────┘
```

**Key point:** The `last_message_id` persisted in step 5 allows the next user message to continue the same Claude Code conversation on the remote host via `--resume`.

## Dependencies

Add to `requirements.txt`:
```
anthropic>=0.40.0
```

## Critical Files to Modify/Create

| File | Action |
|------|--------|
| `devbot/anthropic_client.py` | CREATE - Anthropic SDK wrapper with token tracking |
| `devbot/dev_agent.py` | CREATE - Main agent class with tool definitions |
| `devbot/ssh_proxy.py` | CREATE - Secure SSH execution |
| `devbot/session_manager.py` | CREATE - Session + history persistence |
| `devbot/serve_config.py` | MODIFY - Add hosts/anthropic config |
| `devbot/loop.py` | MODIFY - Replace ChatGPT with DevAgent |
| `devbot/data/config.json` | MODIFY - Add anthropic_api_key + hosts |
| `devbot/data/sessions.json` | CREATE - Session metadata |
| `devbot/data/history/` | CREATE - Directory for JSONL history files |
| `devbot/data/history/archive/` | CREATE - Directory for compacted message archives |

## Future Flexibility

**Response length management (Slack/SMS)** - Deferred for now, but architecture supports:

When needed, add an **output adapter layer**:
```python
# Future: output_adapters.py
class OutputAdapter:
    def format(self, response: str) -> str:
        return response

class SlackAdapter(OutputAdapter):
    MAX_LENGTH = 4000
    def format(self, response: str) -> str:
        if len(response) > self.MAX_LENGTH:
            return response[:self.MAX_LENGTH-3] + "..."
        return response

class SMSAdapter(OutputAdapter):
    MAX_LENGTH = 160
    def format(self, response: str) -> str:
        # Could call local Claude to summarize if too long
        ...
```

**Integration point:** `DevAgent.process_input()` could accept an optional `adapter` parameter, or `loop.py` could wrap the response before display.

**Other extension points:**
- `anthropic_client.py` can be swapped for different LLM backends
- `ssh_proxy.py` could be replaced with HTTP API calls if remote hosts expose REST endpoints
- `SessionManager` interface allows swapping JSON files for SQLite/Redis later

## Open Questions Resolved

1. **SSH safety**: Use subprocess with shell=False + shlex.quote()
2. **Claude CLI streaming**: `--output-format stream-json` returns JSONL with message IDs
3. **Session resume**: `--resume <message-id>` continues conversation
4. **Config structure**: Hosts as dict in config.json, validate against whitelist
5. **Anthropic SDK integration**: New `anthropic_client.py` mirrors `chatgpt.py` pattern with token/cost tracking
6. **OpenAI path**: Fully replaced - DevAgent handles all messages via Anthropic
