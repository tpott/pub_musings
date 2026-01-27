# pdb Debugger Skill

A Claude skill for debugging Python programs using pdb (Python Debugger) with **automated debugging capabilities** and proper PTY (pseudo-terminal) support.

## Overview

This skill enables both **automated (Claude-driven)** and **interactive (human-guided)** debugging of Python programs. It includes:

- **Automated debugging tools** - Claude can run debugging sessions programmatically to collect data and test hypotheses
- **PTY-enabled pdb runner** - Handles the pseudo-terminal requirements that pdb needs
- **Comprehensive documentation** - Instructions for debugging scripts, servers, CLIs, and more
- **Ready-to-use examples** - Working examples for both automated and manual debugging
- **Best practices** - Effective debugging workflows and techniques

## Key Capabilities

### 🤖 Automated Debugging (Claude-Driven)

Claude can now **autonomously debug code** by:
1. Analyzing code and forming hypotheses about bugs
2. Designing pdb command sequences to test hypotheses
3. Running debugging sessions programmatically
4. Analyzing output to collect data about actual behavior
5. Iterating with refined hypotheses until the bug is found

**Example**:
```bash
# Claude runs multiple focused debugging sessions
python scripts/debug_session.py buggy.py "b calculate" "c" "p input" "p result" "q" --output session1.log
python scripts/debug_session.py buggy.py "b calculate" "c" "l" "n" "n" "p intermediate" "q" --output session2.log
```

### 👤 Interactive Debugging (Human-Guided)

For developers who want to debug manually:
```bash
python scripts/pdb_runner.py my_script.py
# User types pdb commands interactively
```

## Why This Skill?

Running pdb directly (e.g., `python -m pdb script.py`) in non-interactive environments often fails because pdb requires a pseudo-terminal (PTY) for its interactive features. This skill solves that problem with a helper script that automatically creates the required PTY.

## Installation

### For Claude.ai Users

1. Download this skill as a zip file
2. Go to Settings > Features in Claude.ai
3. Upload the zip file under "Skills"

### For Claude API Users

1. Create the skill via the Skills API:

```bash
curl https://api.anthropic.com/v1/skills \
  -H "anthropic-version: 2023-06-01" \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-beta: skills-2025-10-02" \
  -H "content-type: multipart/form-data" \
  -F "file=@pdb-debugger.zip"
```

2. Use the skill in your API requests by including its skill_id in the container parameter.

### For Claude Code Users

1. Place this directory in your Claude Code skills directory:
   - `~/.claude/skills/pdb-debugger/` (global)
   - Or `.claude/skills/pdb-debugger/` (project-specific)

2. Claude Code will automatically discover and use the skill.

## Quick Start

### Automated Debugging (Recommended)

When you ask Claude to debug code, it will use the automated approach:

```bash
# Claude designs and runs debugging sessions
python scripts/debug_session.py script.py "b function" "c" "p variable" "q"

# Save output for analysis
python scripts/debug_session.py app.py "c" "w" "p locals()" "q" --output debug.log

# Multiple sessions to iterate on hypotheses
python scripts/debug_session.py buggy.py "b 42" "c" "n" "p state" "q" --output session1.log
python scripts/debug_session.py buggy.py "b helper" "c" "a" "l" "q" --output session2.log
```

### Interactive Debugging (Manual)

For hands-on debugging:

```bash
# Debug a simple script
python scripts/pdb_runner.py my_script.py

# Debug with arguments
python scripts/pdb_runner.py my_script.py --input data.csv --verbose
```

### Server/Service Debugging

```bash
# Flask app
python scripts/pdb_runner.py -m flask run

# Django development server
python scripts/pdb_runner.py manage.py runserver

# FastAPI with uvicorn
python scripts/pdb_runner.py -m uvicorn main:app --reload

# Generic Python server
python scripts/pdb_runner.py server.py --port 8000
```

### CLI Tool Debugging

```bash
# Debug any CLI tool
python scripts/pdb_runner.py cli_tool.py process --config settings.json

# Debug installed package CLI
python scripts/pdb_runner.py -m mypackage.cli analyze data/
```

## Files in This Skill

### Core Files
- **SKILL.md** - Main skill instructions that Claude reads (includes automated debugging workflow)
- **scripts/debug_session.py** - **KEY TOOL**: Enables Claude to run pdb commands programmatically
- **scripts/pdb_runner.py** - PTY-enabled pdb launcher for interactive debugging
- **scripts/auto_postmortem.py** - Automatically enter debugger on crash

