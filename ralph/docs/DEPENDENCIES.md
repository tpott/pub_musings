# Adding Dependencies

## Before Adding ANY Dependency

1. **WebSearch** for: `"{package}" alternatives comparison 2026`
2. **Evaluate** at least 2 alternatives:
   - Maintenance status (last commit, open issues, bus factor)
   - Size impact (bundle size, binary size, transitive deps)
   - Security (known CVEs, audit history)
   - API ergonomics (does it fit our patterns?)

3. **Document** in `LEARNINGS.md` using Decisions format (see LEARNINGS-FORMAT.md)

4. **Only then** add to package.json / go.mod / requirements.txt

## Red Flags — Require Extra Scrutiny

- Last commit > 6 months ago
- < 100 GitHub stars for core functionality
- No TypeScript types (for JS packages)
- Excessive transitive dependencies
- Known security issues in last 12 months

## Approved Patterns

Prefer these known-good choices when applicable:

| Need | Go | TypeScript |
|------|-----|------------|
| HTTP router | stdlib `net/http` | - |
| Database | `mattn/go-sqlite3` | - |
| Encryption | `filippo.io/age` | - |
| Testing | stdlib `testing` | `vitest` |
| Validation | - | `zod` |
