---
name: pdb-debugger
description: Debug Python programs using pdb (Python debugger). Use when the user wants to debug Python code, set breakpoints, step through execution, inspect variables, or run programs under a debugger. Also use for translating python commands to run under pdb, especially for servers, REPLs, and CLI tools. Supports both interactive (human-guided) and automated (Claude-driven) debugging sessions.
---

# Python Debugger (pdb) Skill

This skill helps you debug Python programs using pdb, Python's built-in debugger. It supports two modes:

1. **Interactive Mode**: Guide users through manual debugging sessions
2. **Automated Mode**: Claude runs debugging sessions programmatically to collect data and form hypotheses

## When to Use This Skill

Use this skill when:
- User wants to debug a Python program or understand why code isn't working as expected
- User mentions breakpoints, stepping through code, or inspecting variables
- User wants to run a server, REPL, or CLI tool under the debugger
- User needs to troubleshoot Python runtime issues
- User asks to "debug this", "run this in the debugger", or "find the bug"
- **User wants Claude to investigate a bug** - Use automated mode to iteratively collect data

## Key Concepts

### The PTY Requirement

pdb requires a pseudo-terminal (PTY) to function properly because it needs interactive terminal features like:
- Reading user input character-by-character
- Terminal control sequences for line editing
- Proper signal handling (Ctrl+C, Ctrl+D)

Running pdb directly in a non-PTY environment will fail or behave unpredictably. This skill includes helper scripts that create a PTY automatically.

### Two Debugging Modes

**Interactive Mode** (`pdb_runner.py`): For human-guided debugging sessions
- Start pdb and interact manually
- Good for exploration and learning
- User types commands directly

**Automated Mode** (`debug_session.py`): For Claude-driven debugging
- Claude specifies commands programmatically
- Runs session and analyzes output
- Enables iterative hypothesis testing
- Multiple debugging runs encouraged

## Automated Debugging Workflow (RECOMMENDED FOR CLAUDE)

When a user asks Claude to debug code, Claude should use the automated debugging workflow to systematically investigate issues.

### The Iterative Debugging Process

1. **Analyze the code and error** (if provided)
2. **Form initial hypothesis** about where/why the bug occurs
3. **Design a debugging session** to test the hypothesis
4. **Run automated pdb session** to collect data
5. **Analyze the output** - compare expected vs actual behavior
6. **Refine hypothesis** based on findings
7. **Repeat** with new debugging sessions as needed

### Using debug_session.py

The `debug_session.py` tool allows Claude to run pdb commands programmatically:

```bash
# Basic usage - commands as arguments
python scripts/debug_session.py script.py "b function_name" "c" "p variable" "q"

# Module execution
python scripts/debug_session.py -m package.module "b function" "c" "n" "p x" "q"

# Save output for analysis
python scripts/debug_session.py script.py "b 42" "c" "w" "l" "q" --output debug1.log

# Load commands from file
python scripts/debug_session.py script.py --file debug_commands.txt
```

### Example: Claude's Debugging Session Design

**User reports**: "My calculator script gives wrong results for division"

**Claude's thought process**:
1. Hypothesis: Bug likely in divide() function
2. Need to check: input values, calculation, return value
3. Design session: Set breakpoint, continue to function, inspect variables

**Claude runs**:
```bash
python scripts/debug_session.py calculator.py "b divide" "c" "p a" "p b" "n" "n" "p result" "q" --output debug1.log
```

**Claude analyzes output**:
- Sees `a=10`, `b=3`, `result=3` (integer division!)
- Hypothesis confirmed: Using `//` instead of `/`

**Claude may run additional session**:
```bash
python scripts/debug_session.py calculator.py "b divide" "c" "l" "q" --output debug2.log
```

**Claude views code context** and confirms the fix needed.

### Designing Effective Debugging Sessions

**Start with high-level reconnaissance**:
```bash
# Where does it crash?
"c" "w" "l" "q"

# What's the state at the error?
"c" "p locals()" "w" "q"
```

**Narrow down to specific function**:
```bash
# Break at suspected function
"b problematic_function" "c" "a" "l" "q"
```

**Inspect execution flow**:
```bash
# Step through and watch variables
"b function" "c" "n" "p x" "n" "p x" "n" "p result" "q"
```

**Compare expected vs actual**:
```bash
# Check intermediate calculations
"b calculation" "c" "p input_val" "n" "n" "p intermediate" "n" "p final" "q"
```

### Multiple Debugging Runs Are Encouraged

Don't try to solve everything in one session. Run multiple focused sessions:

**Session 1**: Locate the problem area
```bash
python scripts/debug_session.py app.py "c" "w" "q" --output locate.log
```

**Session 2**: Inspect state at problem location  
```bash
python scripts/debug_session.py app.py "b app.py:42" "c" "p locals()" "q" --output inspect.log
```

**Session 3**: Step through the logic
```bash
python scripts/debug_session.py app.py "b app.py:42" "c" "n" "p var1" "n" "p var2" "q" --output trace.log
```

