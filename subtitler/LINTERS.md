# Linters Guide

This guide covers setting up and running linters for the Subtitler project.

## Backend (Go)

### golangci-lint

[golangci-lint](https://golangci-lint.run/) is the recommended linter aggregator for Go projects.

**Installation:**

```bash
# Linux/macOS (recommended)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.62.2

# macOS (Homebrew)
brew install golangci-lint

# Go install (not recommended for CI)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
```

**Verify installation:**
```bash
golangci-lint --version
```

**Run linter:**
```bash
cd backend
golangci-lint run
```

**Run with fixes:**
```bash
cd backend
golangci-lint run --fix
```

### go fmt

The built-in Go formatter should be run before commits.

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

### Configuration (Optional)

Create `.golangci.yml` in the backend directory for custom configuration:

```yaml
# backend/.golangci.yml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports

linters-settings:
  gofmt:
    simplify: true

issues:
  exclude-use-default: false
```

## Frontend (TypeScript/Astro)

### ESLint

[ESLint](https://eslint.org/) is the standard linter for JavaScript/TypeScript projects.

**Installation:**
```bash
cd frontend
npm install -D eslint @eslint/js typescript-eslint eslint-plugin-astro
```

**Create configuration:**

Create `eslint.config.mjs` in the frontend directory:

```javascript
// frontend/eslint.config.mjs
import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import astro from 'eslint-plugin-astro';

export default [
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...astro.configs.recommended,
  {
    ignores: ['dist/**', 'node_modules/**', '.astro/**']
  }
];
```

**Add scripts to package.json:**
```json
{
  "scripts": {
    "lint": "eslint .",
    "lint:fix": "eslint . --fix"
  }
}
```

**Run linter:**
```bash
cd frontend
npm run lint
```

**Run with fixes:**
```bash
cd frontend
npm run lint:fix
```

### Prettier (Optional)

[Prettier](https://prettier.io/) is an opinionated code formatter.

**Installation:**
```bash
cd frontend
npm install -D prettier eslint-config-prettier
```

**Create configuration:**

Create `.prettierrc` in the frontend directory:

```json
{
  "semi": true,
  "singleQuote": true,
  "tabWidth": 2,
  "trailingComma": "es5"
}
```

**Add scripts to package.json:**
```json
{
  "scripts": {
    "format": "prettier --write .",
    "format:check": "prettier --check ."
  }
}
```

**Run formatter:**
```bash
cd frontend
npm run format
```

## Quick Reference

| Tool | Language | Command |
|------|----------|---------|
| go fmt | Go | `cd backend && go fmt ./...` |
| go vet | Go | `cd backend && go vet ./...` |
| golangci-lint | Go | `cd backend && golangci-lint run` |
| ESLint | TypeScript/Astro | `cd frontend && npm run lint` |
| Prettier | TypeScript/Astro | `cd frontend && npm run format` |

## Pre-commit Hooks (Optional)

Use [pre-commit](https://pre-commit.com/) or [Husky](https://typicode.github.io/husky/) to run linters automatically before commits.

**Example with Husky:**
```bash
cd frontend
npm install -D husky lint-staged
npx husky init
echo "cd frontend && npx lint-staged" > .husky/pre-commit
```

Add to `package.json`:
```json
{
  "lint-staged": {
    "*.{ts,tsx,astro}": ["eslint --fix", "prettier --write"],
    "*.{json,md}": ["prettier --write"]
  }
}
```

## See Also

- [README.md](README.md) - Project overview
- [TESTING.md](TESTING.md) - Running tests
- [INSTALL.md](INSTALL.md) - Installing dependencies