### Documentation
- **AUTOMATED_DEBUGGING_GUIDE.md** - **Essential reading**: Complete examples of Claude's debugging workflow
- **EXAMPLES.md** - Comprehensive examples for different debugging scenarios
- **README.md** - This file
- **QUICK_REFERENCE.md** - pdb command reference
- **TESTING.md** - How to verify the skill works

### Demo
- **demo.py** - Test script with intentional bugs for practice

## How It Works

When you ask Claude to debug Python code, Claude will:

### Automated Debugging Mode (Default for Bug Investigation)

1. **Analyze the code** and form a hypothesis about the bug location
2. **Design a debugging session** - specify pdb commands to test the hypothesis:
   - Set breakpoints at strategic locations
   - Inspect variable values
   - Step through execution
   - Examine stack traces
3. **Run `debug_session.py`** to execute the commands and capture output
4. **Analyze the output** - compare expected vs actual behavior
5. **Refine hypothesis** based on findings
6. **Iterate** - run additional sessions with refined commands
7. **Report findings** and suggest fixes

**Example of Claude's thought process**:
```
Hypothesis: Bug is in divide() function when b=0
Session 1: "b divide" "c" "p a" "p b" "q" → Confirms b=0
Session 2: "b divide" "c" "l" "q" → Shows no zero check
Conclusion: Add if b == 0: raise ValueError
```

### Interactive Debugging Mode (For Human-Guided Exploration)

1. Recognize the debugging request from the skill's description
2. Read the SKILL.md instructions on how to use pdb
3. Use the `pdb_runner.py` helper script to create a proper PTY environment
4. Guide you through setting breakpoints, inspecting variables, and stepping through code

Both modes use the `pdb_runner.py` or `debug_session.py` scripts which create a PTY using Python's `pty` module, enabling pdb's interactive features.

## Common Use Cases

### Automated Debugging Excels At:

1. **Hypothesis Testing** - Claude forms hypotheses and designs sessions to test them
2. **Iterative Investigation** - Multiple focused debugging sessions to narrow down issues
3. **Data Collection** - Systematic gathering of variable states and execution flow
4. **Root Cause Analysis** - Tracing problems back to their source
5. **Comparing Expected vs Actual** - Validating assumptions about program behavior

### When to Use Each Mode:

**Use Automated Debugging when**:
- You want Claude to investigate a bug autonomously
- The bug needs systematic data collection
- Multiple debugging runs would be helpful
- You want to see Claude's reasoning about the bug

**Use Interactive Debugging when**:
- You want to explore the code manually
- Learning how pdb works
- Real-time investigation with human intuition
- Debugging servers that need live interaction

## Common Use Cases

This skill helps with:

1. **Finding bugs** - Set breakpoints and inspect program state
2. **Understanding code flow** - Step through execution line by line
3. **Debugging servers** - Break on specific HTTP requests
4. **Debugging CLI tools** - Inspect argument parsing and processing
5. **Post-mortem debugging** - Analyze crashes after they occur
6. **Testing code paths** - Verify logic with specific inputs

## Essential pdb Commands

Once the debugger starts, use these commands:

- `n` - Execute next line
- `s` - Step into function
- `c` - Continue to next breakpoint
- `p <expr>` - Print expression value
- `l` - List source code
- `b <line>` - Set breakpoint
- `w` - Show stack trace
- `q` - Quit debugger

For complete command reference, see SKILL.md.

## Examples

See EXAMPLES.md for detailed examples including:

- Debugging simple scripts
- Debugging Flask/Django web servers
- Debugging CLI tools with arguments
- Debugging async applications
- Post-mortem debugging after crashes
- Setting conditional breakpoints

## Troubleshooting

### "Not a terminal" errors

Always use `scripts/pdb_runner.py` instead of running `python -m pdb` directly. The runner script creates the required PTY.

### Debugger not stopping at breakpoints

1. Make sure you used `c` (continue) to start execution
2. Verify your breakpoint syntax: `b filename.py:42`
3. Check that the code path actually executes

### Server exits immediately

After starting the server under pdb, use `c` to continue past initialization code. The debugger will break when your breakpoints are hit (e.g., when handling requests).

## Requirements

- Python 3.6+
- No additional packages required (uses standard library only)

## Contributing

Found an issue or have a suggestion? The skill can be extended with:

- Additional helper scripts for common debugging patterns
- More examples for different frameworks
- Integration with other debugging tools

## License

This skill is provided as-is for use with Claude.
