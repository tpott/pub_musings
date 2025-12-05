# Migration Plan: FastAPI + OpenAI SDK

## Overview

**Current Architecture:**
- Python's built-in `http.server` + `socketserver.TCPServer` for webhooks (webhooks.py)
- Threading model: main thread runs webhook server, background thread polls Facebook API every 15s
- Raw `requests.post()` calls to OpenAI API (chatgpt.py)
- Callback pattern: webhook can trigger polling via `/trigger-callback`

**Target Architecture:**
- FastAPI for all webhook routes with async/await throughout
- Unified async architecture: single event loop with background tasks for polling
- Official OpenAI Python SDK with preserved cost tracking
- Comprehensive type hints and improved error handling

**User Requirements:**
✅ Unified async architecture (FastAPI background tasks + asyncio)
✅ Preserve detailed token usage and cost tracking
✅ Add type hints throughout
✅ Improve error handling with FastAPI exceptions and better OpenAI error management

---

## Why These Files Are Implemented This Way

### Current Callback Pattern (webhooks.py ↔ facebook_loop.py)

**How it works:**
1. `facebook_loop.py:297` calls `serve('0.0.0.0', 8443, cert_path, privkey_path, lambda: runOnce(token_expiry_cache))`
2. The lambda function is stored in the TCPServer instance via `setCallback()`
3. MyHandler retrieves it in `setup()` and makes it available as `self.callback`
4. GET `/trigger-callback` endpoint calls `self.callback()` to trigger immediate polling

**Why this pattern exists:**
- Allows webhook to trigger immediate message processing instead of waiting for next 15s poll
- Simple threading model keeps webhook server and polling loop independent
- Standard library approach avoids external dependencies

**What we're changing:**
- Replace callback with FastAPI background task that can be triggered directly
- Unified event loop eliminates need for cross-thread communication
- Background task can be awaited and monitored properly

---

## Critical Files & Changes

### 1. **NEW: app.py** - FastAPI Application Entry Point

**Create:** `/Users/trevor/Github/pub_musings/pricecrawler/app.py`

**Core responsibilities:**
- FastAPI app initialization with lifespan management
- All webhook routes (migrated from webhooks.py)
- Background polling task (replaces threading)
- SSL/HTTPS configuration with uvicorn

**Key components:**

```python
from contextlib import asynccontextmanager
from typing import Optional, Dict, Any
import asyncio
from fastapi import FastAPI, Request, Header, Depends, Query, HTTPException
from fastapi.responses import PlainTextResponse, JSONResponse, FileResponse, HTMLResponse
from fastapi.templating import Jinja2Templates
import uvicorn

templates = Jinja2Templates(directory=".")

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup: launch polling task. Shutdown: cancel gracefully."""
    polling_task = asyncio.create_task(polling_loop())
    yield
    polling_task.cancel()
    try:
        await polling_task
    except asyncio.CancelledError:
        pass

app = FastAPI(lifespan=lifespan)
```

**Routes to implement:**
- `GET /status` - Health check
- `GET /app-validation` - Facebook webhook validation
- `GET /page-validation` - Facebook page webhook validation
- `GET /pay?r={session_id}` - Payment page with template
- `GET /privacy-policy` - Text file serving
- `GET /terms` - Terms of service
- `GET /deleted?id={code}` - Deletion confirmation
- `GET /trigger-callback` - Manual polling trigger
- `GET /barcode_scanner.webp` - Static image
- `GET /og/price-checker` - OG template
- `POST /payment-callback` - Payment completion
- `POST /delete-me` - User data deletion with signature verification

**Signature verification as dependency:**
```python
async def verify_facebook_signature(
    request: Request,
    x_hub_signature_256: str = Header(None)
) -> None:
    if not x_hub_signature_256:
        raise HTTPException(status_code=401, detail="Missing signature")

    body = await request.body()
    config = getConfig()
    expected = hmac.new(
        config['facebook_app_secret'].encode(),
        body,
        hashlib.sha256
    ).hexdigest()

    signature = x_hub_signature_256.replace('sha256=', '')
    if not hmac.compare_digest(expected, signature):
        raise HTTPException(status_code=401, detail="Invalid signature")

@app.post("/delete-me")
async def delete_me(
    request: Request,
    _: None = Depends(verify_facebook_signature)
):
    # Handler implementation
```

