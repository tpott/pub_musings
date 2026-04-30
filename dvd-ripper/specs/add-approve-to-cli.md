# Plan: Add `--approve LABEL` to dvd-ripper

## Context

The post-rip verifier (`verify.py:stage_verify`) sometimes raises `VerificationError` for false positives — a checker fires `count_mismatch`, but Claude correctly assesses the rip as fine and returns `verdict=warn, fix_command=null`. Today the only recovery options are:

- `./rip.py --resume LABEL` — re-runs verify, deterministically fails the same way.
- `VERIFY_ENABLED=false ./rip.py --resume LABEL` — bypasses verification entirely (loses safety net).

Most recent incident: `Legend_Of_Korra_Book_2_Disc_1` (Apr 25, 2026). 7 episodes ripped + transcoded successfully, but disc has 7 short bonus features that pushed the episode-length count to 14, triggering `count_mismatch`. Claude correctly said "rip is fine, no re-rip needed" — but the user got a Matrix notification with no actionable command, and the .mp4 files never made it to Jellyfin.

`--approve LABEL` is the surgical escape hatch: mark this specific failed verification as accepted by the user, skip verify, and run the remaining stages (sync to Jellyfin, finalize, archive state). Audit trail preserved in the state file.

## Behavior

- `--approve` (no arg) — auto-detect from inserted disc via `blkid` (mirrors `--resume`).
- `--approve LABEL` — load `.state/LABEL.json`.
- `--approve /abs/path.json` — load that state file directly.
- Mutually exclusive with `--resume` (argparse-enforced).
- On load: mark `state["stages"]["verify"] = {"status": "complete", "completed_at": now, "approved": True, "approved_at": now}` and reset `state["status"] = "running"`. Save before calling `run_pipeline` so the override is durable if the subsequent run fails.
- `run_pipeline` skips verify (because status==complete), runs sync + finalize, archives state.
- `stage_finalize` appends "(verification manually approved)" to the success notification when `state["stages"]["verify"]["approved"]` is truthy — so the success message is transparent about the override.
- Edge cases:
  - State file missing → notify "Rip FAILED: <label>\nNo state file" and exit 1.
  - Verify never reached (e.g., transcode failed) → still mark verify complete; pipeline will re-run earlier failed stages and may fail again. Documented behavior, not a bug.
  - Already-approved state → idempotent (overwrite with new timestamp, proceed normally).

## Design rationale (validated)

**Why mutate `state["stages"]["verify"]` rather than add a check inside `stage_verify`:** The pipeline runner (`pipeline.py:19,24,30`) overwrites `state["stages"][name]` every time it runs a stage. If `stage_verify` early-returned on an `approved` flag, the runner's "complete" stamp would clobber the `approved=true`/`approved_at` audit fields. Marking verify complete *before* `run_pipeline` causes the runner to skip the entry entirely, preserving the audit trail. No changes needed to `verify.py`.

**Single source of truth for the "approved" fact:** Live in `state["stages"]["verify"]["approved"]`, not duplicated to `state["verification"]["approved"]`. The existing `state["verification"]["issues"]` and `claude_verdict` are preserved untouched as the audit trail of what the checkers/Claude found.

## Files to modify

- `state.py` — add `approve_state(state)` helper (pure dict mutation).
- `rip.py` — add `--approve` argparse, mutual exclusion with `--resume`, dispatch branch in `main()`, "approved" note in `stage_finalize`'s success notification.
- `tests/test_approve.py` — new file, all tests for this feature.

## TDD sequence

Strict red-green: write each test, watch it fail, write minimal code to pass, move on. One new file `tests/test_approve.py`. Run with `python3 -m unittest discover tests` between every step.

### Step 1 — `approve_state` pure-function unit tests

**Class:** `TestApproveState` in `tests/test_approve.py`.

