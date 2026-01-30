# Dependency Justification

Every dependency must have a justification. See `../CLAUDE.md` for the policy.

## Backend (go.mod)

### Direct Dependencies

| Dependency | Version | Justification | Alternatives Considered |
|---|---|---|---|
| `filippo.io/age` | v1.2.1 | File-at-rest encryption using X25519 + ChaCha20-Poly1305. Used for encrypting uploaded media and thumbnails. Authored by Filippo Valsorda (Go cryptography maintainer). | NaCl/libsodium (lower-level, more code to write), GPG (heavier, worse Go API) |
| `github.com/mattn/go-sqlite3` | v1.14.33 | CGo SQLite3 driver for `database/sql`. All persistent state (users, sessions, videos, transcriptions) is stored in SQLite. | `modernc.org/sqlite` (pure Go, slower), PostgreSQL (overkill for single-server deployment) |
| `github.com/piglig/go-qr` | v0.2.6 | Server-side QR code generation for TOTP 2FA setup. Generates PNG data-URLs embedded in API responses. | `github.com/skip2/go-qrcode` (previously used, replaced), external QR API (privacy concern, requires CSP exception) |
| `github.com/prometheus/client_golang` | v1.23.2 | Prometheus metrics (counters, histograms, gauges) for HTTP requests, transcription jobs, uploads, and sessions. Exposes `/metrics` endpoint. | `expvar` (stdlib, limited metric types), OpenTelemetry (heavier, more complex setup) |
| `github.com/resend/resend-go/v2` | v2.28.0 | Official Resend SDK for transactional email (verification, password reset, magic link). | `net/smtp` (requires SMTP server management), SendGrid/Mailgun (different provider choice) |
| `golang.org/x/crypto` | v0.24.0 | `bcrypt` subpackage for password hashing (cost 12) and recovery code hashing (cost 10). Part of the Go extended standard library. | `argon2` (x/crypto also provides it, bcrypt is simpler and sufficient), scrypt (less common in Go ecosystem) |

### Indirect Dependencies

Indirect dependencies are pulled in by direct dependencies and do not require
individual justification. They are managed by `go mod tidy`.

## Frontend (package.json)

### Dependencies

| Dependency | Version | Justification | Alternatives Considered |
|---|---|---|---|
| `astro` | ^5.16.11 | Core framework. Static-site generator with file-based routing, component islands, and built-in dev server with API proxy. | Next.js (heavier, SSR-focused), plain HTML (no component model, no dev tooling) |
| `zod` | ^3.25.76 | Runtime validation of API response shapes. Provides type-safe parsing with TypeScript inference for all backend API contracts. | Manual validation (verbose, no type inference), `io-ts` (heavier, less ergonomic), `yup` (weaker TS inference) |

### Dev Dependencies

| Dependency | Version | Justification | Alternatives Considered |
|---|---|---|---|
| `@playwright/test` | ^1.57.0 | End-to-end browser testing. Tests full user flows (auth, upload, video management) in Chromium with automatic server startup. | Cypress (slower, heavier), Selenium (more setup, less ergonomic) |
| `vitest` | ^3.2.0 | Unit testing framework. Fast, Vite-native, supports mocking, fake timers, and TypeScript out of the box. 498 tests across 20 files. | Jest (slower startup, needs extra TS config), Mocha (more boilerplate, no built-in mocking) |

### CDN Scripts

| Dependency | URL | Justification | Alternatives Considered |
|---|---|---|---|
| hCaptcha | `https://js.hcaptcha.com/1/api.js?render=explicit` | Client-side CAPTCHA widget for bot protection on registration and login forms. Loaded dynamically only when CAPTCHA is enabled (`CAPTCHA_SITE_KEY` set). Backend verifies tokens via `https://hcaptcha.com/siteverify`. | reCAPTCHA (Google privacy concerns), Turnstile (Cloudflare-only), self-hosted challenge (less effective) |

## System Dependencies

Runtime binaries invoked by the backend via `exec.Command`. See [INSTALL.md](../INSTALL.md) for installation instructions.

