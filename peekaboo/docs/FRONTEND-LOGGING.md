# Frontend Log Forwarding

During development, frontend `console.log/warn/error` messages can be forwarded to the Go backend's structured logger. This lets you see all frontend errors in a single terminal alongside backend logs.

## Setup

### 1. Enable on the Backend

Set the environment variable before starting the backend:

```bash
FORWARD_FRONTEND_LOGS=true go run main.go
```

This registers `POST /api/log` on the backend. Without this variable, the endpoint does not exist.

### 2. Enable on the Frontend

Create or edit `frontend/.env` (or `frontend/.env.development`):

```bash
VITE_FORWARD_LOGS=true
```

Then restart the dev server:

```bash
cd frontend && npm run dev
```

The frontend logger will now send each log message to `POST /api/log` as a fire-and-forget `fetch()` call.

## How It Works

The frontend `logger.ts` module (`logger.debug()`, `logger.info()`, `logger.warn()`, `logger.error()`) still writes to the browser console as usual. When forwarding is enabled, it also sends a `POST /api/log` request:

```json
{
  "level": "error",
  "message": "Peekaboo flow error: Error: Transcription failed"
}
```

The backend logs this with slog structured logging:

```
level=ERROR msg="Peekaboo flow error: Error: Transcription failed" source=frontend client_ip=127.0.0.1
```

All forwarded log messages include `source=frontend` so you can distinguish them from backend logs.

## Verifying It Works

### Step 1: Start Both Servers

Terminal 1 (backend):
```bash
cd backend
FORWARD_FRONTEND_LOGS=true LOG_LEVEL=debug go run main.go
```

Terminal 2 (frontend):
```bash
cd frontend
VITE_FORWARD_LOGS=true npm run dev
```

### Step 2: Introduce a Frontend Error

Edit `frontend/src/lib/peekaboo-flow.ts` and add a temporary error at the top of `startRecording()`:

```typescript
async startRecording(): Promise<void> {
  throw new Error('INTENTIONAL TEST ERROR - remove me');
  // ... rest of method
}
```

### Step 3: Trigger the Error

1. Open http://localhost:4321 in your browser
2. Click the microphone button
3. Check the backend terminal - you should see:

```
level=ERROR msg="Peekaboo flow error: Error: INTENTIONAL TEST ERROR - remove me" source=frontend client_ip=127.0.0.1
```

### Step 4: Clean Up

Remove the `throw new Error(...)` line you added.

## API Reference

### POST /api/log

Only available when `FORWARD_FRONTEND_LOGS=true` is set on the backend.

**Request:**
```json
{
  "level": "debug|info|warn|error",
  "message": "Log message text"
}
```

**Responses:**
- `204 No Content` - Log accepted
- `400 Bad Request` - Invalid level or empty message
- `413 Request Entity Too Large` - Body exceeds 2KB

## Security Notes

- This endpoint is **development-only**. Never enable `FORWARD_FRONTEND_LOGS=true` in production.
- The endpoint has a 2KB body size limit to prevent abuse.
- No rate limiting is applied (it's dev-only and would interfere with high-frequency logging).
- The endpoint does not exist at all when the env var is not set, so there is zero attack surface in production.
