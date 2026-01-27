# Environment Variables Reference

Complete reference for all environment variables used by the Subtitler application.

## Quick Reference

| Category | Variables |
|----------|-----------|
| Server | `PORT` |
| Storage | `UPLOAD_DIR`, `DB_PATH`, `KEY_PATH`, `MAX_UPLOAD_SIZE` |
| Whisper | `WHISPER_SERVER_URL`, `USE_WHISPER_SERVER`, `WHISPER_MODEL` |
| Email | `RESEND_API_KEY`, `EMAIL_FROM`, `EMAIL_ENABLED`, `APP_URL` |
| Security | `HTTPS_ONLY`, `TRUST_PROXY`, `ENCRYPTION_ENABLED`, `LOG_VERBOSE`, `CSRF_SECRET`, `CSRF_SECRET_PATH`, `CAPTCHA_SITE_KEY`, `CAPTCHA_SECRET_KEY` |
| Admin | `INITIAL_ADMIN_EMAIL` |
| Rate Limits | `AUTH_RATE_LIMIT`, `PASSWORD_RESET_RATE_LIMIT`, `UPLOAD_RATE_LIMIT`, `TRANSCRIBE_RATE_LIMIT`, `BURN_RATE_LIMIT`, `DOWNLOAD_RATE_LIMIT`, `SCRIPT_RATE_LIMIT`, `METRICS_RATE_LIMIT`, `USER_RATE_LIMIT` |
| Debugging | `LOG_SLOW_QUERIES`, `SLOW_QUERY_THRESHOLD_MS` |
| Maintenance | `DB_MAINTENANCE_INTERVAL` |
| Subtitles | `SUBTITLE_FONT` |

## Server Configuration

### PORT

| Property | Value |
|----------|-------|
| Default | `8080` |
| Required | No |
| Example | `PORT=8060` |

HTTP port the backend server listens on. In production behind Caddy, use a non-standard port like `8060`.

## Storage Configuration

### UPLOAD_DIR

| Property | Value |
|----------|-------|
| Default | `uploads` |
| Required | No |
| Example | `UPLOAD_DIR=/opt/subtitler/uploads` |

Directory where uploaded video files are stored. Files are encrypted at rest (when encryption is enabled).

### DB_PATH

| Property | Value |
|----------|-------|
| Default | `data/subtitler.db` |
| Required | No |
| Example | `DB_PATH=/opt/subtitler/data/subtitler.db` |

Path to the SQLite database file. Parent directory is created automatically if it doesn't exist.

### KEY_PATH

| Property | Value |
|----------|-------|
| Default | `data/age.key` |
| Required | No |
| Example | `KEY_PATH=/opt/subtitler/data/age.key` |

Path to the age encryption private key file. If the file doesn't exist, a new key is generated. **Critical:** Back up this file securely - without it, encrypted files are unrecoverable.

### MAX_UPLOAD_SIZE

| Property | Value |
|----------|-------|
| Default | `500M` (500 MB) |
| Required | No |
| Format | Number with optional suffix: `K` (KB), `M` (MB), `G` (GB) |
| Example | `MAX_UPLOAD_SIZE=1G` |

Maximum allowed file size for video uploads. Note: Cloudflare's free tier limits uploads to 100MB through tunnels.

## Whisper Configuration

The backend supports two transcription modes: CLI mode (spawns whisper-cli) and Server mode (HTTP API).

### WHISPER_SERVER_URL

| Property | Value |
|----------|-------|
| Default | `http://127.0.0.1:8765` |
| Required | No |
| Example | `WHISPER_SERVER_URL=http://10.0.2.2:8765` |

URL of the whisper-server HTTP API. Setting this variable enables server mode automatically. For qemu VMs, use `http://10.0.2.2:8765` to reach the host.

### USE_WHISPER_SERVER

| Property | Value |
|----------|-------|
| Default | `false` |
| Required | No |
| Values | `true`, `false` |
| Example | `USE_WHISPER_SERVER=true` |