| Dependency | Required | Used By | Justification | Alternatives Considered |
|---|---|---|---|---|
| `ffmpeg` | Yes | `backend/audio/audio.go`, `backend/main.go` | Audio extraction from video, subtitle burning into video, format conversion. Core to transcription and export workflows. | GStreamer (less common, worse CLI), pure Go decoders (incomplete codec support) |
| `ffprobe` | Yes (ships with ffmpeg) | `backend/audio/audio.go`, `backend/language/language.go` | Video validation, duration detection, embedded subtitle track discovery, audio language metadata extraction. | `mediainfo` (less common), parsing headers manually (fragile) |
| `whisper-cli` | Yes (CLI mode) | `backend/main.go` | Speech-to-text transcription via whisper.cpp. Spawned per-job. Used when `USE_WHISPER_SERVER=false`. | Cloud STT APIs (cost, privacy), Vosk (lower accuracy), whisper-server (alternative mode) |
| `whisper-server` | Yes (server mode) | `backend/main.go` | HTTP API for whisper.cpp transcription. Keeps model in memory for faster repeated transcriptions. Used when `USE_WHISPER_SERVER=true`. | whisper-cli (simpler but slower per-job), cloud APIs (cost, privacy) |
| `sqlite3` | Optional | Admin/backup tasks | CLI for manual database queries, integrity checks, and backups. Not invoked by the application at runtime (Go driver handles all DB access). | Database GUI tools, Go-based backup scripts |

## External Services

Third-party APIs and services the application communicates with at runtime.

| Service | Required | Justification | Alternatives Considered |
|---|---|---|---|
| [Resend](https://resend.com) | Optional (email features) | Transactional email delivery for verification, password reset, and magic link emails. Used via `github.com/resend/resend-go/v2` SDK. Disabled when `EMAIL_ENABLED=false`. | SendGrid (heavier SDK), Mailgun (similar), `net/smtp` with own SMTP server (operational overhead) |
| [hCaptcha](https://www.hcaptcha.com) | Optional (bot protection) | Server-side CAPTCHA token verification. Backend POSTs to `https://hcaptcha.com/siteverify`. Disabled when `CAPTCHA_SITE_KEY`/`CAPTCHA_SECRET_KEY` are not set. | reCAPTCHA (Google privacy concerns), Turnstile (Cloudflare-specific) |

## Deployment Dependencies

Tools used for production deployment but not required for local development. See [specs/deployment.md](../specs/deployment.md) and [specs/human_deploy.md](../specs/human_deploy.md).

| Dependency | Justification |
|---|---|
| [Caddy](https://caddyserver.com) | Reverse proxy serving static frontend and proxying `/api/*` to the Go backend. Automatic TLS, security headers. |
| [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) (`cloudflared`) | Routes external traffic to the application without exposing ports publicly. Free tier limits request bodies to 100MB. |
| [sops](https://github.com/getsops/sops) | Encrypts/decrypts `secrets.enc.yaml` containing environment variables for deployment. Uses age keys. |
| [systemd](https://systemd.io) | Service management for backend, Caddy, and cloudflared processes. |
| [fail2ban](https://www.fail2ban.org) | Intrusion prevention for SSH protection. Recommended in [SECURITY_CHECKLIST.md](SECURITY_CHECKLIST.md). |

## Build-Time Dependencies

| Dependency | Version | Justification |
|---|---|---|
| [Go](https://go.dev) | 1.23+ (toolchain 1.24) | Compiles the backend. CGo required for sqlite3 driver (needs C compiler / `build-essential`). |
| [Node.js](https://nodejs.org) / npm | 18+ | Builds the frontend (`astro build`), runs dev server, and executes tests. |
| Noto fonts (optional) | — | Required for burning Indic script subtitles into video. Without them, non-Latin characters render as empty boxes. See [INSTALL.md](../INSTALL.md). |

## Development Tools

| Tool | Version | Justification | Alternatives Considered |
|---|---|---|---|
| [golangci-lint](https://golangci-lint.run) | v2.8.0 | Runs `funlen` (function length) and `revive` (`file-length-limit`) linters to enforce size limits on Go functions and files. Prevents unbounded growth after the main.go split. Configured in `backend/.golangci.yml`. | `go vet` only (no length checks), standalone `revive` binary (less linter aggregation), custom shell scripts with `wc -l` (fragile, no per-function granularity) |
