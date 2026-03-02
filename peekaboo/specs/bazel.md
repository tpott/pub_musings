# Bazel Migration Research

**Status:** Research complete (2026-02-05)
**Task:** 116
**Recommendation:** Do not migrate at this time (see [Recommendation](#recommendation))

## Project Structure

```
peekaboo/
├── backend/           # Go 1.24, go.mod with 6 dependencies
│   ├── api/           # HTTP/WebSocket handlers
│   ├── crypto/        # Age encryption
│   ├── db/            # SQLite (cgo via mattn/go-sqlite3)
│   ├── llm/           # Anthropic/OpenAI providers
│   ├── logging/       # Structured logging
│   └── tts/           # Piper TTS client
└── frontend/          # Astro 5, TypeScript
    ├── src/lib/       # Core modules (vitest)
    └── tests/e2e/     # Playwright e2e tests
```

## 1. Go Backend Build

### Tool: rules_go v0.59+ with Gazelle

**Support level:** Production-ready

**How it would work:**
- `MODULE.bazel` at project root declares `rules_go` and `gazelle` dependencies
- Gazelle auto-generates `BUILD.bazel` files from Go source
- `go_deps` module extension reads `go.mod` for transitive dependencies
- `go_binary(name = "peekaboo")` builds the server
- Each package (`api/`, `crypto/`, `db/`, etc.) gets its own `go_library` target

**Key concern: cgo dependency (go-sqlite3)**

`mattn/go-sqlite3` requires cgo (C compiler). This is supported by rules_go but adds complexity:
- Must ensure `CC` toolchain is available in Bazel sandbox
- Cross-compilation becomes harder
- Build times increase due to C compilation
- The `pure = "off"` flag (or default) is needed for cgo targets

**Sample BUILD.bazel for `backend/`:**
```python
load("@rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "backend_lib",
    srcs = ["main.go"],
    importpath = "github.com/tpott/pub_musings/peekaboo/backend",
    deps = [
        "//backend/api",
        "//backend/crypto",
        "//backend/db",
        "//backend/llm",
        "//backend/logging",
        "//backend/tts",
    ],
)

go_binary(
    name = "peekaboo",
    embed = [":backend_lib"],
)
```

**Verdict:** Would work well. Gazelle handles most BUILD file generation.

## 2. Go Tests

### Tool: go_test (part of rules_go)

**Support level:** Production-ready

**How it would work:**
- Gazelle generates `go_test` targets alongside `go_library` targets
- `bazel test //backend/...` runs all Go tests
- Test fixtures need to be declared as `data` attributes

**Key concern: test fixtures and data files**

Several tests use file system fixtures (e.g., `tests/fixtures/`). These must be explicitly listed:
```python
go_test(
    name = "api_test",
    srcs = glob(["*_test.go"]),
    data = ["//tests/fixtures:all"],
    embed = [":api_lib"],
)
```

**Verdict:** Would work well with minor fixture path adjustments.

## 3. Node.js / npm Builds

### Tool: rules_js (Aspect Build)

**Support level:** Production-ready

**How it would work:**
- `npm_translate_lock` reads `package-lock.json` to create Bazel repository rules
- Each npm package becomes a Bazel target
- `js_library` and `js_binary` rules for source code
- Dependencies lazily fetched (only what's needed for current target)

**Key concern: lock file format**

rules_js natively supports pnpm lockfiles. npm lockfiles require `npm_translate_lock` which works but pnpm is preferred. This would mean switching the project from npm to pnpm.

**Verdict:** Would work. Slight friction if staying with npm over pnpm.

## 4. Astro Static Build

### Tool: Custom genrule or js_binary wrapper

**Support level:** Experimental - no native Bazel rules for Astro

**How it would work:**

Option A - genrule:
```python
genrule(
    name = "astro_build",
    srcs = glob(["src/**/*", "public/**/*"]) + [
        "package.json",
        "astro.config.mjs",
        "tsconfig.json",
    ],
    outs = ["dist"],
    cmd = "cd frontend && npx astro build --outDir $@",
    tools = ["@npm//:node_modules/astro"],
)
```

Option B - js_binary wrapping astro CLI:
```python
js_binary(
    name = "astro",
    data = ["//:node_modules/astro"],
    entry_point = "node_modules/astro/astro.js",
)
```

**Key concerns:**
- Astro's build uses dynamic imports and file system traversal that's hard to sandbox
- All input files must be explicitly declared for Bazel caching to work
- Astro plugins may access files outside the sandbox
- Vite (Astro's build tool) creates temp files that may break sandboxing

**Verdict:** Possible but fragile. Would require significant testing and may not cache reliably.

## 5. Vitest

### Tool: fremtind_rules_vitest v0.2.1

**Support level:** Beta/Experimental

**How it would work:**
```python
load("@fremtind_rules_vitest//vitest:defs.bzl", "vitest_test")

vitest_test(
    name = "logger_test",
    srcs = ["src/lib/logger.test.ts"],
    deps = [
        ":logger",
        "//:node_modules/vitest",
    ],
    config = "vitest.config.ts",
)
```

**Key concerns:**
- Test file discovery may not work out of the box (known issue)
- jsdom environment setup (used by our tests) needs careful configuration
- Import.meta.env mocking requires Vite integration that may not work in sandbox
- The 237 existing tests use relative imports that Bazel may not resolve correctly

**Verdict:** Risky. Known issues with test file discovery and environment setup.

## 6. Playwright E2E Tests

### Tool: rules_playwright v0.5.3

**Support level:** Functional but complex

**How it would work:**
- Browser binaries downloaded as Bazel targets
- `PLAYWRIGHT_BROWSERS_PATH` must be set
- Tests need access to both frontend build and backend binary

**Key concerns:**
- Browser binaries are 100-300MB each, adding significant download time
- Need to orchestrate: build frontend → start backend → run tests → stop backend
- Our tests use `addInitScript()` for mocking, which requires Chromium
- Platform-specific browser versions must be managed
- The mock WebSocket helpers use Playwright's `routeWebSocket` API

**Integration test orchestration:**
```python
# This is the hard part - need a test runner that:
# 1. Starts the Go backend
# 2. Builds and serves the Astro frontend
# 3. Runs Playwright against localhost
# 4. Tears everything down
```

**Verdict:** Possible but the most complex part. Integration test orchestration is non-trivial in Bazel.

## 7. Implementation Attempt

**Bazel is not installed on this machine.** I could not attempt an actual implementation.

To attempt implementation, you would need:
```bash
# Install Bazelisk (Bazel version manager)
npm install -g @bazel/bazelisk
# or: go install github.com/bazelbuild/bazelisk@latest

# Then from project root:
bazel build //backend/...   # Build Go backend
bazel test //backend/...    # Run Go tests
bazel build //frontend:dist # Build Astro frontend
bazel test //frontend/...   # Run vitest
```

### Files that would be needed:

1. `MODULE.bazel` - Root module with all dependencies
2. `BUILD.bazel` - Root build file
3. `backend/BUILD.bazel` - Go binary target
4. `backend/api/BUILD.bazel` - API library + tests (repeat for each package)
5. `frontend/BUILD.bazel` - Astro build + vitest + playwright targets
6. `.bazelrc` - Configuration flags
7. `.bazelignore` - Exclude node_modules, data/, etc.

### Sample MODULE.bazel:
```python
module(
    name = "peekaboo",
    version = "0.0.1",
)

# Go
bazel_dep(name = "rules_go", version = "0.59.0")
bazel_dep(name = "gazelle", version = "0.42.0")

go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
go_deps.from_file(go_mod = "//backend:go.mod")

# JavaScript
bazel_dep(name = "aspect_rules_js", version = "2.3.0")
bazel_dep(name = "rules_nodejs", version = "6.3.0")  # for node toolchain

npm = use_extension("@aspect_rules_js//npm:extensions.bzl", "npm")
npm.npm_translate_lock(
    name = "npm",
    pnpm_lock = "//frontend:pnpm-lock.yaml",
)

# Vitest (optional)
bazel_dep(name = "fremtind_rules_vitest", version = "0.2.1")

# Playwright (optional)
bazel_dep(name = "rules_playwright", version = "0.5.3")
```

## Cost-Benefit Analysis

### Benefits of Bazel migration:
1. **Hermetic builds** - Same output regardless of machine state
2. **Incremental builds** - Only rebuild what changed
3. **Cross-language caching** - Share cache between Go and TS
4. **Remote caching** - Share build cache across team/CI
5. **Unified build command** - `bazel build //...` builds everything

### Costs:
1. **Large learning curve** - Bazel is complex, especially for Astro/Vitest integration
2. **Maintenance overhead** - BUILD files must be kept in sync (Gazelle helps for Go only)
3. **Fragile Astro build** - No native support, custom rules needed
4. **Vitest risk** - Beta support, known issues
5. **Playwright complexity** - Browser binary management, test orchestration
6. **cgo complication** - go-sqlite3 requires C toolchain in sandbox
7. **Development workflow change** - `go test` and `npm test` replaced with `bazel test`
8. **No Bazel currently installed** - Would need to be installed and maintained

### Current build times (without Bazel):
- `go build ./...` - ~2 seconds
- `go test ./...` - ~5 seconds
- `npm test` - ~19 seconds
- `npx playwright test` - ~60 seconds

These are already fast. Bazel's incremental build advantage is most valuable when builds take minutes, not seconds.

## Recommendation

**Do not migrate to Bazel at this time.**

Reasons:
1. **Project is small** - 6 Go packages, 1 Astro frontend, ~300 tests. Current build/test cycle is under 90 seconds total.
2. **Solo developer** - Remote caching benefit is minimal without a team.
3. **Astro has no native support** - This is the weakest link and would require ongoing custom maintenance.
4. **Vitest integration is beta** - Risk of test discovery issues with 237 tests.
5. **Existing tools work well** - `go test`, `npm test`, and `npx playwright test` are simple and reliable.
6. **ROI is negative** - Migration effort (estimated 2-4 days) exceeds the time it would save over the project's lifetime.

### When to reconsider:
- If the project grows to 20+ Go packages or 50+ frontend modules
- If a team of 3+ developers would benefit from remote caching
- If build times exceed 5 minutes
- If native Astro rules emerge in the Bazel ecosystem
