"""Shared wrapper for invoking the Claude CLI with a JSON-output contract.

Callers supply a system prompt that instructs Claude to return JSON-only. This
module handles CLI invocation and lenient JSON extraction (tolerating prose
preambles, trailing text, and markdown fences around the object), merging any
failure note into the caller-supplied fallback dict.
"""

import json
import os
import re
import shutil
import subprocess
from pathlib import Path


def _balanced_brace_spans(text):
    """Return each top-level balanced ``{...}`` substring, ignoring braces
    that appear inside JSON strings (so escaped quotes and literal braces in
    string values don't throw off the depth count)."""
    spans = []
    depth = 0
    start = None
    in_str = False
    escape = False
    for i, ch in enumerate(text):
        if in_str:
            if escape:
                escape = False
            elif ch == "\\":
                escape = True
            elif ch == '"':
                in_str = False
            continue
        if ch == '"':
            in_str = True
        elif ch == "{":
            if depth == 0:
                start = i
            depth += 1
        elif ch == "}" and depth > 0:
            depth -= 1
            if depth == 0:
                spans.append(text[start:i + 1])
    return spans


def _extract_json(text):
    """Best-effort extraction of a single JSON object from model output.

    Models sometimes ignore "JSON only" instructions and wrap the object in a
    prose preamble, a trailing explanation, or a ```fence```. Tries, in order:
      1. The whole stripped text as JSON.
      2. The body of a ```...``` fenced block.
      3. The largest balanced ``{...}`` span that parses (the real verdict
         dwarfs any stray brace in surrounding prose).
    Returns the parsed dict, or None if nothing yields a JSON object.
    """
    text = (text or "").strip()
    if not text:
        return None

    try:
        return json.loads(text)
    except json.JSONDecodeError:
        pass

    fence = re.search(r"```(?:json)?\s*(.*?)```", text, re.DOTALL)
    if fence:
        try:
            return json.loads(fence.group(1).strip())
        except json.JSONDecodeError:
            pass

    best = None
    for span in _balanced_brace_spans(text):
        try:
            obj = json.loads(span)
        except json.JSONDecodeError:
            continue
        if best is None or len(span) > best[0]:
            best = (len(span), obj)
    return best[1] if best else None


def is_available(conf):
    """True if the claude CLI is installed and invocable."""
    claude_bin = conf.get("CLAUDE_BIN", "claude")
    if os.path.isabs(claude_bin):
        return Path(claude_bin).exists()
    return bool(shutil.which(claude_bin))


def run(prompt, system_prompt, conf, fallback):
    """Invoke Claude CLI and parse JSON response.

    Returns the parsed JSON dict on success, or a copy of `fallback` with
    `recommendation` overwritten to describe the failure plus an explicit
    sentinel key (`_cli_failed` or `_parse_failed`) so callers can detect
    that the verifier never actually ran rather than inferring it from the
    fallback verdict (which would otherwise look like a benign result).
    """
    claude_bin = conf.get("CLAUDE_BIN", "claude")
    model = conf.get("VERIFY_MODEL") or "sonnet"
    cmd = [
        claude_bin, "--print",
        "--dangerously-skip-permissions",
        "--system-prompt", system_prompt,
        "--model", model,
    ]

    cwd = conf.get("RIP_DIR")
    if cwd and not Path(cwd).is_dir():
        cwd = None

    print("+ claude --print", flush=True)
    proc = subprocess.Popen(
        cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
        stderr=subprocess.PIPE, text=True, cwd=cwd,
    )
    stdout, stderr = proc.communicate(input=prompt)

    result = dict(fallback)
    if proc.returncode != 0:
        result["recommendation"] = (
            f"Claude CLI failed (rc={proc.returncode}): {stderr[:500]}"
        )
        result["_cli_failed"] = True
        return result

    parsed = _extract_json(stdout)
    if isinstance(parsed, dict):
        return parsed

    result["recommendation"] = (
        f"Could not parse Claude response: {stdout[:500]}"
    )
    result["_parse_failed"] = True
    return result