**SSL setup in main:**
```python
if __name__ == "__main__":
    config = getConfig()
    uvicorn.run(
        "app:app",
        host="0.0.0.0",
        port=8443,
        ssl_keyfile=config.get('webhook_key_file'),
        ssl_certfile=config.get('webhook_cert_file'),
    )
```

---

### 2. **chatgpt.py** - OpenAI SDK Migration

**File:** `/Users/trevor/Github/pub_musings/pricecrawler/chatgpt.py`

**Changes:**

**Add TypedDict for structured returns:**
```python
from typing import TypedDict, List, Dict, Optional

class CompletionResult(TypedDict):
    content: str
    usage: Dict[str, int]
    cost: Dict[str, float]
```

**Migrate to OpenAI SDK:**
```python
from openai import AsyncOpenAI

async def chat_completions(
    messages: List[Dict[str, str]],
    model: Optional[str] = None
) -> CompletionResult:
    """Get chat completions from OpenAI with cost tracking."""
    if model is None:
        model = "gpt-5"

    config = getConfig()
    client = AsyncOpenAI(api_key=config['open_api_key'])

    response = await client.chat.completions.create(
        model=model,
        messages=messages
    )

    # Extract usage from response object
    usage = {
        'prompt_tokens': response.usage.prompt_tokens,
        'completion_tokens': response.usage.completion_tokens,
        'total_tokens': response.usage.total_tokens,
    }

    # Preserve cost calculation
    model_pricing = MODEL_PRICING.get(model, {"input": 2.00, "output": 8.00})
    input_cost = usage['prompt_tokens'] * model_pricing["input"] / 1e6
    output_cost = usage['completion_tokens'] * model_pricing["output"] / 1e6

    cost = {
        'input_cost': input_cost,
        'output_cost': output_cost,
        'total_cost': input_cost + output_cost,
    }

    # Preserve logging
    print(f"Usage: {usage['prompt_tokens']} prompt, {usage['completion_tokens']} completion")
    print(f"Cost: ${cost['total_cost']:.6f}")

    return {
        'content': response.choices[0].message.content,
        'usage': usage,
        'cost': cost,
    }
```

**Key changes:**
- ❌ Remove: `import requests`, function name `chatCompletitions`
- ✅ Add: `from openai import AsyncOpenAI`, function name `chat_completions`
- ✅ Change: Synchronous → async function
- ✅ Change: Return type `str` → `CompletionResult` TypedDict
- ✅ Preserve: MODEL_PRICING table, cost calculation logic, logging

---

### 3. **facebook_loop.py** - Async Conversion

**File:** `/Users/trevor/Github/pub_musings/pricecrawler/facebook_loop.py`

**Major changes:**

**1. Remove threading, requests:**
```python
# Remove
import threading
import requests

# Add
import aiohttp
from typing import Optional, Dict, List, Any, Tuple
```

**2. Convert all functions to async + rename (PEP 8):**

| Old (sync) | New (async) |
|------------|-------------|
| `getMyId(page_token)` | `async def get_my_id(page_token: str) -> Optional[str]` |
| `checkTokenExpiry(...)` | `async def check_token_expiry(...) -> int` |
| `maybeRefreshToNonExpiringToken(...)` | `async def maybe_refresh_to_non_expiring_token(...) -> str` |
| `getRecentConversations(...)` | `async def get_recent_conversations(...) -> List[Dict[str, Any]]` |
| `getRecentMessages(...)` | `async def get_recent_messages(...) -> List[Dict[str, Any]]` |
| `postMessage(...)` | `async def post_message(...) -> None` |
| `runOnce(...)` | `async def run_once(...) -> None` |
| `runLoop(sleep_time)` | `async def polling_loop() -> None` |

