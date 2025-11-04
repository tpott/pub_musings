# Payment Session System Design

## Overview

This document describes a payment session system for a Facebook Messenger bot that checks product prices. The system tracks payment links sent to users, validates payment sessions, and stores user/conversation data for compliance and analytics.

### Goals
- Generate unique payment links for each user/conversation
- Track payment status (pending/completed/expired)
- Cache conversation messages for context
- Enable user data deletion for privacy compliance
- Keep payment URLs short (~50-60 characters)

### Key Design Decisions
- **Session IDs**: 8 hex characters (64-bit entropy, ~1 in 1B collision risk with 1M active sessions)
- **URL Format**: `https://example.com/pay?r=a3f9c2e1` (uses `?r=` instead of `?request_id=`)
- **Expiry**: 24 hours (hardcoded constant)
- **Storage**: File-based system with separate directories for sessions, users, and conversations
- **Status Values**: `"pending"`, `"completed"`, `"expired"`

---

## Architecture

### Directory Structure

```
payment_sessions_dir/
  a3f9c2e1.json              # Session file (see schema below)
  b7d4e8f2.json
  ...

users_dir/
  6397427050373172/           # user_id from Facebook
    status.json               # User metadata (see schema below)
  108420048810326/
    status.json
  ...

conversations_dir/
  t_123456789/                # conversation_id (thread ID) from Facebook
    status.json               # Conversation metadata (see schema below)
    messages/
      1730678400.json         # Timestamp-named message files
      1730678455.json
      ...
  t_987654321/
    status.json
    messages/
      ...
```

### File Schemas

#### `payment_sessions_dir/{session_id}.json`
```json
{
  "user_id": "6397427050373172",
  "conversation_id": "t_123456789",
  "status": "pending",
  "created_at": 1730678400,
  "expires_at": 1730764800,
  "facebook_response": null
}
```

After payment completion:
```json
{
  "user_id": "6397427050373172",
  "conversation_id": "t_123456789",
  "status": "completed",
  "created_at": 1730678400,
  "expires_at": 1730764800,
  "facebook_response": {
    "payment_id": "...",
    "amount": "...",
    "status": "completed",
    "request_id": "a3f9c2e1"
  }
}
```

**Fields**:
- `user_id` (string): Facebook user ID who initiated the payment
- `conversation_id` (string): Facebook conversation/thread ID
- `status` (string): One of `"pending"`, `"completed"`, `"expired"`
- `created_at` (int): Unix timestamp when session was created
- `expires_at` (int): Unix timestamp when session expires (created_at + 24 hours)
- `facebook_response` (object|null): Full response from FB.ui() callback after payment

#### `users_dir/{user_id}/status.json`
```json
{
  "payment_sessions": ["a3f9c2e1", "b7d4e8f2"],
  "conversations": ["t_123456789", "t_987654321"]
}
```

**Fields**:
- `payment_sessions` (array[string]): List of session IDs associated with this user
- `conversations` (array[string]): List of conversation IDs this user has participated in

#### `conversations_dir/{conversation_id}/status.json`
```json
{
  "payment_sessions": ["a3f9c2e1"],
  "participants": ["6397427050373172"]
}
```

