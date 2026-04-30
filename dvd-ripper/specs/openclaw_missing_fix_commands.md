# Plan: openclaw fix-command coverage, classification severity, label casing

## Context

Three related defects make failure notifications less actionable than they should be:

1. **Failure notifications sometimes lack a follow-up fix command.** The `--approve`/`--resume`/`--force` triad covers every recoverable failure, but the code that emits the second openclaw message only fires for one narrow path (`verdict="warn"` with `fix_command=None`). Six distinct incidents over the past 30 days produced a "Rip FAILED:" notification with no actionable follow-up, forcing manual diagnosis.
2. **Movie/show misclassification is treated as a warning.** `check_is_movie` and `check_is_show` in `verify.py` flag misclassifications at `severity="warning"`, so `stage_verify` never raises and the wrong-named files sync silently to Jellyfin.
3. **Disc-label casing is inconsistent across entry points.** Commit `645e1e3` normalized the show-name *output* of `parse_disc_label`, but the raw label used as the state-file key is still whatever blkid returned. A user-supplied `--approve Legend_of_Korra_Book_3_Disc_1` cannot find a state file written as `Legend_Of_Korra_Book_3_Disc_1.json`, surfacing as a misleading FileNotFoundError that Claude diagnoses (incorrectly) as "SHOW_NAME casing mismatch".

This plan addresses all three. Order matters — issue 3 unblocks the `--approve` flow that issues 1 and 2 depend on.

## Issue 1 — Always include an actionable command in failure notifications

### Failure modes and their right answer

| Case | Failure path | Today's behavior | Right answer |
|---|---|---|---|
| A | `verify.check_episode_count` raises `VerificationError(message)` with `fix_command=None`; Claude either skipped or returned `verdict="fail"` with no fix | Single notification with raw error text | `--approve <label>` (artifacts on disk, user reviews) |
| B | Claude `verdict="fail"` returns `fix_command=null` | Single notification | `--approve <label>` |
| C | rsync/scp subprocess fails in transcode/sync stage | `analyze_error` returns `fix_command=None` per its system-prompt rule | `--resume <label>` (state intact, retry tail of pipeline) |
| D | Pre-pipeline crash before state is created (e.g. MakeMKV expired key) | No state, no fix | `--force` re-run (or manual diagnosis text) |

### Fix shape

In `rip.py` main `except` block (~lines 406-444), after `recommendation` and `fix_command` are computed:

- Factor a helper `_default_fix_command(state, exception, rip_script_path)` that returns a `systemd-run … rip.py …` string based on the decision tree below. Always quote with `shlex.quote`.
- If `fix_command is None`, set `fix_command = _default_fix_command(...)`.
- Send `fix_command` as the second openclaw message unconditionally when non-empty. Drop the narrow `verdict="warn"` branch at rip.py:431-441 — the helper covers it.

Decision tree inside the helper:

```
state has _state_path AND state file exists on disk:
    isinstance(exception, VerificationError)        → --approve <label>
    transcode/sync subprocess error                 → --resume <label>
    other exception                                 → --resume <label>
state file missing (pre-state crash):
    disc_label is set                               → --force (with disc_label)
    disc_label unknown                              → None (manual)
```

Also relax `error_analysis.py:42-43` — the rule "set fix_command to null unless the fix is clearly a re-run" pushed Claude away from suggesting `--resume`. Replace with: "for subprocess failures during rip/transcode/sync stages, recommend `--resume <label>` using the absolute rip.py path."

### TDD sequence

1. **Write `tests/test_failure_notifies_command.py`** with three failing tests:
   - `test_count_mismatch_without_claude_includes_approve` — stub `claude.is_available=False`, force `check_episode_count` to raise, capture both `notify` calls, assert second contains `--approve` and the disc label.
   - `test_subprocess_failure_includes_resume` — simulate `subprocess.CalledProcessError` from sync stage with valid state, assert second `notify` call contains `--resume`.
   - `test_verification_fail_with_null_fix_includes_approve` — Claude returns `verdict="fail", fix_command=None`, assert `--approve` appears.
2. **Run** `python3 -m unittest tests.test_failure_notifies_command` — confirm three failures.
3. **Implement** `_default_fix_command` in `rip.py`. Update the except block to call it when `fix_command` is None. Remove the narrow `verdict="warn"` branch.
4. **Re-run** — three tests pass.
5. **Edit** `error_analysis.py:42-43` to replace the null-encouraging rule with `--resume` guidance.
6. **Run** `python3 -m unittest discover tests` — full suite green.

