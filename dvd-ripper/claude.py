"""Shared wrapper for invoking the Claude CLI with a JSON-output contract.

Callers supply a system prompt that instructs Claude to return JSON-only. This
module handles CLI invocation, markdown-fence stripping, and JSON parsing,
merging any failure note into the caller-supplied fallback dict.
"""

import json
import os
import shutil
import subprocess
from pathlib import Path


def is_available(conf):
    """True if the claude CLI is installed and invocable."""
    claude_bin = conf.get("CLAUDE_BIN", "claude")
    if os.path.isabs(claude_bin):
        return Path(claude_bin).exists()
    return bool(shutil.which(claude_bin))


def run(prompt, system_prompt, conf, fallback):
    """Invoke Claude CLI and parse JSON response.

    Returns the parsed JSON dict on success, or a copy of `fallback` with
    `recommendation` overwritten to describe the failure.
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
        return result

    text = stdout.strip()
    if text.startswith("```"):
        lines = text.split("\n")
        end = -1 if lines[-1].strip().startswith("```") else len(lines)
        text = "\n".join(lines[1:end]).strip()

    try:
        return json.loads(text)
    except json.JSONDecodeError:
        result["recommendation"] = (
            f"Could not parse Claude response: {stdout[:500]}"
        )
        return result