**Session 4**: Test the fix hypothesis
```bash
# After proposing a fix, verify understanding
python scripts/debug_session.py app.py "b app.py:42" "c" "p condition" "l" "q" --output verify.log
```

### Reading Debug Session Output

The output shows each command and pdb's response:

```
(Pdb) b calculate
Breakpoint 1 at /path/to/script.py:10
(Pdb) c
> /path/to/script.py(11)calculate()
-> result = a / b
(Pdb) p a
10
(Pdb) p b
0
(Pdb) q
```

Claude should:
1. Look for the actual values of variables
2. Compare to expected values
3. Note the line numbers and code context
4. Identify discrepancies

### Common Debugging Command Patterns

**Initial Investigation**:
- `"c" "w" "l" "p locals()" "q"` - Where are we? What's the state?

**Function Entry Point**:
- `"b function_name" "c" "a" "l" "q"` - What arguments were passed?

**Step Through Logic**:
- `"b 42" "c" "n" "p x" "n" "p y" "n" "p result" "q"` - Trace execution

**Inspect Data Structures**:
- `"b function" "c" "p len(data)" "pp data" "q"` - Check collections

**Stack Analysis**:
- `"c" "w" "up" "p variable" "down" "q"` - Examine call stack

## Quick Start

### Automated Debugging (Claude-Driven)

**When Claude is asked to debug code**, use this approach:

```bash
# Run a debugging session with specified commands
python scripts/debug_session.py script.py "b main" "c" "p variable" "q"

# Multiple commands to collect data
python scripts/debug_session.py app.py "b problematic_func" "c" "a" "l" "n" "p result" "q"

# Save output for analysis
python scripts/debug_session.py buggy.py "c" "w" "p locals()" "q" --output session1.log
```

### Interactive Debugging (Human-Guided)

**When guiding a human through debugging**:

```bash
# Start interactive pdb session
python scripts/pdb_runner.py target_script.py [args...]

# The user will then type pdb commands manually
```

### Debugging Servers, REPLs, and CLI Tools (Both Modes)

Translate any Python command to run under pdb:

```bash
# Interactive mode
python scripts/pdb_runner.py server.py --port 8000

# Automated mode
python scripts/debug_session.py server.py "b handle_request" "c" "p request.path" "q"
```

## Common Use Cases

### 1. Debugging a Web Server

```bash
# Django development server
python scripts/pdb_runner.py manage.py runserver

# Flask application
python scripts/pdb_runner.py -m flask run --debug

# FastAPI with uvicorn
python scripts/pdb_runner.py -m uvicorn main:app --reload
```

### 2. Debugging a CLI Tool

```bash
# Custom CLI script
python scripts/pdb_runner.py my_cli.py --verbose --config config.json

# Installed module
python scripts/pdb_runner.py -m my_package.cli analyze data.csv
```

### 3. Debugging a REPL or Interactive Tool

```bash
# Custom REPL
python scripts/pdb_runner.py repl.py

# IPython-based tool
python scripts/pdb_runner.py -m my_interactive_tool
```

### 4. Debugging with Breakpoints Set

Insert breakpoints in code before running:

```python
# In your Python file
def problematic_function(data):
    import pdb; pdb.set_trace()  # Debugger will stop here
    # ... rest of function
```

Then run with the helper script:

```bash
python scripts/pdb_runner.py my_script.py
```

## Essential pdb Commands

Once in the debugger, use these commands:

### Navigation
- `n` (next) - Execute current line, step over function calls
- `s` (step) - Execute current line, step into function calls  
- `c` (continue) - Continue execution until next breakpoint
- `r` (return) - Continue until current function returns
- `unt <lineno>` (until) - Continue until line number reached

### Inspection
- `p <expr>` (print) - Evaluate and print expression
- `pp <expr>` (pretty-print) - Pretty-print expression
- `l` (list) - Show source code around current line
- `ll` (longlist) - Show source for entire current function
- `w` (where) - Show stack trace
- `a` (args) - Print arguments of current function

### Breakpoints
- `b <lineno>` - Set breakpoint at line in current file
- `b <file>:<lineno>` - Set breakpoint at line in specific file  
- `b <function>` - Set breakpoint at first line of function
- `cl` (clear) - Clear all breakpoints
- `cl <bpnum>` - Clear specific breakpoint

### Control
- `q` (quit) - Exit debugger (terminates program)
- `h` (help) - Show help
- `h <command>` - Show help for specific command

## Workflow Guide

### Standard Debugging Workflow

1. **Start the program under pdb**:
   ```bash
   python scripts/pdb_runner.py your_script.py
   ```

2. **Set breakpoints at key locations**:
   ```
   (Pdb) b problematic_function
   (Pdb) b my_module.py:42
   ```

3. **Run to first breakpoint**:
   ```
   (Pdb) c
   ```

4. **Inspect variables and state**:
   ```
   (Pdb) p local_var
   (Pdb) pp complex_object.__dict__
   (Pdb) w
   ```

5. **Step through execution**:
   ```
   (Pdb) n    # step to next line
   (Pdb) s    # step into function
   (Pdb) l    # see code context
   ```

6. **Continue or quit**:
   ```
   (Pdb) c    # continue to next breakpoint
   (Pdb) q    # quit debugger
   ```

