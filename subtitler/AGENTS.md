## Override: Install Commands

The parent `pub_musings/CLAUDE.md` says "do not run install commands." This project's CLAUDE.md **overrides that rule** when running in the Ralph loop (`--dangerously-skip-permissions`).

When running as Ralph, you MAY:
- Run install commands (`npm install`, `go get`, etc.)
- Execute builds and tests
- Run the development servers

## Commands

```bash
# Frontend - install deps and run dev server
cd frontend && npm install && npm run dev
# Runs on http://localhost:4321

# Frontend Unit Tests
cd frontend && npm test
# Or use the script:
./scripts/test-frontend.sh

# Backend - run Go server
cd backend && go run .
# Runs on http://localhost:8080

# Backend Tests
cd backend && go test ./... -v
# Or use the script:
./scripts/test-backend.sh

# End-to-end Tests (requires servers running)
cd frontend && npm run test:e2e
# Or use the script:
./scripts/test-e2e.sh

# Linting (Go fmt/vet + golangci-lint + frontend build + file size check + doc sync)
./scripts/lint.sh

# Full Verification (lint + all unit tests)
./scripts/verify-all.sh
```

## Verification Scripts

All scripts are in the `scripts/` directory:

| Script | Purpose |
|--------|---------|
| `lint.sh` | Run Go fmt/vet, golangci-lint (if installed), frontend build, file size check, and doc sync |
| `lint-frontend-filesize.sh` | Check frontend source file sizes (error >4000, warn >1000 lines) |
| `lint-doc-sync.sh` | Check docs (deps, env vars, routes, rate limits) stay in sync with code |
| `test-backend.sh` | Run backend Go tests |
| `test-frontend.sh` | Run frontend Vitest tests |
| `test-e2e.sh` | Run Playwright E2E tests |
| `verify-all.sh` | Run lint + backend + frontend tests |
| `pre-commit` | Git pre-commit hook (lint + unit + E2E tests) |
| `fetch-feedback.py` | Fetch new user feedback from prod to FEEDBACK.md |
| `query-security-events.py` | Filter/analyze security events from journalctl |

## Dependency Policy

Every new dependency added to `go.mod` or `package.json` must be justified.
Before adding a dependency, update `docs/deps.md` with:

1. **What** the dependency is (name, version)
2. **Why** it is needed (what problem it solves)
3. **Alternatives considered** and why they were rejected

Prefer the Go standard library or existing dependencies over new ones.
Do not add a dependency for functionality that can be achieved with a small
amount of straightforward code.

## Decision Tracking

When making dependency choices, architectural decisions, or significant design
trade-offs, add an entry to the **Decisions** section of `LEARNINGS.md` using
the decision format (Context / Options considered / Decision / Outcome). This
applies to:

- Adding or replacing dependencies
- Choosing between implementation approaches
- Architectural patterns (e.g., state management, file organization)
- Tool/library selection for new features

## Pre-commit Hook

To install the pre-commit hook:

```bash
ln -sf ../../scripts/pre-commit .git/hooks/pre-commit
```

The hook runs `lint.sh`, `test-backend.sh`, `test-frontend.sh`, and `test-e2e.sh` before each commit.