**3. Replace requests with aiohttp:**
```python
# Old
resp = requests.get(url)
results = resp.json()

# New
async with aiohttp.ClientSession() as session:
    async with session.get(url) as resp:
        results = await resp.json()
```

**4. Update OpenAI calls:**
```python
# Old
summary_obj = priceSummaries(context_messages[-1]['content'], model='gpt-5')

# New
summary_obj = await price_summaries(context_messages[-1]['content'], model='gpt-5')

# Access content from result
for summary in summary_obj['summaries']:
    # summary_text is now from ChatGPT's CompletionResult['content']
```

**5. Rewrite polling loop:**
```python
async def polling_loop() -> None:
    """Background task that polls Facebook every 15 seconds."""
    token_expiry_cache: Dict[str, int] = {}
    while True:
        try:
            await run_once(token_expiry_cache)
            await asyncio.sleep(15.0)
        except Exception as e:
            print(f"Error in polling loop: {e}")
            import traceback
            traceback.print_exc()
            await asyncio.sleep(15.0)  # Continue despite errors
```

**6. Delete main() function:**
- Replaced by app.py's lifespan and entry point

**Error handling improvements:**
- Add try-except around individual conversation processing
- Log errors with conversation_id context
- Continue processing other conversations on error
- Distinguish transient vs permanent API failures

---

### 4. **pricechecker.py** - Async Function Signature

**File:** `/Users/trevor/Github/pub_musings/pricecrawler/pricechecker.py`

**Changes (minimal - already uses async internally):**

**1. Rename and async-ify main function:**
```python
# Old
def priceSummaries(user_input, model):
    # ... uses asyncio.run() internally

# New
async def price_summaries(user_input: str, model: Optional[str]) -> Dict[str, Any]:
    # ... directly uses await
```

**2. Update chatgpt imports and calls:**
```python
# Old
from chatgpt import chatCompletitions
target = chatCompletitions(messages, model if model is not None else "gpt-5-mini")
summarized = chatCompletitions(messages, model)

# New
from chatgpt import chat_completions
result = await chat_completions(messages, model if model is not None else "gpt-5-mini")
target = result['content']

result = await chat_completions(messages, model)
summarized = result['content']
```

**3. Remove asyncio.run() wrapper:**
```python
# Old
results = asyncio.run(fetch_all(target, targets[target]))

# New
results = await fetch_all(target, targets[target])
```

**4. Add type hints:**
```python
from typing import Optional, Dict, Any, List

async def fetch_url(
    target: str,
    session: aiohttp.ClientSession,
    target_obj: Dict[str, Any]
) -> Tuple[str, Dict, int, str]:
```

**Error handling:**
- Already has try-except in fetch_url
- Add timeout handling for BeautifulSoup parsing
- Return partial results if some URLs fail

---

### 5. **session_storage.py** - No Changes Needed

**File:** `/Users/trevor/Github/pub_musings/pricecrawler/session_storage.py`

**Status:** ✅ Already has type hints, no async needed (file I/O is fast)

**Optionally (if performance issues):**
- Could add `async` variants using `aiofiles` library
- But synchronous file I/O is fine for this use case

---

### 6. **webhooks.py** - DEPRECATE

**File:** `/Users/trevor/Github/pub_musings/pricecrawler/webhooks.py`

**Status:** ⚠️ All logic moved to app.py

**Migration approach:**
1. Copy route handler logic to app.py FastAPI routes
2. Copy helper functions (signature verification) to app.py
3. Delete this file after migration complete

---

### 7. **requirements.txt** - Update Dependencies

**File:** `/Users/trevor/Github/pub_musings/pricecrawler/requirements.txt`

**Changes:**

```txt
# Keep
asyncio
aiohttp
beautifulsoup4

# Remove
requests

# Add
fastapi>=0.115.0
uvicorn[standard]>=0.32.0
openai>=1.55.0
python-multipart>=0.0.9
jinja2>=3.1.4
```

