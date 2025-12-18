# Matrix Bot Integration Plan

## Overview
Update `matrix_loop.py` to be a Matrix bot that:
- Uses config from `serve_config` with stored access tokens
- Listens to messages and routes them through `DevAgent` (like `loop.py`)
- Supports E2E encryption with emoji verification
- Stores keys in `data/` directory

## Files to Create/Modify

### 1. `matrix_login.py` (NEW) - Interactive Login Script
Interactive script for first-time Matrix login and device verification.

**Responsibilities:**
- Prompt for homeserver, username, password
- Login and store credentials to `data/matrix_credentials.json`
- Create E2E key store at `data/matrix_store/`
- Wait for and handle emoji verification requests
- Exit cleanly once verified

**Key code pattern** (from `matrix_example_verify_with_emoji.py`):
```python
client_config = AsyncClientConfig(
    store_sync_tokens=True,
    encryption_enabled=True,
)
client = AsyncClient(homeserver, user_id, store_path=STORE_PATH, config=client_config)
```

### 2. `matrix_loop.py` (MODIFY) - Main Bot Loop
Rewrite to be a proper bot using stored credentials and DevAgent.

**Config additions to `data/config.json`:**
```json
{
  "matrix": {
    "allowed_users": ["@user1:matrix.org", "@user2:example.com"],
    "max_room_size": 5
  }
}
```

**Structure:**
```
MatrixBot class:
  - __init__(config): Load matrix credentials, init DevAgent
  - async start(): Login with stored credentials, register callbacks, sync_forever
  - async message_callback(room, event):
      - Check sender is in allowed_users
      - Check room size <= max_room_size
      - Get/create ConversationManager for (room_id, sender) pair
      - Process through DevAgent
      - Send response back to room
```

**Key flow:**
1. Load config via `get_config()`
2. Load matrix credentials from `data/matrix_credentials.json`
3. Initialize AsyncClient with E2E encryption
4. Restore login using stored access_token
5. Register `RoomMessageText` callback
6. Run `sync_forever()`

**Message handling:**
```python
async def message_callback(room: MatrixRoom, event: RoomMessageText):
    # Skip own messages
    if event.sender == client.user_id:
        return

    # Authorization check
    if event.sender not in config["matrix"]["allowed_users"]:
        return

    # Room size check
    if len(room.users) > config["matrix"]["max_room_size"]:
        return

    # Get or create session for this room
    session_name = get_or_create_room_session(room.room_id)
    session_manager.switch_session(session_name)

    # Process through DevAgent (shared per room)
    response = await dev_agent.process_input(event.body)

    # Send response
    await client.room_send(
        room.room_id,
        message_type="m.room.message",
        content={"msgtype": "m.text", "body": response}
    )
```

### 3. `serve_config.py` (MINOR MODIFY)
Add helper functions for Matrix config.

```python
def get_matrix_credentials_path() -> Path:
    return get_data_dir() / "matrix_credentials.json"

def get_matrix_store_path() -> Path:
    return get_data_dir() / "matrix_store"

def get_matrix_room_sessions_path() -> Path:
    return get_data_dir() / "matrix_room_sessions.json"

def get_matrix_config(config: Optional[Dict] = None) -> Dict:
    if config is None:
        config = get_config()
    return config.get("matrix", {})
```

## Data Storage Layout

```
data/
  config.json              # Add "matrix" section for allowed_users, max_room_size
  matrix_credentials.json  # NEW: homeserver, user_id, device_id, access_token
  matrix_store/            # NEW: E2E encryption key store (created by nio)
  matrix_room_sessions.json # NEW: {room_id: session_name} mapping
  history/                 # Existing session history
  sessions.json            # Existing session metadata
```

## Conversation Management (Simplified)
**One session per Matrix room** - all users in a room share the same conversation context.

- Session name derived from room: `#matrix-<room_id_short>` (e.g., `#matrix-abc123`)
- Leverages existing `SessionManager` directly
- On first message in a room, create session via `session_manager.create_session()`
- Natural fit: people in same room discuss related topics with the bot
- Mapping stored in `data/matrix_room_sessions.json`: `{room_id: session_name}`

## Implementation Steps

1. **Add matrix config helpers to `serve_config.py`**
   - `get_matrix_credentials_path()`
   - `get_matrix_store_path()`
   - `get_matrix_config()`

2. **Create `matrix_login.py`**
   - Reuse login logic from `matrix_example_verify_with_emoji.py`
   - Store credentials in `data/matrix_credentials.json`
   - Use `data/matrix_store/` for E2E keys
   - Handle emoji verification callback
   - Exit after successful verification

3. **Rewrite `matrix_loop.py`**
   - Load config and credentials
   - Initialize AsyncClient with E2E
   - Create conversation manager cache
   - Implement authorization checks in callback
   - Route messages through DevAgent
   - Send responses back to Matrix

4. **Update `data/config.json` schema** (documented, user updates manually)
   - Add `matrix.allowed_users` array
   - Add `matrix.max_room_size` integer (default 5)
