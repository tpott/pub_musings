# 002_initialize_project_structure

**Implements Task:** 1

## Objective
Create the foundational project structure for the subtitler service with Astro frontend and Go backend scaffolding.

## Current State
- v1/ directory contains previous Python-based subtitler work
- No frontend/ or backend/ directories exist
- README.md is minimal
- TASKS.jsonl and root plan (001_RALPH_SUBTITLER.md) exist

## Plan

### 1. Initialize Frontend (Astro)
- Create frontend/ directory
- Run `npm create astro@latest` to scaffold Astro project
- Configure for static site generation initially
- Add basic dependencies (will expand in later tasks)
- Verify package.json exists and has required scripts

### 2. Initialize Backend (Go)
- Create backend/ directory structure:
  ```
  backend/
  ├── cmd/
  │   └── server/
  │       └── main.go
  ├── internal/
  │   ├── auth/
  │   ├── transcribe/
  │   └── storage/
  └── go.mod
  ```
- Run `go mod init` with appropriate module name
- Create placeholder main.go with basic HTTP server
- Create empty package directories (auth, transcribe, storage) with placeholder files
- Verify go.mod and basic structure exist

### 3. Update Documentation
- Update README.md with:
  - Project description
  - Prerequisites (Go, Node.js, npm)
  - Directory structure overview
  - Links to more detailed docs
- Create INSTALL.md with:
  - Dependency installation instructions
  - Setup steps for frontend and backend
  - How to run locally
- Create UNIT_TESTS.md placeholder
- Create INTEGRATION_TESTS.md placeholder
- Create LINTERS.md placeholder

### 4. Update CLAUDE.md
- Add information about project structure
- Document commands for running frontend and backend
- Note any Go/Node version requirements

## Testing Acceptance Criteria
1. Verify frontend/ exists and contains:
   - package.json with scripts (dev, build)
   - src/ directory with Astro structure
   - Basic Astro configuration file
2. Verify backend/ exists and contains:
   - go.mod file
   - cmd/server/main.go (compiles successfully)
   - internal/auth/ directory
   - internal/transcribe/ directory
   - internal/storage/ directory
3. Verify documentation:
   - README.md has project overview
   - INSTALL.md has setup instructions
4. Run `cd frontend && npm install` successfully
5. Run `cd backend && go mod tidy` successfully
6. Run `cd backend && go build ./cmd/server` successfully

## Notes
- This is scaffolding only; no functional code beyond boilerplate
- Frontend and backend will be expanded in subsequent tasks
- Following the architecture from 001_RALPH_SUBTITLER.md
