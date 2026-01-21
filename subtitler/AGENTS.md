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

# Frontend Tests
# TODO

# Backend - run Go server
cd backend && go run main.go
# Runs on http://localhost:8080
# Note: go may not be in PATH, use full path, ex: /home/trevor/go/bin/go

# Backend Tests
cd backend && go test ./... -v

# End-to-end Tests
# TODO
```