Tests:
1. `test_marks_verify_complete_with_approved_flag` — given a state with verify=failed, after `approve_state(state)`: `state["stages"]["verify"]["status"] == "complete"`, `["approved"] is True`, `["approved_at"]` is a recent ISO timestamp, `["completed_at"]` is present.
2. `test_resets_top_level_status_to_running` — state["status"]=="failed" before, "running" after.
3. `test_preserves_verification_issues` — pre-existing `state["verification"]["issues"]` and `["claude_verdict"]` are unchanged after the call (audit trail).
4. `test_idempotent` — calling `approve_state` twice doesn't corrupt the dict; second call still leaves `status=complete, approved=True`.

**Production change:** add `approve_state(state)` to `state.py`. No mocks needed.

### Step 2 — Label/path resolution unit tests

**Class:** `TestApproveLoadState` in `tests/test_approve.py`. Use `tempfile.mkdtemp()` + `shutil.rmtree` (mirror `tests/test_state.py` setUp/tearDown).

Tests:
5. `test_load_by_label_missing_raises` — call resolver with a label whose state file doesn't exist; assert it raises `RuntimeError` with a message containing the label and "No state file" (not bare `FileNotFoundError`).
6. `test_load_by_label_present` — pre-create a state file via `new_state` + `save_state`, call resolver with that label; assert the loaded state has the right `disc_label`.
7. `test_load_by_absolute_path` — pass an absolute path; assert it loads.
8. `test_load_by_blkid_autodetect` — patch `state.subprocess.run` to return a fake disc label (mirror `tests/test_discover_active_state.py:66-78`), pre-create the matching state file, pass `None`/`True` to resolver; assert correct state loaded.
9. `test_load_blkid_no_disc` — patch blkid to raise `CalledProcessError`; assert resolver raises `RuntimeError("No disc in drive...")`.
10. `test_load_blkid_disc_no_state` — patch blkid to return a label with no matching state file; assert resolver raises `RuntimeError("No state file matches...")`.

**Production change:** if the resolution logic is identical enough to the `--resume` block at `rip.py:345-361`, factor that block out into a small helper (e.g., `state.py:resolve_state_arg(rip_dir, arg)`) and call it from both branches. Otherwise inline a parallel block in `main()`. The helper approach is cleaner and gives a unit-testable surface; lean toward extracting the helper.

### Step 3 — CLI parsing tests

**Class:** `TestApproveCLI` in `tests/test_approve.py`. Mirror `tests/test_no_active_state_notify.py` for argv-patching.

Tests:
11. `test_approve_alone_parses` — `sys.argv = ["rip.py", "--approve", "LABEL"]`; calling `main()` reaches the approve branch (verified by patching the resolver and asserting it was called with "LABEL").
12. `test_approve_no_arg_triggers_autodetect` — `sys.argv = ["rip.py", "--approve"]`; resolver called with `True` (or `None`).
13. `test_approve_and_resume_mutually_exclusive` — `sys.argv = ["rip.py", "--approve", "X", "--resume", "Y"]`; assert `SystemExit(2)` (argparse error).

**Production change:** add `parser.add_mutually_exclusive_group()` containing `--approve` and `--resume`. Both use `nargs="?", const=True, default=None`.

### Step 4 — `main()` happy-path integration test

**Class:** `TestApproveMainHappyPath` in `tests/test_approve.py`. Mirror `tests/test_state_archive_on_new_rip.py:48-75` for the `fake_run_pipeline` pattern. Patch `rip.notify`, `rip.analyze_error`, `rip.archive_state`.

Tests:
14. `test_approve_skips_verify_runs_remaining` — pre-create a state with stages scan_disc/plan/rip/transcode complete and verify=failed, plus `_jobs` populated. `sys.argv = ["rip.py", "--approve", "LABEL"]`. Patch `rip.run_pipeline` with a fake that captures the state it received. Run `main()`. Assert: no `SystemExit`, the state passed to `run_pipeline` had `stages.verify.status=="complete"` and `stages.verify.approved is True`, `archive_state` was called with the final state.
15. `test_approve_state_saved_before_run_pipeline` — assert `save_state` was called with the approved state *before* `run_pipeline` was invoked (so the override is durable on a subsequent crash). Use a side-effect on the patched `save_state` to record call ordering, or check the file on disk.

