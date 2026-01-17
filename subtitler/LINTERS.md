# Linters and Code Quality

This document describes the linting and code quality tools for the Subtitler project.

## Frontend Linting

### ESLint
Configuration: `frontend/.eslintrc.js` (to be created)

Run linter:
```bash
cd frontend
npm run lint
```

Auto-fix issues:
```bash
cd frontend
npm run lint:fix
```

### TypeScript
Type checking:
```bash
cd frontend
npm run type-check
# Or: npx tsc --noEmit
```

### Prettier (Code Formatting)
Configuration: `frontend/.prettierrc` (to be created)

Format code:
```bash
cd frontend
npm run format
```

Check formatting:
```bash
cd frontend
npm run format:check
```

## Backend Linting

### golangci-lint
Comprehensive Go linter with multiple linters enabled.

Install:
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

Configuration: `backend/.golangci.yml` (to be created)

Run linter:
```bash
cd backend
golangci-lint run
```

### gofmt
Standard Go formatting tool.

Format code:
```bash
cd backend
gofmt -w .
```

Check formatting:
```bash
cd backend
gofmt -l .
```

### go vet
Go's built-in static analysis tool.

Run vet:
```bash
cd backend
go vet ./...
```

## Pre-commit Hooks

Consider setting up pre-commit hooks to run linters automatically:

```bash
# Example using git hooks
.git/hooks/pre-commit
```

## Continuous Integration

Linters should run in CI pipeline:
1. ESLint on frontend code
2. TypeScript type checking
3. golangci-lint on backend code
4. gofmt check
5. go vet

## Code Quality Metrics

Future integration with:
- Code coverage reports
- Cyclomatic complexity analysis
- Dependency vulnerability scanning

## To Be Implemented

- [ ] ESLint configuration
- [ ] Prettier configuration
- [ ] golangci-lint configuration
- [ ] Pre-commit hooks
- [ ] CI integration for linters
- [ ] Documentation linting (markdownlint)
