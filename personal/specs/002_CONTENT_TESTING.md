# Blog Content Testing System

A 5-layer testing pyramid for blog posts, mirroring the software testing analogy.

## New Dependencies

Add to `devDependencies` in `package.json`, then user runs `npm install`:
- `gray-matter` — parse YAML frontmatter from markdown files
- `markdownlint` — programmatic markdown linting (library, not CLI)
- `cspell` — spell checking
- `@anthropic-ai/sdk` — Anthropic SDK for LLM evals

## File Plan

### Shared infrastructure

| File | Purpose |
|------|---------|
| `tests/content/helpers/parse-posts.ts` | Read all `.md` files from `src/content/blog/`, parse frontmatter with `gray-matter`, export `getAllPosts()` / `getPublishedPosts()` / `getDraftPosts()` |
| `tests/content/helpers/dictionary.txt` | Custom word list for spell checking (MCP, pdb, Astro, Plaid, etc.) |
| `cspell.json` | Project-root cspell config pointing at `src/content/blog/**/*.md` and the custom dictionary |
| `vitest.content.config.ts` | Vitest config for layers 1-3 (lint + narrative tests) |
| `vitest.eval.config.ts` | Vitest config for layer 5 only (LLM evals, sequential, long timeouts) |

### Layer 1: Lint (mechanical, fast, no judgment)

| File | Checks |
|------|--------|
| `tests/content/lint/frontmatter.test.ts` | Mirrors Zod schema from `src/content/config.ts`. Title present, description present on published posts, pubDate valid, at least one tag, title length ≤ 80 chars |
| `tests/content/lint/spelling.test.ts` | Thin vitest wrapper around `cspell` CLI, per-post output |
| `tests/content/lint/markdown-syntax.test.ts` | Uses `markdownlint` programmatic API. Disables noisy rules (line length, inline HTML, first-line h1) |
| `tests/content/lint/links.test.ts` | Regex-extracts `[text](url)` links. Checks: non-empty link text, valid URL format, internal links resolve to known slugs. External link checking behind `CHECK_EXTERNAL_LINKS=true` env var |

### Layer 2: Section quality (per-section unit checks)

| File | Checks |
|------|--------|
| `tests/content/narrative/section-quality.test.ts` | Has at least one heading, no empty sections (<5 words), word count 100-5000, has intro text before first heading |

### Layer 3: Narrative flow (whole-post integration checks)

| File | Checks |
|------|--------|
| `tests/content/narrative/flow.test.ts` | Heading hierarchy (no skipping levels), description keywords appear in body, final section has substance (not abrupt), no `<!-- SUGGESTED` comments left in published posts |

### Layer 4: Build + render (Playwright)

| File | Checks |
|------|--------|
| `tests/e2e/blog-render.spec.ts` | Dynamically generates tests from blog posts on disk. Each published post: renders with correct title, shows date, has back link, appears on blog index. Draft posts: not listed, return 404 |

### Layer 5: LLM eval (acceptance tests)

| File | Purpose |
|------|---------|
| `tests/content/eval/personas.ts` | 4 reader personas: casual-reader, senior-engineer, hiring-manager, skeptic |
| `tests/content/eval/rubrics.ts` | 4 rubrics (comprehension, engagement, rigor, perception) each with 3 scored dimensions (1-5 scale) |
| `tests/content/eval/eval-runner.ts` | Sends post + persona + rubric to Claude Sonnet via `@anthropic-ai/sdk`, parses structured JSON scores |
| `tests/content/eval/blog-eval.test.ts` | Runs all persona×rubric combinations per post. Filterable via `EVAL_POST` and `EVAL_PERSONA` env vars. Min average score threshold: 2.5 |

### Modify existing files

**`package.json`** -- add devDependencies (see above) and scripts:
- `test:content` — layers 1-3 (lint + narrative)
- `test:spell` — cspell standalone
- `test:eval` — layer 5 LLM evals (costs money)
- `test:fast` — layers 1-4 combined (everything free/local)
- `test:all` — all 5 layers

## How it works for a new post

1. Write `src/content/blog/new-post.md`
2. `npm run test:content` — all lint/narrative tests auto-discover it
3. Add any flagged words to `dictionary.txt`
4. `npm run test:e2e` — rendering tests auto-discover it
5. `EVAL_POST=new-post npm run test:eval` — optional LLM feedback

No test files need editing. `getAllPosts()` reads from disk; `it.each` generates test cases dynamically.

## Verification

1. After creating all files, user runs `npm install`
2. `npm run test:content` — should pass (or flag known issues in current posts like the `explortation` typo and `:mindblown:` in skills-vs-mcp.md)
3. `npm run test:e2e` — should pass (extends existing Playwright setup)
4. `ANTHROPIC_API_KEY=... npm run test:eval` — should return scores and commentary for each published post