## Issue 2 — Promote movie/show misclassification to severity="error"

### Root cause

`verify.py:219, 238, 267` set `severity="warning"`. In `stage_verify` (verify.py:455), only errors raise `VerificationError`. Movie-on-TV-disc and TV-on-movie-disc misclassifications — which produce wrong filenames that won't work in Jellyfin — currently log a warning and continue to sync. Commit `4472af9` introduced these checks specifically to catch the Bluey "27 episodes ripped as a 7-min movie" bug; promoting them to error matches the original intent.

### Fix shape

Change three string literals in `verify.py` from `"warning"` to `"error"`:
- Line 219 — `check_is_movie` TV-label heuristic
- Line 238 — `check_is_movie` 3-similar-titles heuristic
- Line 267 — `check_is_show` single-episode-on-single-title heuristic

This routes the issue through the existing `if errors:` raise at verify.py:455. The user receives a `VerificationError` notification, and after manual review can run `rip.py --approve <label>` to bypass and finish syncing. The `--approve` plumbing (rip.py:287-288, 329, 356-358) is already wired up.

`check_is_show` only fires when `media_type == "tv"` AND exactly 1 planned episode AND the disc has 1 episode-range title — the exact failure mode introduced by the `TITLES` env override at rip.py:122. Promoting it to error gives that override a guardrail.

### TDD sequence

1. **Add tests in `tests/test_verify.py` under `TestStageVerify`:**
   - `test_suspect_movie_tv_label_raises` — state with `media_type="movie"`, disc_label `"Avatar_Book_1_Disc_1"`, single planned episode, single 6000s title; stub Claude unavailable; assert `stage_verify` raises `VerificationError` whose message mentions the suspect classification.
   - `test_suspect_movie_3_similar_titles_raises` — state with `media_type="movie"`, 3 titles ~1400s each; assert raises.
   - `test_suspect_show_single_title_raises` — state with `media_type="tv"`, 1 episode, 1 episode-range title; assert raises.
   - `test_movie_with_unrelated_label_passes` — control: `media_type="movie"`, label `"INCEPTION"`, single 7000s title; assert no raise (catches accidental over-flagging).
2. **Run** `python3 -m unittest tests.test_verify` — first three fail, fourth passes.
3. **Edit** `verify.py` lines 219, 238, 267 — `"warning"` → `"error"`.
4. **Re-run** — all four pass.
5. **Run** `python3 -m unittest discover tests` — confirm no regressions.

## Issue 3 — Label casing parity across entry points

### Root cause (Apr 28 incident)

1. blkid extracted `Legend_Of_Korra_Book_3_Disc_1` (capital `Of`); state file written with that exact name.
2. Verify failed. User copy-pasted the label as `Legend_of_Korra_Book_3_Disc_1` (lowercase `of`) and ran `--approve`.
3. `state.resolve_state_arg` (state.py:111-115) did a case-sensitive `Path.exists()` check, raised `FileNotFoundError`.
4. `analyze_error` invoked Claude, which guessed at "SHOW_NAME casing mismatch" — a misleading diagnosis since the casing diff was in the *disc-label argument*, not the SHOW_NAME variable.

Commit `645e1e3` only normalized the `show_name` output of `parse_disc_label` (line 46, `name.title()`). The raw label string is still passed through unchanged to `state_path_for_label`, `--resume`, and `--approve`.

### Fix shape — three coordinated changes

**3a. `disc.py`** — add `normalize_disc_label(label: str) -> str`:
- Splits on `_`, title-cases each token (digits passthrough), rejoins with `_`.
- `Legend_of_Korra_Book_3_Disc_1` → `Legend_Of_Korra_Book_3_Disc_1`.
- Empty string and labels without underscores pass through unchanged.

**3b. `rip.py`** — apply `normalize_disc_label` at every label-entry point:
- `stage_scan_disc` (lines 92-95) — after blkid extraction.
- `main()` fresh-run blkid block (lines 380-385) — same normalization.
- `--resume` arg parsing (lines 360-376) — normalize the user-supplied label before calling `state_path_for_label`.
- `--approve` arg parsing (line 357) — same.

**3c. `state.py`** — add case-insensitive fallback in `resolve_state_arg`:
- After the line-114 `path.exists()` miss, scan `<rip_dir>/.state/*.json`, build `lower_stem → actual_path` map.
- If exactly one match for `arg.lower()`, load it.
- If multiple matches, raise `RuntimeError` listing all candidates.
- Same fallback in `discover_active_state` after the line-140 miss (handles old non-normalized state files left over from before this change).