**Install command:**
```bash
pip install fastapi uvicorn[standard] openai python-multipart jinja2
pip uninstall requests
```

---

## Implementation Phases

### Phase 1: Foundation Setup
**Goal:** Install dependencies, set up FastAPI skeleton

1. Update requirements.txt
2. Run `pip install fastapi uvicorn[standard] openai python-multipart jinja2`
3. Create app.py with basic FastAPI structure and lifespan
4. Add simple `/status` endpoint to test

**Validation:** `uvicorn app:app --port 8000` should start successfully

---

### Phase 2: OpenAI SDK Migration
**Goal:** Migrate chatgpt.py to use OpenAI SDK

1. Add `CompletionResult` TypedDict to chatgpt.py
2. Install openai: `pip install openai`
3. Replace `chatCompletitions` with `async def chat_completions`
4. Test with standalone script to verify cost tracking matches

**Validation:** Cost calculations match previous implementation

---

### Phase 3: Update Importers (pricechecker.py)
**Goal:** Make pricechecker.py async and use new chat_completions

1. Rename `priceSummaries` → `async def price_summaries`
2. Update chatgpt import and extract `result['content']`
3. Remove `asyncio.run()` wrapper
4. Add type hints

**Validation:** Test price_summaries independently with `asyncio.run(price_summaries(...))`

---

### Phase 4: Facebook Loop Async Conversion
**Goal:** Convert facebook_loop.py to async

1. Add type hints to all functions
2. Convert functions to async one-by-one (bottom-up dependency order)
3. Replace `requests.get/post` with aiohttp
4. Update `priceSummaries` → `await price_summaries`
5. Rewrite `polling_loop` with error handling
6. Test `run_once` independently

**Validation:** `asyncio.run(run_once({}))` completes without errors

---

### Phase 5: FastAPI Routes Implementation
**Goal:** Implement all webhook routes in app.py

1. Copy GET routes from webhooks.py:
   - `/status`, `/app-validation`, `/page-validation`, `/pay`, `/privacy-policy`, `/terms`, `/deleted`, `/robots.txt`
2. Copy POST routes:
   - `/payment-callback`, `/delete-me`
3. Implement signature verification dependency
4. Add template rendering with Jinja2
5. Implement `/trigger-callback` to manually trigger polling

**Validation:** Test each route with curl/Postman

---

### Phase 6: Integration & Lifespan
**Goal:** Wire up polling loop with FastAPI lifespan

1. Import `polling_loop` from facebook_loop in app.py
2. Start polling task in lifespan startup
3. Cancel gracefully on shutdown
4. Test SSL certificate loading

**Validation:**
- Server starts with SSL on port 8443
- Background polling runs every 15 seconds
- Graceful shutdown cancels task

---

### Phase 7: End-to-End Testing
**Goal:** Verify complete integration

1. Configure Facebook webhook to point to new server
2. Send test message via Facebook Messenger
3. Verify:
   - Message received via webhook
   - Polling detects new message
   - ChatGPT called successfully
   - Price summary generated
   - Response posted to Facebook
4. Monitor logs for errors
5. Test payment flow
6. Test deletion endpoint with signature

**Validation:** Complete message round-trip works

---

### Phase 8: Cleanup
**Goal:** Remove deprecated code

1. Delete webhooks.py
2. Remove `main()` from facebook_loop.py
3. Update documentation/README
4. Git commit with migration complete

---

## Testing Strategy

### Unit Tests

**Test chatgpt.py:**
```python
import pytest
from chatgpt import chat_completions

@pytest.mark.asyncio
async def test_chat_completions_returns_structured_result():
    messages = [{"role": "user", "content": "Say hello"}]
    result = await chat_completions(messages, model="gpt-5-mini")

    assert 'content' in result
    assert 'usage' in result
    assert 'cost' in result
    assert result['usage']['total_tokens'] > 0
    assert result['cost']['total_cost'] > 0
```

