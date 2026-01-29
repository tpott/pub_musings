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
cd backend && go run main.go
# Runs on http://localhost:8080

# Backend Tests
cd backend && go test ./... -v
# Or use the script:
./scripts/test-backend.sh

# End-to-end Tests (requires servers running)
cd frontend && npm run test:e2e
# Or use the script:
./scripts/test-e2e.sh

# Linting (Go fmt/vet + frontend build)
./scripts/lint.sh

# Full Verification (lint + all unit tests)
./scripts/verify-all.sh
```

## Verification Scripts

All scripts are in the `scripts/` directory:

| Script | Purpose |
|--------|---------|
| `lint.sh` | Run Go fmt/vet and frontend build |
| `test-backend.sh` | Run backend Go tests |
| `test-frontend.sh` | Run frontend Vitest tests |
| `test-e2e.sh` | Run Playwright E2E tests |
| `verify-all.sh` | Run lint + backend + frontend tests |
| `pre-commit` | Git pre-commit hook (lint + unit tests) |
| `fetch-feedback.sh` | Fetch new user feedback from prod to FEEDBACK.md |

## Pre-commit Hook

To install the pre-commit hook:

```bash
ln -sf ../../scripts/pre-commit .git/hooks/pre-commit
```

The hook runs `lint.sh`, `test-backend.sh`, and `test-frontend.sh` before each commit.
E2E tests are skipped in pre-commit as they're too slow.
