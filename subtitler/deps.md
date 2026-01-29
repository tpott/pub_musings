# Dependency Justification

Every dependency must have a justification. See `CLAUDE.md` for the policy.

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
| `vitest` | ^3.2.0 | Unit testing framework. Fast, Vite-native, supports mocking, fake timers, and TypeScript out of the box. 492 tests across 20 files. | Jest (slower startup, needs extra TS config), Mocha (more boilerplate, no built-in mocking) |