### Debugging Server Applications

When debugging web servers or long-running applications:

1. **Set breakpoints before starting**:
   - Add `import pdb; pdb.set_trace()` in request handlers
   - Or use conditional breakpoints in pdb after starting

2. **Start server under pdb**:
   ```bash
   python scripts/pdb_runner.py server.py
   ```

3. **Let server start** (continue past initialization):
   ```
   (Pdb) c
   ```

4. **Trigger the code path** (make HTTP request, etc.)
   - Server will break at your breakpoint
   
5. **Inspect request state**, step through handler logic

6. **Continue** to handle next request:
   ```
   (Pdb) c
   ```

## Advanced Techniques

### Post-Mortem Debugging

If your script crashes, you can debug at the point of failure:

```python
import pdb
import sys

def main():
    # Your code here
    problematic_function()

if __name__ == "__main__":
    try:
        main()
    except Exception:
        pdb.post_mortem(sys.exc_info()[2])
```

### Conditional Breakpoints

Set breakpoints that only trigger when a condition is true:

```
(Pdb) b my_function, x > 100
(Pdb) b module.py:42, user_id == "debug_user"
```

### Programmatic Breakpoints

For complex conditions or temporary debugging:

```python
if some_complex_condition():
    import pdb; pdb.set_trace()
```

## Helper Script Reference

The `scripts/pdb_runner.py` script handles PTY creation and supports:

- **Direct script execution**: `pdb_runner.py script.py args`
- **Module execution**: `pdb_runner.py -m module args`  
- **Automatic PTY setup**: Creates pseudo-terminal automatically
- **Argument forwarding**: All arguments after script/module pass through
- **Signal handling**: Proper Ctrl+C behavior

## Troubleshooting

### "Not a terminal" errors
- Use `scripts/pdb_runner.py` instead of direct `python -m pdb`
- The helper script creates the required PTY

### Debugger not stopping at breakpoints
- Verify breakpoint syntax: `b filename.py:lineno`
- Check that you used `c` to continue execution
- Ensure breakpoint is in code that actually executes

### Server immediately exits
- Did you use `c` to continue past initialization?
- Check for early exit conditions in server startup

### Can't see local variables
- Use `p` or `pp` commands, not just variable name
- Use `a` to see function arguments
- Use `w` to verify you're in the expected function

## Automated Debugging with Claude

Claude can automate debugging workflows using the `debug_session.py` script. This enables a systematic, hypothesis-driven debugging approach:

### Automated Debugging Workflow

1. **Analyze the problem** - Form hypothesis about where the bug is
2. **Design debugging session** - Specify pdb commands to test hypothesis
3. **Run debug session** - Execute with `debug_session.py` and collect data
4. **Analyze output** - Review variable values, compare expected vs actual
5. **Refine hypothesis** - Based on findings, form new hypothesis
6. **Iterate** - Design new session and repeat until bug found

### Using debug_session.py

The `debug_session.py` tool allows Claude to run pdb commands programmatically:

```bash
# Basic usage
python scripts/debug_session.py script.py "b function" "c" "p variable" "q"

# Multiple sessions to collect different data
python scripts/debug_session.py app.py "c" "w" "q" --output locate.log
python scripts/debug_session.py app.py "b app.py:42" "c" "p locals()" "q" --output inspect.log
```

### Example: Automated Debugging Session

**User reports**: "My calculator gives wrong division results"

**Claude's approach**:

```bash
# Session 1: Locate the problem
python scripts/debug_session.py calculator.py "b divide" "c" "a" "l" "q" --output debug1.log

# Session 2: Inspect variables
python scripts/debug_session.py calculator.py "b divide" "c" "p a" "p b" "n" "p result" "q" --output debug2.log

# Session 3: Check calculation type
python scripts/debug_session.py calculator.py "b divide" "c" "p type(a)" "p type(b)" "l" "q" --output debug3.log
```

Claude analyzes each output, forms hypotheses, and designs the next session accordingly.

### Hypothesis-Driven Debugging

Claude excels at:

1. **Forming hypotheses** based on:
   - Error messages and tracebacks
   - Code analysis
   - Expected vs actual behavior

2. **Designing sessions**:
   - Where to set breakpoints to test hypothesis
   - What variables to inspect
   - What commands reveal the most information

3. **Analyzing results**:
   - Identifying suspicious values (None, 0, empty, negative)
   - Comparing actual vs expected values
   - Spotting logic errors

4. **Iterating**:
   - Refining hypothesis based on data
   - Running multiple focused sessions
   - Each session narrows the search

### When to Use Automated Debugging

Use automated debugging when:
- Need to systematically collect data about program state
- Want to test specific hypotheses
- Need to trace variable values through execution
- Analyzing complex execution flows
- Multiple debugging runs would be valuable

## Examples

See `EXAMPLES.md` for complete working examples of both interactive and automated debugging.

See `AUTOMATED_DEBUGGING_GUIDE.md` for detailed examples showing Claude's complete debugging workflows with hypothesis formation, session design, output analysis, and iteration.