Set to `true` to use whisper-server with the default URL. Not needed if `WHISPER_SERVER_URL` is set.

### WHISPER_MODEL

| Property | Value |
|----------|-------|
| Default | `$HOME/Github/whisper.cpp/models/ggml-medium.bin` |
| Required | No (CLI mode only) |
| Example | `WHISPER_MODEL=/path/to/ggml-large-v3-turbo.bin` |

Path to the Whisper model file. Only used in CLI mode (when server mode is disabled).

## Email Configuration

Email functionality requires the Resend API for transactional emails (verification, password reset, magic links).

### RESEND_API_KEY

| Property | Value |
|----------|-------|
| Default | *(none)* |
| Required | Yes (for email features) |
| Format | Starts with `re_` |
| Example | `RESEND_API_KEY=re_123abc...` |

API key from [Resend](https://resend.com). Required for email verification, password reset, and magic link authentication.

### EMAIL_FROM

| Property | Value |
|----------|-------|
| Default | `noreply@subtitler.app` |
| Required | No |
| Example | `EMAIL_FROM=noreply@yourdomain.com` |

Sender email address for outgoing emails. Must be a verified domain in Resend.

### EMAIL_ENABLED

| Property | Value |
|----------|-------|
| Default | `true` |
| Required | No |
| Values | `true`, `false` |
| Example | `EMAIL_ENABLED=false` |

Set to `false` to disable email sending. Useful for development or when email features aren't needed.

### APP_URL

| Property | Value |
|----------|-------|
| Default | `http://localhost:4321` |
| Required | No |
| Example | `APP_URL=https://subtitler.example.com` |

Base URL for links in emails (password reset, email verification, magic links). Must match the public URL where users access the application.

## Security Configuration

### HTTPS_ONLY

| Property | Value |
|----------|-------|
| Default | `false` |
| Required | No |
| Values | `true`, `1`, `false`, `0` |
| Example | `HTTPS_ONLY=true` |

When enabled, session cookies have the `Secure` flag set, meaning they're only sent over HTTPS connections. **Enable in production** when using HTTPS.

### TRUST_PROXY

| Property | Value |
|----------|-------|
| Default | `false` |
| Required | No |
| Values | `true`, `1`, `false`, `0` |
| Example | `TRUST_PROXY=true` |

Controls whether the rate limiter trusts `X-Forwarded-For` and `X-Real-IP` headers.

**Security Warning:** Only enable when running behind a trusted reverse proxy (Caddy, nginx, load balancer). If enabled without a proxy, attackers can spoof their IP to bypass rate limiting.

### ENCRYPTION_ENABLED

| Property | Value |
|----------|-------|
| Default | `true` |
| Required | No |
| Values | `true`, `false`, `0` |
| Example | `ENCRYPTION_ENABLED=false` |

Controls whether uploaded files are encrypted at rest using age encryption.

- **Enabled (default):** Files are encrypted before storage
- **Disabled:** Files stored unencrypted (development only)

Note: Disabling only affects new files. Existing encrypted files (`.age` extension) can still be read.

### LOG_VERBOSE

| Property | Value |
|----------|-------|
| Default | `false` |
| Required | No |
| Values | `true`, `1`, `false`, `0` |
| Example | `LOG_VERBOSE=true` |

Controls whether detailed error messages are returned to clients.

- **Disabled (default):** User-friendly error messages are returned without internal details. Safe for production.
- **Enabled:** Detailed error messages including paths, server errors, and stack traces are returned. **Development only.**

**Security Warning:** Never enable in production. Detailed errors can leak:
- File system paths (`/opt/subtitler/uploads/...`)
- Database error details
- Internal server IPs and ports
- API keys in error messages

### CSRF_SECRET

| Property | Value |
|----------|-------|
| Default | *(auto-generated and persisted to `data/csrf.key`)* |
| Required | No |
| Example | `CSRF_SECRET=your-32-byte-secret-here` |

HMAC key for generating CSRF tokens. Secret is determined in this priority order:
1. **Environment variable:** If `CSRF_SECRET` is set, use it directly
2. **Persisted file:** If `data/csrf.key` exists, load secret from it
3. **Generate new:** Create a random 32-byte secret and save to `data/csrf.key`

This ensures CSRF tokens remain valid across server restarts without requiring manual configuration.

### CSRF_SECRET_PATH

| Property | Value |
|----------|-------|
| Default | `data/csrf.key` |
| Required | No |
| Example | `CSRF_SECRET_PATH=/opt/subtitler/data/csrf.key` |

Path to the CSRF secret file. Only used when `CSRF_SECRET` env var is not set. The file is created automatically with restrictive permissions (0600) if it doesn't exist.

### CAPTCHA_SITE_KEY

| Property | Value |
|----------|-------|
| Default | *(none)* |
| Required | No (but required with CAPTCHA_SECRET_KEY) |
| Example | `CAPTCHA_SITE_KEY=10000000-ffff-ffff-ffff-000000000001` |

Public hCaptcha site key. Both `CAPTCHA_SITE_KEY` and `CAPTCHA_SECRET_KEY` must be set to enable CAPTCHA protection. Get keys from [hCaptcha](https://www.hcaptcha.com/).

### CAPTCHA_SECRET_KEY

| Property | Value |
|----------|-------|
| Default | *(none)* |
| Required | No (but enables CAPTCHA when set) |
| Example | `CAPTCHA_SECRET_KEY=0x...` |

Secret hCaptcha key for server-side verification. When set (along with `CAPTCHA_SITE_KEY`), CAPTCHA protection is enabled for registration and login forms. This helps prevent automated bot attacks.

**Behavior:**
- **Not set:** CAPTCHA is disabled; registration and login work without CAPTCHA
- **Set:** CAPTCHA required on registration and initial login (not required for TOTP code entry)

## Rate Limit Configuration

All rate limits use the format `count/window` where:
- `count` is the number of requests allowed
- `window` is the time period: `s`, `sec`, `min`, `minute`, `h`, `hr`, `hour`, or Go duration (e.g., `15m`, `1h30m`)

Rate limits are per-IP address using a sliding window. When exceeded, returns HTTP 429 with `Retry-After` header.

### AUTH_RATE_LIMIT

| Property | Value |
|----------|-------|
| Default | `5/min` |
| Required | No |
| Example | `AUTH_RATE_LIMIT=10/min` |

Rate limit for authentication endpoints: login, register, TOTP verify, magic link.

### PASSWORD_RESET_RATE_LIMIT

| Property | Value |
|----------|-------|
| Default | `3/15m` |
| Required | No |
| Example | `PASSWORD_RESET_RATE_LIMIT=5/15m` |

Rate limit for password reset requests. Lower limit to prevent email spam.

### UPLOAD_RATE_LIMIT

| Property | Value |
|----------|-------|
| Default | `10/min` |
| Required | No |
| Example | `UPLOAD_RATE_LIMIT=20/hour` |

Rate limit for file uploads.

### TRANSCRIBE_RATE_LIMIT

| Property | Value |
|----------|-------|
| Default | `5/min` |
| Required | No |
| Example | `TRANSCRIBE_RATE_LIMIT=10/min` |

Rate limit for transcription requests. Consider whisper-server capacity when adjusting.

### BURN_RATE_LIMIT

| Property | Value |
|----------|-------|
| Default | `2/min` |
| Required | No |
| Example | `BURN_RATE_LIMIT=5/min` |

Rate limit for subtitle burning (CPU-intensive ffmpeg operation).

### DOWNLOAD_RATE_LIMIT

| Property | Value |
|----------|-------|
| Default | `30/min` |
| Required | No |
| Example | `DOWNLOAD_RATE_LIMIT=60/min` |

Rate limit for file downloads (video, thumbnail, burned video). Prevents bandwidth abuse and CPU exhaustion from repeated decryption operations.

### SCRIPT_RATE_LIMIT

| Property | Value |
|----------|-------|
| Default | `10/min` |
| Required | No |
| Example | `SCRIPT_RATE_LIMIT=20/min` |

Rate limit for script detection and conversion endpoints.

### METRICS_RATE_LIMIT

| Property | Value |
|----------|-------|
| Default | `10/min` |
| Required | No |
| Example | `METRICS_RATE_LIMIT=30/min` |

Rate limit for the `/metrics` endpoint (Prometheus format). Prevents reconnaissance attacks from unauthenticated sources probing the metrics endpoint.

### USER_RATE_LIMIT

| Property | Value |
|----------|-------|
| Default | `60/min` |
| Required | No |
| Example | `USER_RATE_LIMIT=120/min` |

Per-user rate limit for authenticated requests across all endpoints. Applied in addition to per-IP rate limiting. When authenticated users exceed this limit, they receive HTTP 429 with `X-RateLimit-Limit` and `X-RateLimit-Remaining` headers. Anonymous requests are not affected (they rely on IP-based limiting).

## Monitoring Configuration

### METRICS_API_KEY

| Property | Value |
|----------|-------|
| Default | (empty - requires admin authentication) |
| Required | No |
| Format | String (any secure random value) |
| Example | `METRICS_API_KEY=your-secure-api-key-here` |

API key for accessing the `/metrics` endpoint (Prometheus format). If set, requests with this key in the `X-Metrics-API-Key` header or `api_key` query parameter are allowed. If not set, session-based authentication with **admin role** is required.

**Access control:**
1. **API Key (recommended for monitoring systems):** If `METRICS_API_KEY` is set and the request includes a matching key, access is granted immediately
2. **Admin Session:** If no API key is configured or provided, the user must be authenticated with an admin role (see `INITIAL_ADMIN_EMAIL`)

**Example usage:**
```bash
# With API key header
curl -H "X-Metrics-API-Key: your-key" https://example.com/metrics

# With query parameter
curl "https://example.com/metrics?api_key=your-key"
```

**Exposed metrics:**
- `http_requests_total` - HTTP requests by method, path, and status
- `http_request_duration_seconds` - Request duration histogram
- `transcription_total` - Transcription jobs by status (started/completed/failed)
- `transcription_duration_seconds` - Transcription duration histogram
- `active_sessions_total` - Current active user sessions
- `uploads_bytes_total` - Total bytes uploaded
- `uploads_total` - Upload count by status (success/failed)

## Admin Configuration

### INITIAL_ADMIN_EMAIL

| Property | Value |
|----------|-------|
| Default | (empty) |
| Required | No |
| Format | Valid email address |
| Example | `INITIAL_ADMIN_EMAIL=admin@example.com` |

Email address of the user to promote to admin role on server startup. If the user exists, their role is set to `admin`. If the user doesn't exist yet, no action is taken (no error).

**Use case:** Bootstrapping the first admin user for a new deployment. Set this to your email, register an account, and you'll automatically become an admin.

**Notes:**
- The promotion happens on every server start, so you can leave this configured
- If the user doesn't exist, a log message is written but no error occurs
- To promote additional admins, use database operations directly:
  ```bash
  sqlite3 data/subtitler.db "UPDATE users SET role = 'admin' WHERE email = 'user@example.com'"
  ```

## Debugging Configuration

### LOG_SLOW_QUERIES

| Property | Value |
|----------|-------|
| Default | `false` |
| Required | No |
| Values | `true`, `1`, `false`, `0` |
| Example | `LOG_SLOW_QUERIES=true` |

Enable database query performance logging. When enabled, queries exceeding the threshold are logged as warnings with timing information.

**Logged information:**
- Operation type (SELECT, INSERT, UPDATE, DELETE)
- Table name
- Execution time in milliseconds
- Rows affected (for write operations)

**Example log output:**
```
WARN Slow query detected operation=SELECT table=videos duration_ms=150 rows_affected=0
```

### SLOW_QUERY_THRESHOLD_MS

| Property | Value |
|----------|-------|
| Default | `100` (100 milliseconds) |
| Required | No |
| Format | Integer (milliseconds) |
| Example | `SLOW_QUERY_THRESHOLD_MS=50` |

Threshold in milliseconds for logging slow queries. Only used when `LOG_SLOW_QUERIES=true`. Queries taking longer than this threshold are logged as warnings.

**Use cases:**
- Set to `50` for stricter monitoring
- Set to `200` for less verbose logging
- Set to `1` during debugging to log all queries

## Maintenance Configuration

### DB_MAINTENANCE_INTERVAL

| Property | Value |
|----------|-------|
| Default | `24h` |
| Required | No |
| Format | Go duration (e.g., `12h`, `30m`) or `0`/`disabled` |
| Example | `DB_MAINTENANCE_INTERVAL=12h` |

Interval for automatic SQLite maintenance (VACUUM and ANALYZE). Set to `0` or `disabled` to disable.

## Subtitle Configuration

### SUBTITLE_FONT

| Property | Value |
|----------|-------|
| Default | (empty - uses system default) |
| Required | No |
| Format | Font name or path |
| Example | `SUBTITLE_FONT=Noto Sans Devanagari` |

Font to use when burning subtitles into video. Required for proper rendering of non-Latin scripts like Hindi (Devanagari), Tamil, Telugu, etc.

**Font name:** Use the font family name as recognized by fontconfig:
```bash
SUBTITLE_FONT="Noto Sans Devanagari"
```

**Font path:** Use the full path to a font file:
```bash
SUBTITLE_FONT="/usr/share/fonts/truetype/noto/NotoSansDevanagari-Regular.ttf"
```

**Why this is needed:**
- Without this setting, non-Latin characters may appear as boxes (□) in burned subtitles
- The downloaded SRT/VTT files contain the correct text - only the burn feature is affected
- Install fonts that support your target scripts (see INSTALL.md for details)

## Configuration Examples

### Development

```bash
# Minimal development setup
export PORT=8080
export ENCRYPTION_ENABLED=false
export EMAIL_ENABLED=false
export USE_WHISPER_SERVER=true
```

### Production (systemd)

```ini
# /etc/systemd/system/subtitler.service
[Service]
Environment=PORT=8060
Environment=WHISPER_SERVER_URL=http://10.0.2.2:8765
Environment=UPLOAD_DIR=/opt/subtitler/uploads
Environment=DB_PATH=/opt/subtitler/data/subtitler.db
Environment=KEY_PATH=/opt/subtitler/data/age.key
Environment=MAX_UPLOAD_SIZE=1G
Environment=HTTPS_ONLY=true
Environment=TRUST_PROXY=true
Environment=DB_MAINTENANCE_INTERVAL=24h
Environment=RESEND_API_KEY=re_...
Environment=APP_URL=https://subtitler.example.com
Environment=EMAIL_FROM=noreply@subtitler.example.com
```

### Production (.env file)

```bash
# .env (load with export $(cat .env | xargs))
PORT=8060
WHISPER_SERVER_URL=http://10.0.2.2:8765
UPLOAD_DIR=/opt/subtitler/uploads
DB_PATH=/opt/subtitler/data/subtitler.db
KEY_PATH=/opt/subtitler/data/age.key
MAX_UPLOAD_SIZE=1G
HTTPS_ONLY=true
TRUST_PROXY=true
DB_MAINTENANCE_INTERVAL=24h
RESEND_API_KEY=re_...
APP_URL=https://subtitler.example.com
EMAIL_FROM=noreply@subtitler.example.com
CSRF_SECRET=your-persistent-csrf-secret
CAPTCHA_SITE_KEY=your-hcaptcha-site-key
CAPTCHA_SECRET_KEY=your-hcaptcha-secret-key
```

## See Also

- [../backend/README.md](../backend/README.md) - Backend overview with quick reference
- [../specs/deployment.md](../specs/deployment.md) - Production deployment guide
- [./RATE_LIMITS.md](./RATE_LIMITS.md) - Rate limiting details
- [../specs/auth.md](../specs/auth.md) - Authentication specification
- [../specs/encryption.md](../specs/encryption.md) - Encryption specification