**Test FastAPI routes:**
```python
from fastapi.testclient import TestClient
from app import app

client = TestClient(app)

def test_status_endpoint():
    response = client.get("/status")
    assert response.status_code == 200
    assert response.text == "1-AM-ALIVE"

def test_payment_callback_requires_body():
    response = client.post("/payment-callback")
    assert response.status_code == 400

def test_delete_me_requires_signature():
    response = client.post("/delete-me", json={"user_id": "123"})
    assert response.status_code in [401, 422]  # Unauthorized or validation error
```

### Integration Tests

**Test polling loop:**
```python
@pytest.mark.asyncio
async def test_run_once_processes_conversations():
    # Mock Facebook API responses
    # Call run_once({})
    # Assert OpenAI was called
    # Assert messages were posted
```

### Manual Testing Checklist

- [ ] `uvicorn app:app --port 8443 --ssl-keyfile key.pem --ssl-certfile cert.pem` starts
- [ ] GET /status returns "1-AM-ALIVE"
- [ ] GET /app-validation?hub.challenge=test returns "test"
- [ ] POST /payment-callback with valid JSON succeeds
- [ ] POST /delete-me without signature returns 401
- [ ] POST /delete-me with valid signature deletes user data
- [ ] Background polling runs every 15 seconds
- [ ] GET /trigger-callback triggers immediate polling
- [ ] Facebook messages trigger ChatGPT responses
- [ ] Cost tracking logs appear correctly
- [ ] Payment flow works end-to-end

---

## Potential Risks & Mitigations

### Risk 1: Missing `await` Keywords
**Impact:** Functions silently return coroutines instead of values
**Mitigation:** Use mypy with async plugin, comprehensive testing
**Detection:** Type errors, `RuntimeWarning: coroutine was never awaited`

### Risk 2: Background Task Crashes
**Impact:** Polling stops, no new messages processed
**Mitigation:** Wrap polling loop in try-except, log errors, continue
**Detection:** Monitor logs for exceptions, health check endpoint

### Risk 3: SSL Certificate Issues
**Impact:** Server won't start, webhooks fail
**Mitigation:** Test certificate loading before migration, document manual startup
**Detection:** Uvicorn startup errors

### Risk 4: OpenAI SDK Response Structure Changes
**Impact:** Code expects `result['content']` but structure differs
**Mitigation:** Test OpenAI SDK thoroughly in Phase 2, verify usage attributes
**Detection:** KeyError exceptions, missing cost data

### Risk 5: Facebook Signature Verification Breaks
**Impact:** POST /delete-me rejects valid requests
**Mitigation:** Exact same HMAC logic, test with real Facebook webhook
**Detection:** 401 errors on valid requests

### Risk 6: Concurrent aiohttp Sessions
**Impact:** Too many open connections, connection pool exhausted
**Mitigation:** Reuse single session per request, add connection limits
**Detection:** Connection errors, timeouts

---

## Rollback Plan

**If migration fails:**

1. **Keep old code on separate branch:**
   ```bash
   git checkout -b pre-fastapi-migration  # Save current state
   git checkout trunk
   # Do migration work on trunk
   git checkout pre-fastapi-migration  # Rollback if needed
   ```

2. **Run both servers in parallel (different ports):**
   - Old: `python facebook_loop.py` on port 8443
   - New: `uvicorn app:app --port 8444`
   - Update Facebook webhook URL to old server if needed

3. **Gradual cutover:**
   - Test new server with `/trigger-callback` manually
   - Monitor logs for errors (1 hour minimum)
   - Switch Facebook webhook URL when confident
   - Keep old server running for 24h as backup

---

## Success Criteria

✅ All FastAPI routes respond correctly
✅ Background polling runs continuously every 15 seconds
✅ Facebook messages trigger ChatGPT responses
✅ OpenAI cost tracking logs match previous format
✅ Payment flow works end-to-end
✅ User deletion with signature verification works
✅ No exceptions in logs during 1-hour test
✅ SSL/HTTPS works on port 8443
✅ Type hints on all modified functions
✅ Improved error handling prevents crashes
✅ webhooks.py deleted, no legacy code remains
