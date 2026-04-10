# Plan: Claude-in-the-Loop Verification Stage

## Context

The She-Ra S01 disc had 7 episode titles (all multi-segment), but `select_episode_titles()` in `titles.py:179-185` deduped them by segment string — titles 1-6 all have segments `"1-4,5"` so only one survived. Result: 2 of 7 episodes ripped, pipeline reported "success" for E01-E02. The user wants Claude CLI to evaluate rip results so this class of bug gets caught automatically, with a recommended fix command in the failure notification.

## Approach

The verify stage runs three deterministic checkers, then invokes `claude --print --dangerously-skip-permissions --system-prompt` to evaluate the results. Claude returns a structured JSON verdict. On failure, the pipeline halts before sync and sends one openclaw message with Claude's recommended fix command.

## Files to Create

### 1. `verify.py` (~200 lines)

Structure:
```
check_episode_count(state) -> dict     # compare mp4 count vs disc title count
check_duration_anomaly(state) -> dict  # ffprobe durations, flag >40% deviation
check_file_integrity(state) -> dict    # planned episodes all have non-zero mp4s
build_verify_prompt(state, checker_results) -> str
run_claude_verify(prompt, conf) -> dict
stage_verify(conf, state) -> state
```

**Checkers** are pure functions. `check_episode_count` counts episode-length titles (15-65 min) in `state["scan_disc"]["makemkv_info"]` and compares against `len(state["plan"]["episodes"])`. This alone would have caught She-Ra (7 disc titles vs 2 selected).

**Claude invocation** adapts the ralph pattern from `ralph/src/ralph/loop.py:543-598`:
- `subprocess.Popen(["claude", "--print", "--dangerously-skip-permissions", "--output-format=stream-json", "--system-prompt", SYSTEM_PROMPT], stdin=PIPE, stdout=PIPE)`
- Pipe dynamic prompt via stdin (state + checker results + raw makemkv info + ls of output dir)
- Scan stream-json for `"type":"result","subtype":"success"`, extract `result` text, `json.loads()` it

**System prompt** forces a JSON schema response:
```json
{
  "verdict": "pass|fail|warn",
  "confidence": 0.0-1.0,
  "issues": [{"type": "...", "detail": "...", "severity": "error|warning"}],
  "recommendation": "human-readable",
  "fix_command": "TITLES=0,1,2,3,4,5,6 python3 rip.py"
}
```

Claude is invoked when any checker flags an issue OR `VERIFY_CLAUDE_ALWAYS=true` in rip.conf. Skipped on clean rips by default to save API cost.

**On failure**: `stage_verify` raises `VerificationError`. The pipeline runner marks the stage failed in state.json. `stage_finalize` (or the exception handler) composes one openclaw message:
```
Rip HALTED: SHE RA S01 — only 2 of 7 episodes ripped.
Fix: TITLES=0,1,2,3,4,5,6 python3 /home/trevor/.../rip.py
State: .state/SHE_RA.json
```

### 2. `tests/test_verify.py` (~120 lines)

Unit tests for the three checkers using fixture data. Deterministic, no Claude needed.

- `test_count_mismatch_she_ra`: 7 disc titles, 2 selected → checker returns fail. Uses real She-Ra TINFO data from journald as a new `SHE_RA_DISC_INFO` constant in `test_data.py`.
- `test_count_ok_avatar`: 8 disc titles, 8 selected → checker returns pass. Uses existing `AVATAR_DISC_INFO`.
- `test_duration_anomaly_detected`: one 46m episode among 23m episodes → flagged.
- `test_duration_all_similar`: all ~23m → pass.
- `test_file_integrity_missing`: planned 5, only 4 exist → fail.

### 3. `tests/eval_verify.py` (~150 lines)

LLM eval tests — invoke real `claude` CLI with fixture data and assert on the structured JSON verdict. Gated with `@unittest.skipUnless(shutil.which("claude"), "claude CLI not available")`.