**3d. `error_analysis.py`** — extend `ERROR_SYSTEM_PROMPT` Guidelines block with two new bullets:
- `--resume LABEL` — when the state file exists and the failure was mid-pipeline (rip/transcode/sync). Recommends continuing the pipeline rather than restarting.
- `--approve LABEL` — when artifacts are on disk and only verify failed. Bypasses verify and finishes sync.
- Show concrete `systemd-run --user --unit="dvd-rip-$(date +%s)" "/abs/path/rip.py" --resume <label>` and `… --approve <label>` syntax.

### TDD sequence

1. **Write `tests/test_normalize_disc_label.py`** (failing first):
   - `test_lowercase_connector_normalized` — `normalize_disc_label("Legend_of_Korra_Book_3_Disc_1") == "Legend_Of_Korra_Book_3_Disc_1"`.
   - `test_already_normalized_passthrough` — string in equals string out.
   - `test_all_caps_normalized` — `"BREAKING_BAD_S3_D2"` → `"Breaking_Bad_S3_D2"`.
   - `test_empty_string_passthrough`.
   - `test_no_underscore_label` — single token unchanged.
   - Run → fails. Add `disc.normalize_disc_label`. Re-run → passes.

2. **Write `tests/test_state_label_casing.py`** (failing first):
   - `test_resolve_state_arg_finds_case_variant` — write `.state/Legend_Of_Korra_Book_3_Disc_1.json`, call `resolve_state_arg(rip_dir, "Legend_of_Korra_Book_3_Disc_1")`, assert state loads.
   - `test_resolve_state_arg_exact_match_unchanged` — control case (exact-case lookup still works).
   - `test_resolve_state_arg_ambiguous_raises` — write two state files differing only in case, assert raises with both candidates in the message.
   - `test_resolve_state_arg_missing_still_raises` — nonexistent label → RuntimeError as today.
   - Run → first and third fail. Implement case-insensitive fallback in `state.resolve_state_arg`. Re-run → all pass.

3. **Write `tests/test_label_normalization_at_entry.py`** (failing first):
   - `test_stage_scan_disc_normalizes_label` — patch `subprocess.run` to return `"Legend_of_Korra_Book_3_Disc_1"`; call `stage_scan_disc`; assert `state["disc_label"] == "Legend_Of_Korra_Book_3_Disc_1"`.
   - `test_main_fresh_run_normalizes_label` — same idea via the main() blkid fallback (unit-test the helper extracted from main).
   - Run → fails. Apply normalization at the three entry points in `rip.py`. Re-run → passes.

4. **Edit `error_analysis.py` ERROR_SYSTEM_PROMPT**. No new unit test for prompt text, but run `python3 -m unittest discover tests -p 'eval_*.py'` to confirm `eval_errors.py` still passes (it exercises real Claude responses against the prompt).

5. **Final regression sweep** — `python3 -m unittest discover tests`.

## Files to modify

- `disc.py` — add `normalize_disc_label` (issue 3).
- `state.py` — case-insensitive fallback in `resolve_state_arg` and `discover_active_state` (issue 3).
- `rip.py` — apply `normalize_disc_label` at entry points (issue 3); add `_default_fix_command` helper, rewrite the except-block fix-command logic (issue 1).
- `verify.py` — three `"warning"` → `"error"` changes (issue 2).
- `error_analysis.py` — relax the null-fix-command rule for subprocess failures; document `--resume`/`--approve` in `ERROR_SYSTEM_PROMPT` (issues 1 + 3).
- `tests/test_failure_notifies_command.py` — new (issue 1).
- `tests/test_verify.py` — four new tests under `TestStageVerify` (issue 2).
- `tests/test_normalize_disc_label.py` — new (issue 3).
- `tests/test_state_label_casing.py` — new (issue 3).
- `tests/test_label_normalization_at_entry.py` — new (issue 3).

## Execution order

1. **Issue 3 first.** Foundational: label normalization is a prerequisite for `--approve` working reliably. Lowest risk — pure helper + one fallback path.
2. **Issue 2 next.** Three one-line severity changes. Verifies `--approve` plumbing works with normalized labels from #3.
3. **Issue 1 last.** Builds on `--approve`/`--resume` correctness from #3 and the `VerificationError` route from #2. Tests can assert the full chain end-to-end.

After each step, run `python3 -m unittest discover tests` before moving on.