**Fields**:
- `payment_sessions` (array[string]): List of session IDs created for this conversation
- `participants` (array[string]): List of user IDs in this conversation (excluding the bot's page_id)

#### `conversations_dir/{conversation_id}/messages/{timestamp}.json`
```json
{
  "message": "What is the price of iPhone 15?",
  "created_time": "2023-11-03T10:20:00+0000",
  "from": {
    "id": "6397427050373172",
    "name": "Trevor Pottinger"
  }
}
```

This is a cached copy of messages from Facebook's API response.

---

## Configuration

### SERVE_CONFIG Changes

The JSON file pointed to by the `SERVE_CONFIG` environment variable needs three new fields:

```json
{
  "facebook_app_id": "...",
  "facebook_app_secret": "...",
  "open_api_key": "...",
  "webhook_cert_file": "...",
  "webhook_key_file": "...",
  "webhook_hostname": "yourdomain.com",
  "payment_sessions_dir": "/path/to/payment_sessions",
  "users_dir": "/path/to/users",
  "conversations_dir": "/path/to/conversations",
  "pages": [...]
}
```

**New Fields**:
- `webhook_hostname` (string, required): The hostname for constructing payment URLs (e.g., `"yourdomain.com"`)
- `payment_sessions_dir` (string, required): Absolute path to directory for storing payment session files
- `users_dir` (string, required): Absolute path to directory for storing user data
- `conversations_dir` (string, required): Absolute path to directory for storing conversation data

### Constants

Add to `serve_config.py` or appropriate module:

```python
PAYMENT_LINK_EXPIRY_HOURS = 24
```

---

## API Endpoints

### GET /pay

**Purpose**: Render the payment page

**Query Parameters**:
- `r` (string, optional): 8-character hex session ID

**Behavior**:
1. Read `r` parameter from query string
2. If `r` is missing or empty:
   - Pass `YOUR_REQUEST_ID = None` and `YOUR_STATUS = None` to template
3. If `r` is provided:
   - Call `get_payment_session(r)`
   - If session not found or expired:
     - Pass `YOUR_REQUEST_ID = None` and `YOUR_STATUS = None` to template
   - If session found:
     - Pass `YOUR_REQUEST_ID = session_id` and `YOUR_STATUS = session["status"]` to template
4. Also pass `YOUR_APP_ID` from config
5. Render `pay.html.tmpl` with these variables

**Response**: HTML page (200 OK)

### POST /payment-callback

**Purpose**: Process payment completion from Facebook SDK callback

**Request Body** (JSON):
```json
{
  "session_id": "a3f9c2e1",
  "facebook_response": {
    "payment_id": "...",
    "amount": "...",
    "status": "completed",
    "request_id": "a3f9c2e1"
  }
}
```

**Behavior**:
1. Parse JSON body
2. Validate `session_id` exists
3. Call `complete_payment_session(session_id, facebook_response)`
4. Return success response

**Response** (JSON):
```json
{
  "status": "ok"
}
```

**Error Responses**:
- 400 Bad Request: Invalid JSON or missing fields
- 404 Not Found: Session ID not found
- 500 Internal Server Error: File write errors

### POST /delete-me (Updated)

**Purpose**: Handle Facebook data deletion requests

**Current Behavior**: Validates signature, generates confirmation code

**New Behavior**:
1. Perform existing signature validation
2. Extract `user_id` from request body
3. **Delete user directory**: `rm -rf users_dir/{user_id}/`
4. **Find and delete user's payment sessions**:
   - Scan `payment_sessions_dir/`
   - For each `{session_id}.json`, check if `user_id` matches
   - Delete matching session files
5. Return confirmation response (existing behavior)

**Note**: Conversations are NOT deleted (they may have other participants)

---

## Component Details

### 1. session_storage.py (New File)

This module provides helper functions for managing payment sessions, users, and conversations.

```python
import json
import os
import secrets
import time
from typing import Dict, List, Optional

from serve_config import getConfig, PAYMENT_LINK_EXPIRY_HOURS


def create_payment_session(user_id: str, conversation_id: str) -> str:
    """
    Create a new payment session and update user/conversation status files.

    Args:
        user_id: Facebook user ID
        conversation_id: Facebook conversation/thread ID

    Returns:
        session_id: 8-character hex string

    Side effects:
        - Creates payment_sessions_dir/{session_id}.json
        - Creates/updates users_dir/{user_id}/status.json
        - Creates/updates conversations_dir/{conversation_id}/status.json
    """
    config = getConfig()
    session_id = secrets.token_hex(4)  # 8 hex chars

    # Create session file
    session = {
        "user_id": user_id,
        "conversation_id": conversation_id,
        "status": "pending",
        "created_at": int(time.time()),
        "expires_at": int(time.time()) + (PAYMENT_LINK_EXPIRY_HOURS * 3600),
        "facebook_response": None
    }
    session_path = os.path.join(config['payment_sessions_dir'], f'{session_id}.json')
    with open(session_path, 'w') as f:
        json.dump(session, f, indent=2)

    # Update user status
    update_user_status(user_id, session_id, conversation_id)

    # Update conversation status
    update_conversation_status(conversation_id, session_id, [user_id])

    return session_id


def get_payment_session(session_id: str) -> Optional[Dict]:
    """
    Retrieve a payment session by ID, checking expiry.

    Args:
        session_id: 8-character hex string

    Returns:
        Session dict if found and not expired, None otherwise
    """
    config = getConfig()
    session_path = os.path.join(config['payment_sessions_dir'], f'{session_id}.json')

    if not os.path.exists(session_path):
        return None

    with open(session_path, 'r') as f:
        session = json.load(f)

    # Check expiry
    if session['expires_at'] < int(time.time()):
        return None

    return session


def complete_payment_session(session_id: str, facebook_response: Dict) -> bool:
    """
    Mark a payment session as completed and store Facebook's response.

    Args:
        session_id: 8-character hex string
        facebook_response: Full response object from FB.ui() callback

    Returns:
        True if successful, False if session not found

    Side effects:
        - Updates payment_sessions_dir/{session_id}.json
    """
    config = getConfig()
    session_path = os.path.join(config['payment_sessions_dir'], f'{session_id}.json')

    if not os.path.exists(session_path):
        return False

    with open(session_path, 'r') as f:
        session = json.load(f)

    session['status'] = 'completed'
    session['facebook_response'] = facebook_response

    with open(session_path, 'w') as f:
        json.dump(session, f, indent=2)

    return True


def update_user_status(user_id: str, session_id: str, conversation_id: str) -> None:
    """
    Add session and conversation to user's status file.

    Args:
        user_id: Facebook user ID
        session_id: 8-character hex string
        conversation_id: Facebook conversation ID

    Side effects:
        - Creates users_dir/{user_id}/ if needed
        - Creates/updates users_dir/{user_id}/status.json
    """
    config = getConfig()
    user_dir = os.path.join(config['users_dir'], user_id)
    os.makedirs(user_dir, exist_ok=True)

    status_path = os.path.join(user_dir, 'status.json')

    # Load existing status or create new
    if os.path.exists(status_path):
        with open(status_path, 'r') as f:
            status = json.load(f)
    else:
        status = {
            "payment_sessions": [],
            "conversations": []
        }

    # Add session and conversation if not already present
    if session_id not in status['payment_sessions']:
        status['payment_sessions'].append(session_id)
    if conversation_id not in status['conversations']:
        status['conversations'].append(conversation_id)

    with open(status_path, 'w') as f:
        json.dump(status, f, indent=2)


def update_conversation_status(conversation_id: str, session_id: Optional[str], participants: List[str]) -> None:
    """
    Update conversation status with session and/or participants.

    Args:
        conversation_id: Facebook conversation ID
        session_id: 8-character hex string (can be None if just updating participants)
        participants: List of user IDs (excluding page_id)

    Side effects:
        - Creates conversations_dir/{conversation_id}/ if needed
        - Creates/updates conversations_dir/{conversation_id}/status.json
    """
    config = getConfig()
    conv_dir = os.path.join(config['conversations_dir'], conversation_id)
    os.makedirs(conv_dir, exist_ok=True)

    status_path = os.path.join(conv_dir, 'status.json')

    # Load existing status or create new
    if os.path.exists(status_path):
        with open(status_path, 'r') as f:
            status = json.load(f)
    else:
        status = {
            "payment_sessions": [],
            "participants": []
        }

    # Add session if provided
    if session_id is not None and session_id not in status['payment_sessions']:
        status['payment_sessions'].append(session_id)

    # Update participants (replace with new list to handle changes)
    for participant in participants:
        if participant not in status['participants']:
            status['participants'].append(participant)

    with open(status_path, 'w') as f:
        json.dump(status, f, indent=2)


def cache_messages(conversation_id: str, messages: List[Dict]) -> None:
    """
    Cache messages to conversation directory.

    Args:
        conversation_id: Facebook conversation ID
        messages: List of message objects from Facebook API

    Side effects:
        - Creates conversations_dir/{conversation_id}/messages/ if needed
        - Writes message files with timestamp names
    """
    config = getConfig()
    messages_dir = os.path.join(config['conversations_dir'], conversation_id, 'messages')
    os.makedirs(messages_dir, exist_ok=True)

    for message in messages:
        # Parse created_time to get timestamp
        # Format: "2023-11-03T10:20:00+0000"
        from datetime import datetime, timezone
        created_time = datetime.strptime(message['created_time'], '%Y-%m-%dT%H:%M:%S%z')
        timestamp = int(created_time.timestamp())

        message_path = os.path.join(messages_dir, f'{timestamp}.json')
        with open(message_path, 'w') as f:
            json.dump(message, f, indent=2)


def delete_user_data(user_id: str) -> None:
    """
    Delete all data for a user (for privacy compliance).

    Args:
        user_id: Facebook user ID

    Side effects:
        - Deletes users_dir/{user_id}/ directory
        - Deletes all payment sessions belonging to this user
    """
    import shutil

    config = getConfig()

    # Delete user directory
    user_dir = os.path.join(config['users_dir'], user_id)
    if os.path.exists(user_dir):
        shutil.rmtree(user_dir)

    # Find and delete user's payment sessions
    sessions_dir = config['payment_sessions_dir']
    for filename in os.listdir(sessions_dir):
        if not filename.endswith('.json'):
            continue

        session_path = os.path.join(sessions_dir, filename)
        with open(session_path, 'r') as f:
            session = json.load(f)

        if session.get('user_id') == user_id:
            os.remove(session_path)
```

### 2. serve_config.py Updates

Add the constant and update validation in `getConfig()`:

```python
# Add this constant at module level
PAYMENT_LINK_EXPIRY_HOURS = 24

def getConfig() -> Dict[str, Any]:
    # ... existing code ...

    # Add validation after existing asserts
    assert 'webhook_hostname' in config, "Config missing required field: webhook_hostname"
    assert 'payment_sessions_dir' in config, "Config missing required field: payment_sessions_dir"
    assert 'users_dir' in config, "Config missing required field: users_dir"
    assert 'conversations_dir' in config, "Config missing required field: conversations_dir"

    return config
```

### 3. webhooks.py Updates

#### Update `getPayPage()` method (around line 161):

```python
def getPayPage(self):
    config = getConfig()

    # Import here to avoid circular dependency
    from session_storage import get_payment_session

    # Parse query parameter
    request = urllib.parse.urlparse(self.path)
    params = urllib.parse.parse_qs(request.query)
    session_id = params.get('r', [None])[0]

    # Get session if session_id provided
    session = None
    status = None
    if session_id:
        session = get_payment_session(session_id)
        if session:
            status = session['status']

    # Render template
    self.getTemplateFile('pay.html.tmpl', 'Pay template file not found', {
        'YOUR_APP_ID': config['facebook_app_id'],
        'YOUR_REQUEST_ID': session_id if session else 'null',
        'YOUR_STATUS': f'"{status}"' if status else 'null',
    })
```

#### Add `postPaymentCallback()` method:

```python
def postPaymentCallback(self):
    from session_storage import complete_payment_session

    content_length = int(self.headers.get('Content-Length', 0))
    if content_length == 0:
        s = b'{"error": "No request body provided"}'
        self.send_response(http.server.HTTPStatus.BAD_REQUEST)
        self.send_header('Content-Length', len(s))
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(s)
        return

    body = self.rfile.read(content_length)

    try:
        data = json.loads(body.decode('utf-8'))
    except json.JSONDecodeError:
        s = b'{"error": "Invalid JSON in request body"}'
        self.send_response(http.server.HTTPStatus.BAD_REQUEST)
        self.send_header('Content-Length', len(s))
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(s)
        return

    session_id = data.get('session_id')
    facebook_response = data.get('facebook_response')

    if not session_id or not facebook_response:
        s = b'{"error": "Missing session_id or facebook_response"}'
        self.send_response(http.server.HTTPStatus.BAD_REQUEST)
        self.send_header('Content-Length', len(s))
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(s)
        return

    success = complete_payment_session(session_id, facebook_response)

    if not success:
        s = b'{"error": "Session not found"}'
        self.send_response(http.server.HTTPStatus.NOT_FOUND)
        self.send_header('Content-Length', len(s))
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(s)
        return

    s = b'{"status": "ok"}'
    self.send_response(http.server.HTTPStatus.OK)
    self.send_header('Content-Length', len(s))
    self.send_header('Content-Type', 'application/json')
    self.end_headers()
    self.wfile.write(s)
```

#### Update `postHandler()` method (around line 296):

Add this route:

```python
def postHandler(self):
    request = urllib.parse.urlparse(self.path)
    # ... existing routes ...

    if request.path == '/payment-callback':
        self.postPaymentCallback()
        return

    # ... rest of postHandler ...
```

#### Update `handleDeleteMe()` method (around line 331):

After validating signature and extracting user_id, add:

```python
# Around line 379, after extracting user_id
from session_storage import delete_user_data

# Delete all user data
delete_user_data(user_id)

# Then continue with existing confirmation code logic...
confirmation_code = self.generate_confirmation_code()
# ... rest of existing code ...
```

### 4. pay.html.tmpl Updates

Replace the entire file with:

```html
<html>
  <head>
    <title>Pay for using price checker</title>
  </head>
  <body>
    <script>
var requestId = {YOUR_REQUEST_ID};
var status = {YOUR_STATUS};

function callback(res) {{
  console.log('Got a dialog callback', res);

  // Send result back to server
  fetch('/payment-callback', {{
    method: 'POST',
    headers: {{
      'Content-Type': 'application/json'
    }},
    body: JSON.stringify({{
      session_id: requestId,
      facebook_response: res
    }})
  }})
  .then(response => response.json())
  .then(data => {{
    console.log('Server response:', data);
    if (data.status === 'ok') {{
      document.body.innerHTML = '<div style="padding: 20px; font-family: sans-serif;"><h2>Payment Successful!</h2><p>Thank you for your payment.</p></div>';
    }}
  }})
  .catch(error => {{
    console.error('Error sending payment to server:', error);
  }});
}}

// Check session status before initializing Facebook SDK
if (requestId === null || status === null) {{
  // Invalid or missing session
  document.body.innerHTML = '<div style="padding: 20px; font-family: sans-serif;"><h2>Invalid Payment Link</h2><p>Please request a new payment link.</p></div>';
}} else if (status === 'completed') {{
  // Already paid
  document.body.innerHTML = '<div style="padding: 20px; font-family: sans-serif;"><h2>Payment Already Completed</h2><p>Thank you! This payment has already been processed.</p></div>';
}} else {{
  // Status is "pending", show payment dialog
  window.fbAsyncInit = function() {{
    FB.init({{
      appId            : '{YOUR_APP_ID}',
      xfbml            : true,
      version          : 'v23.0'
    }});

    FB.ui({{
        method: 'pay',
        action: 'purchaseitem',
        product: window.location.origin + '/og/price-checker',
        request_id: requestId
      }},
      callback
    );
  }};

  // Load Facebook SDK
  var script = document.createElement('script');
  script.src = 'https://connect.facebook.net/en_US/sdk.js';
  script.async = true;
  script.defer = true;
  script.crossOrigin = 'anonymous';
  document.body.appendChild(script);
}}
    </script>
  </body>
</html>
```

**Key Changes**:
- Added conditional rendering based on `requestId` and `status`
- If session invalid: Show error message
- If already completed: Show success message
- If pending: Initialize Facebook SDK and show payment dialog
- Added `fetch()` call in callback to POST result to `/payment-callback`
- Dynamically load Facebook SDK script only when needed

### 5. facebook_loop.py Updates

#### Add imports:

```python
from session_storage import (
    create_payment_session,
    update_conversation_status,
    cache_messages
)
```

#### Update `getRecentMessages()` function (around line 104):

```python
def getRecentMessages(conversation_id: str, page_token: str) -> List[Dict[str, Any]]:
    resp = requests.get(f'https://graph.facebook.com/{conversation_id}?fields=messages{{message,created_time,from}}&access_token={page_token}')
    results = resp.json()
    print(results)

    messages = sorted(results['messages']['data'], key=lambda x: x['created_time'])

    # Cache messages
    cache_messages(conversation_id, messages)

    return messages
```

#### Update `runOnce()` function (around line 156):

```python
def runOnce() -> None:
    config = getConfig()
    app_id = config['facebook_app_id']
    app_secret = config['facebook_app_secret']
    webhook_hostname = config['webhook_hostname']

    for page_config in config['pages']:
        page_id = page_config['page_id']
        page_token = page_config['page_token']
        last_run_time = page_config['last_run']

        print(f"Processing page {page_id}")

        new_token = maybeRefreshToNonExpiringToken(app_id, app_secret, page_token)
        if new_token != page_token:
            print(f"Token was refreshed for page {page_id}")
            page_config['page_token'] = new_token
            page_token = new_token

        conversations = getRecentConversations(page_id, page_token, last_run_time)
        for conv in conversations:
            messages = getRecentMessages(conv[0], page_token)

            # Update conversation participants
            participants = [conv[1]]  # conv[1] is the user_id
            update_conversation_status(conv[0], None, participants)

            if messages[-1]['from']['id'] == page_id:
                print(f'skipping conversation t_id {conv[0]} with {conv[1]}')
                continue

            # ... existing message processing ...

            context_messages = []
            for msg in messages:
                if msg['from']['id'] == page_id:
                    context_messages.append({"role": "assistant", "content": msg['message']})
                else:
                    context_messages.append({"role": "user", "content": msg['message']})
                print(context_messages[-1])

            summary_obj = priceSummaries(context_messages[-1]['content'], model='gpt-5')
            if 'error' in summary_obj:
                postMessage(page_id, page_token, conv, summary_obj['error'])
                continue

            for summary in summary_obj['summaries']:
                target = summary['target']
                url = summary['url']
                if 'summary_text' not in summary:
                    postMessage(page_id, page_token, conv, f"{target}\nURL: {url}")
                    continue
                summary_text = summary['summary_text']
                postMessage(page_id, page_token, conv, f"{target}\n{summary_text}\nURL: {url}")

            # TODO: Add logic here to decide when to send payment links
            # Example: After N free queries, send payment link
            # session_id = create_payment_session(conv[1], conv[0])
            # payment_url = f"https://{webhook_hostname}/pay?r={session_id}"
            # postMessage(page_id, page_token, conv, f"To continue using the price checker, please complete payment: {payment_url}")

        page_config['last_run'] = int(time.time())

    saveConfig(config)
```

**Note**: The payment link generation is commented out as a TODO. You'll need to add logic to determine when to send payment links (e.g., after a user has made N queries).

---

## Payment Flow

### Step-by-Step User Journey

1. **User sends message to Facebook page**
   - User: "What's the price of iPhone 15?"

2. **Bot processes message** (facebook_loop.py)
   - `runOnce()` fetches recent conversations
   - `getRecentMessages()` fetches and caches messages
   - Bot calls OpenAI API to get price summary
   - Bot sends price summary back to user

3. **Bot decides user needs to pay** (business logic)
   - After N free queries, bot needs payment
   - Calls `create_payment_session(user_id, conversation_id)` → returns `"a3f9c2e1"`
   - Constructs URL: `https://yourdomain.com/pay?r=a3f9c2e1`
   - Sends message: "To continue, please complete payment: https://yourdomain.com/pay?r=a3f9c2e1"

4. **User clicks payment link**
   - Browser requests `GET /pay?r=a3f9c2e1`
   - webhooks.py validates session (checks if exists and not expired)
   - Renders pay.html.tmpl with session data

5. **Payment page loads** (pay.html.tmpl)
   - JavaScript checks session status
   - If pending: Loads Facebook SDK and shows payment dialog
   - If completed: Shows "already paid" message
   - If invalid: Shows error message

6. **User completes payment in Facebook dialog**
   - Facebook processes payment
   - Calls JavaScript `callback(res)` with payment result

7. **Callback POSTs to server**
   - JavaScript sends `POST /payment-callback` with:
     ```json
     {
       "session_id": "a3f9c2e1",
       "facebook_response": {...}
     }
     ```
   - webhooks.py calls `complete_payment_session()`
   - Session file updated to `"status": "completed"`

8. **Future queries**
   - User can continue using bot
   - Bot can check if user has completed payments before responding

---

## Error Handling

### Edge Cases

#### Expired Sessions
- **When**: User clicks link after 24 hours
- **Behavior**: `get_payment_session()` returns `None`, template shows "Invalid payment link"
- **Solution**: User requests new payment link from bot

#### Invalid/Missing Session ID
- **When**: User visits `/pay` without `?r=` or with wrong ID
- **Behavior**: Template shows "Invalid payment link"
- **Solution**: User requests payment link from bot

#### Duplicate Payment Attempts
- **When**: User clicks payment link after already paying
- **Behavior**: Template shows "Payment already completed" message
- **Facebook Behavior**: Facebook's API may reject duplicate `request_id`, but we prevent it at template level

#### Network Errors During Callback POST
- **When**: POST to `/payment-callback` fails due to network issues
- **Behavior**: Payment completed with Facebook, but our database not updated
- **Mitigation**: Facebook may have webhooks for payment events (check Facebook Payments API docs)
- **Manual Fix**: Admin can manually update session file

#### Concurrent Session Creation
- **When**: Bot sends multiple payment links to same user simultaneously
- **Behavior**: Multiple sessions created, all valid
- **Okay**: Each session has unique ID, no collision

#### User Data Deletion During Active Session
- **When**: User requests deletion (`POST /delete-me`) while having pending payment session
- **Behavior**: User directory and all sessions deleted
- **Result**: Payment links become invalid (session files deleted)
- **Okay**: User data deletion takes precedence

---

## Security Considerations

### Session Security
- **Short IDs**: 8 hex chars = 4 billion possibilities, collision risk low for short-lived sessions
- **Expiry**: 24-hour expiry reduces attack window
- **No secret data in URL**: Session ID is just a lookup key, not sensitive

### Payment Validation
- **Facebook handles payment security**: We don't process credit cards
- **Session ties payment to user**: Can't reuse someone else's payment link meaningfully (Facebook knows who's logged in)

### Privacy Compliance
- **Data deletion**: `delete_user_data()` removes all user traces
- **Message caching**: Conversations cached for context, deleted on user request
- **No sensitive data**: Only store Facebook IDs and message content (which Facebook already has)

### HTTPS Required
- **Facebook SDK requires HTTPS**: Payment dialogs won't work over HTTP
- **SSL configured**: webhooks.py already uses SSL context

---

## Implementation Checklist

### Phase 1: Configuration and Storage
- [ ] Add `webhook_hostname`, `payment_sessions_dir`, `users_dir`, `conversations_dir` to your SERVE_CONFIG JSON file
- [ ] Create the three directories with appropriate permissions
- [ ] Update `serve_config.py`: Add `PAYMENT_LINK_EXPIRY_HOURS = 24` constant
- [ ] Update `serve_config.py`: Add validation for new config fields in `getConfig()`

### Phase 2: Session Storage Module
- [ ] Create `session_storage.py` with all helper functions:
  - [ ] `create_payment_session()`
  - [ ] `get_payment_session()`
  - [ ] `complete_payment_session()`
  - [ ] `update_user_status()`
  - [ ] `update_conversation_status()`
  - [ ] `cache_messages()`
  - [ ] `delete_user_data()`

### Phase 3: Webhooks Updates
- [ ] Update `webhooks.py` - `getPayPage()` method to read `?r=` param and validate session
- [ ] Update `webhooks.py` - Add `postPaymentCallback()` method
- [ ] Update `webhooks.py` - Add `/payment-callback` route to `postHandler()`
- [ ] Update `webhooks.py` - Update `handleDeleteMe()` to call `delete_user_data()`

### Phase 4: Template Updates
- [ ] Replace `pay.html.tmpl` with new version that:
  - [ ] Checks session status
  - [ ] Shows error for invalid sessions
  - [ ] Shows success for completed sessions
  - [ ] Shows payment dialog for pending sessions
  - [ ] POSTs callback to `/payment-callback`

### Phase 5: Facebook Loop Integration
- [ ] Update `facebook_loop.py` - Add imports for session_storage functions
- [ ] Update `facebook_loop.py` - Update `getRecentMessages()` to call `cache_messages()`
- [ ] Update `facebook_loop.py` - Update `runOnce()` to call `update_conversation_status()`
- [ ] Update `facebook_loop.py` - Add payment link generation logic (TODO based on business rules)

### Phase 6: Testing
- [ ] Test invalid session: Visit `/pay?r=invalid123`
- [ ] Test missing session: Visit `/pay`
- [ ] Test valid pending session: Create session, visit link, verify payment dialog shows
- [ ] Test payment completion: Complete payment, verify POST to `/payment-callback` works
- [ ] Test duplicate payment: Visit same link after completion, verify "already paid" message
- [ ] Test expired session: Create session, wait 24+ hours (or manually edit expiry), verify error
- [ ] Test data deletion: Create user data, call `/delete-me`, verify all files deleted
- [ ] Test message caching: Send messages, verify files created in conversations_dir

### Phase 7: Production
- [ ] Add monitoring for payment session creation/completion rates
- [ ] Add cleanup job for expired sessions (optional: saves disk space)
- [ ] Document payment link generation logic (business rules for when to charge)
- [ ] Consider Facebook Payments webhooks for payment event notifications

---

## Future Enhancements

1. **Cleanup Job**: Periodic deletion of expired session files to save disk space
2. **Payment History**: Add endpoint to show user their payment history
3. **Usage Tracking**: Track queries per user to determine when to request payment
4. **Pricing Tiers**: Multiple payment amounts for different service levels
5. **Facebook Payments Webhooks**: Listen for payment events from Facebook for redundancy
6. **Admin Dashboard**: Web UI to view sessions, users, and payments

---

## Notes

- This design uses file-based storage for simplicity. For production scale, consider migrating to a database (SQLite, PostgreSQL)
- Facebook SDK v23.0 is used; check for updates
- The `request_id` parameter in FB.ui() is optional but recommended for tracking
- Facebook may have rate limits on payment API calls
- Consider adding logging for debugging payment issues