**Production change:** add the `if args.approve:` dispatch branch in `main()` after the `--resume` block. It calls the resolver, calls `approve_state`, calls `save_state`, then falls through to the existing `run_pipeline(STAGES, conf, state, save_state)` + `archive_state(state)`.

### Step 5 — `stage_finalize` "approved" note test

**Class:** `TestStageFinalizeApprovedNote` in `tests/test_approve.py`. Patch `rip.notify`. No tmpdir needed — `stage_finalize` is a pure function over state + conf.

Tests:
16. `test_finalize_appends_approved_note_when_approved` — call `stage_finalize(conf, state)` with `state["stages"]["verify"]["approved"] = True`; assert the message passed to `notify` contains "(verification manually approved)".
17. `test_finalize_no_note_when_not_approved` — same but without the approved flag; assert the note is absent.

**Production change:** in `stage_finalize` (rip.py:271), after appending the existing warnings note, also check `state.get("stages", {}).get("verify", {}).get("approved")` and append `" (verification manually approved)"` if truthy.

### Step 6 — Missing state file integration test

**Class:** `TestApproveMainMissingState` in `tests/test_approve.py`. Mirror `tests/test_no_active_state_notify.py:34-56`.

Tests:
18. `test_approve_missing_state_notifies_and_exits` — `sys.argv = ["rip.py", "--approve", "NONEXISTENT"]`. Patch `rip.notify`, `rip.analyze_error`. Assert `SystemExit(1)` raised, `notify` called with a message containing "FAILED" and "NONEXISTENT".

No new production code needed if Step 2's resolver raises `RuntimeError` and the existing `main()` exception handler catches it (lines 391-414).

### Step 7 — Transcode-also-failed edge case integration test

**Class:** `TestApproveMainHappyPath` (additional method).

Tests:
19. `test_approve_with_failed_transcode_still_fails_pipeline` — pre-create state with transcode=failed and verify=failed. `--approve LABEL`. Patch `rip.run_pipeline` to raise (simulating transcode re-failing). Patch `rip.analyze_error` (returns empty dict). Assert `SystemExit(1)`, but also assert that the state passed in had `stages.verify.status=="complete"` (so the user can see --approve did its part — the new failure is the underlying transcode bug).

This is a documentation test; no new production code.

## Step 8 - Last step

The last change is making the failure notification suggest `--approve` when a `VerificationError` fires with `fix_command=None` and `claude_verdict.verdict == "warn"`. That is the only condition where `--approve` is appropriate (don't suggest it on `verdict=fail`, which means Claude judged the rip genuinely bad). Implementation: in `rip.py:411-413`, after the existing `notify(conf, summary)`, add a second `notify(conf, approve_cmd)` when those conditions hold, where `approve_cmd` is built programmatically from `disc_label` and `RIP_DIR`. Tests would live in a separate `tests/test_verification_error_notify.py`. Splitting from this plan keeps the diff reviewable; the approve flag is independently useful.

## Verification

End-to-end manual test:

1. Find an actual failed-verify state in `.state/` (e.g., reproduce by running on the Korra disc, or restore the Korra state from `.state/history/Legend_Of_Korra_Book_2_Disc_1-20260424T230712.json`).
2. Confirm the .mp4 files exist locally in `TV/Legend Of Korra/Season 02/`.
3. Run systemd-run rip.py --approve Legend_Of_Korra_Book_2_Disc_1, like recommended in the README.md.
4. Expect: pipeline runs sync (rsync to backup host) and finalize stages, sends a success Matrix notification with "(verification manually approved)" appended, and archives the state to `.state/history/`.
5. Verify on the backup host that the .mp4 files arrived in the Jellyfin TV directory.
6. Verify `.state/history/Legend_Of_Korra_Book_2_Disc_1-<timestamp>.json` contains both `verification.issues` (the original count_mismatch) AND `stages.verify.approved == true`.

Test suite: `cd dvd-ripper && python3 -m unittest discover tests` — all existing tests pass, all new tests pass.
