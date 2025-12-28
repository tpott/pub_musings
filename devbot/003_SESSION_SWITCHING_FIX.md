# Plan: Fix Matrix Session Switching Bug & Extend History Format

## Summary
Two issues to fix:
1. Matrix bot forces session switch on every message, breaking `/switch` command
2. History files don't capture tool calls and tool results

## Bug 1: Matrix Session Switching

### Root Cause
`matrix_loop.py:189-194` forces a switch to the room's session on EVERY incoming message:
```python
session_name = self._get_or_create_room_session(room.room_id)
self.session_manager.switch_session(session_name)  # Forces session before processing
```

When user runs `/switch #default2`, it works momentarily, but next message resets it.

### Fix
Remove the forced `switch_session()` call on line 191. Only ensure the session exists:

**File: `matrix_loop.py`**
- Line 191: Delete `self.session_manager.switch_session(session_name)`
- The `_get_or_create_room_session` method already calls `create_session()` for NEW rooms, which sets current_session appropriately
- For existing rooms, we no longer force the switch back

### Behavior after fix
- New room: First message creates session and sets it as current (via `create_session`)
- Existing room: Uses whatever session is currently active
- `/switch #other` persists across subsequent messages
- Session is global, not per-room (simpler architecture)

### Alternative: Per-room session tracking
If you later want per-room session persistence (room A switched to X, room B switched to Y), that would require:
- Storing `room_id -> current_session_override` mapping
- Switching to the room's overridden session (or default) on each message
This is more complex and not part of this fix.

## Bug 2: Extended History Format

### Current Format
```json
{"role": "user", "content": "...", "timestamp": 1234, "proxied": true}
{"role": "assistant", "content": "...", "timestamp": 1234, "remote_message_id": "..."}
```

### New Format
Add optional `tool_calls` and `tool_results` fields:
```json
{"role": "assistant", "content": "...", "timestamp": 1234, "tool_calls": [{"id": "...", "name": "...", "input": {...}}]}
{"role": "user", "content": null, "timestamp": 1234, "tool_results": [{"tool_use_id": "...", "content": "..."}]}
```

### Changes

**File: `session_manager.py`**
- `add_message()` (line 121): Add optional parameters:
  - `tool_calls: Optional[List[Dict]] = None`
  - `tool_results: Optional[List[Dict]] = None`
- Store these in the message dict when provided

**File: `dev_agent.py`**
- `process_input()` (lines 184-211): After tool execution, call `session_manager.add_message()` with tool_calls/tool_results
- Specifically, when building the assistant message with tool_use (line 185-197), also persist it
- When building the tool_result message (line 199-211), also persist it

## Files to Modify
1. `matrix_loop.py` - Remove forced session switch (lines 189-194)
2. `session_manager.py` - Extend add_message() signature (line 121+)
3. `dev_agent.py` - Persist tool calls/results in history (lines 184-211)

## Testing
1. Matrix session switching:
   - Start matrix_loop.py
   - Send `/info` - should show matrix session
   - Send `/sessions` - should list sessions
   - Send `/switch #default` - should switch
   - Send `/info` - should now show #default (not matrix session)

2. History format:
   - Send a message that triggers tool use via Matrix or loop.py
   - Check the history JSONL file for tool_calls/tool_results fields