**Eval cases:**
1. `test_eval_count_mismatch`: She-Ra state (2 of 7 episodes) → expect `verdict="fail"`, `fix_command` contains title IDs
2. `test_eval_all_correct`: Avatar state (8 of 8) → expect `verdict="pass"`
3. `test_eval_duration_anomaly`: 46m episode among 23m → expect verdict mentions it (warn or fail)
4. `test_eval_fix_command_valid`: For She-Ra case, assert `fix_command` is a runnable shell command containing `TITLES=`

Each eval runs once by default. Asserts on `verdict` string (the most stable field). Subsidiary fields logged but not hard-asserted.

### 4. `tests/test_data.py` — add `SHE_RA_DISC_INFO`

New constant with the real She-Ra makemkv info from journald (7 titles, all multi-segment with 2 segments, segments "1-5,6" and "1-4,5").

### 5. `pipeline.py` (~60 lines) — from existing plan

Tiny DAG runner. Each stage is `fn(conf, state) -> state`. Persists state.json between stages. On exception, marks stage failed.

```python
STAGES = [
    ("scan_disc",   [],              stage_scan_disc),
    ("plan",        ["scan_disc"],   stage_plan),
    ("rip",         ["plan"],        stage_rip),
    ("transcode",   ["rip"],         stage_transcode),
    ("verify",      ["transcode"],   stage_verify),
    ("sync",        ["verify"],      stage_sync),
    ("finalize",    ["sync"],        stage_finalize),
]
```

### 6. `state.py` (~100 lines) — from existing plan

State at `RIP_DIR/.state/<disc-label>.json`. Archived to `.state/history/<label>-<timestamp>.json` on completion.

## Files to Modify

### `rip.py`
- Add argparse (`--resume [LABEL_OR_PATH]`, `--force`)
- Break `main()` into stage functions
- `stage_scan_disc` must store raw `makemkv_info` in state (needed by verify)
- Import and wire to `pipeline.run()`

### `transcode.py`
- Split `transcode_and_sync` into `transcode_only` (lines 49-84) and `sync_only` (lines 86-89) so verify can run between them

### `rip.conf.example`
- Add: `VERIFY_CLAUDE_ALWAYS="false"`, `VERIFY_MODEL=""`, `VERIFY_FRAME_OFFSET="75"`, `ANTHROPIC_API_KEY=""`

### `.gitignore`
- Add: `.state/`, `.frames/`

## Key Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| Claude invoked always? | Only on checker issues (configurable) | Saves API cost on clean rips |
| Include raw makemkv info in prompt? | Yes, full output | Claude needs title/segment mapping to recommend TITLES= fix |
| Import ralph's run_claude? | No, copy the ~30-line pattern | Cross-project import is fragile |
| Eval test strategy | Fixture-driven, assert on verdict string | Handles LLM non-determinism; stable on the primary field |
| OCR (Check 3) | Included but gated on ANTHROPIC_API_KEY | User made it non-optional in plan doc |
| openclaw agent for auto-fix? | Deferred to v2 | Different trust boundary; v1 detects + recommends |

## Implementation Order

1. `tests/test_data.py` — add `SHE_RA_DISC_INFO` fixture
2. `state.py` + `tests/test_state.py`
3. `pipeline.py` + `tests/test_pipeline.py`
4. `rip.py` refactor — stage functions + argparse + wire to pipeline. Split transcode_and_sync.
5. `verify.py` — checkers first (testable without Claude), then Claude invocation + prompt
6. `tests/test_verify.py` — unit tests for checkers
7. `tests/eval_verify.py` — LLM eval tests
8. Config, gitignore updates

## Verification

- `python3 -m unittest discover tests` — all unit tests pass (existing + new)
- `python3 -m unittest tests.eval_verify -v` — LLM evals pass (requires `claude` CLI)
- Manual: create a mock state.json with She-Ra's 2-of-7 scenario, run `stage_verify` standalone, confirm Claude returns fail verdict with TITLES= fix command
- Manual: create a state.json with Avatar's 8-of-8 scenario, confirm Claude returns pass
- Confirm the full DAG runs end-to-end on a real disc (will need to ask user to run)
