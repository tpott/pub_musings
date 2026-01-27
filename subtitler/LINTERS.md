# Linters Guide

This guide covers the linting setup for the Subtitler project.

## Current Linting Setup

The project uses minimal linting to keep the toolchain simple. All linting is handled by `scripts/lint.sh`:

```bash
./scripts/lint.sh
```

This runs:
- **Backend**: `go fmt` and `go vet` for formatting and static analysis
- **Frontend**: `npm run build` (TypeScript compilation catches type errors)

## Backend (Go)

### go fmt

The built-in Go formatter ensures consistent code style.

```bash
cd backend
go fmt ./...
```

### go vet

The built-in Go static analyzer catches common mistakes.

```bash
cd backend
go vet ./...
```

### golangci-lint (Optional - NOT CURRENTLY USED)

[golangci-lint](https://golangci-lint.run/) is available for more comprehensive linting if needed.

**Installation:**
```bash
# Linux/macOS (recommended)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.62.2

# macOS (Homebrew)
brew install golangci-lint
```

**Run linter:**
```bash
cd backend
golangci-lint run
```

**Note:** The project does not currently have a `.golangci.yml` configuration file. Add one if you want to customize linter rules.

## Frontend (TypeScript/Astro)

### TypeScript Compilation

TypeScript errors are caught during the build process:

```bash
cd frontend
npm run build
```

This performs full type checking via Astro's build process.

### ESLint (Optional - NOT CURRENTLY USED)

ESLint is NOT currently integrated into the project. If you want to add it:

**Installation:**
```bash
cd frontend
npm install -D eslint @eslint/js typescript-eslint eslint-plugin-astro
```

**Create `eslint.config.mjs`:**
```javascript
import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import astro from 'eslint-plugin-astro';

export default [
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...astro.configs.recommended,
  { ignores: ['dist/**', 'node_modules/**', '.astro/**'] }
];
```

**Add to `package.json`:**
```json
{
  "scripts": {
    "lint": "eslint .",
    "lint:fix": "eslint . --fix"
  }
}
```

### Prettier (Optional - NOT CURRENTLY USED)

Prettier is NOT currently integrated. If you want to add it:

**Installation:**
```bash
cd frontend
npm install -D prettier
```

**Create `.prettierrc`:**
```json
{
  "semi": true,
  "singleQuote": true,
  "tabWidth": 2,
  "trailingComma": "es5"
}
```

## Pre-commit Hook

The project uses a shell script pre-commit hook (NOT Husky):

```bash
# Install the hook
ln -sf ../../scripts/pre-commit .git/hooks/pre-commit
```

The hook runs `lint.sh`, `test-backend.sh`, and `test-frontend.sh` before each commit.

See `scripts/pre-commit` for details.

## Quick Reference

| Tool | Status | Command |
|------|--------|---------|
| go fmt | **IN USE** | `go fmt ./...` |
| go vet | **IN USE** | `go vet ./...` |
| npm build | **IN USE** | `npm run build` (type checking) |
| golangci-lint | Optional | `golangci-lint run` |
| ESLint | Not integrated | (see setup above) |
| Prettier | Not integrated | (see setup above) |
| Husky | Not used | (shell script hook instead) |

## See Also

- [README.md](README.md) - Project overview
- [TESTING.md](TESTING.md) - Running tests
- [INSTALL.md](INSTALL.md) - Installing dependencies
