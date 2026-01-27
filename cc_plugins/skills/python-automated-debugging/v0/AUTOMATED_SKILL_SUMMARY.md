# pdb Debugger Skill - Complete Package (With Automated Debugging)

## 🎯 Core Innovation: Claude Can Now Debug Code Autonomously

This skill enables Claude to **automatically debug Python programs** by running pdb sessions programmatically, collecting data, forming hypotheses, and iterating until the bug is found.

## What Makes This Special

### Before This Skill
- Claude could only *explain* debugging concepts
- Users had to run debuggers manually
- No way to collect actual runtime data
- Debugging was purely theoretical

### With This Skill
- ✅ Claude **runs** debugging sessions automatically
- ✅ Claude **collects** real runtime data (variable values, stack traces)
- ✅ Claude **forms and tests hypotheses** systematically
- ✅ Claude **iterates** through multiple debugging sessions
- ✅ Claude provides **data-driven** bug analysis

## The Two Modes

### 🤖 Automated Debugging (Primary Use Case)

**When to use**: User asks Claude to debug code, find bugs, or investigate issues

**What Claude does**:
1. Analyzes code and error messages
2. Forms hypothesis about bug location
3. Designs pdb command sequence to test hypothesis
4. Runs `debug_session.py` to collect data
5. Analyzes output (variable values, execution flow)
6. Refines hypothesis based on findings
7. Iterates with new sessions until bug is found

**Example workflow**:
```bash
# Session 1: Where does it crash?
python scripts/debug_session.py buggy.py "c" "w" "q" --output locate.log

# Session 2: What's the state at that point?
python scripts/debug_session.py buggy.py "b buggy.py:42" "c" "p locals()" "q" --output inspect.log

# Session 3: Step through the logic
python scripts/debug_session.py buggy.py "b buggy.py:42" "c" "n" "p x" "n" "p y" "q" --output trace.log

# Claude analyzes all three outputs and identifies the bug
```

### 👤 Interactive Debugging (Secondary Use Case)

**When to use**: User wants to debug manually, learn pdb, or explore interactively

**What it does**:
```bash
python scripts/pdb_runner.py script.py
# User types pdb commands manually: b, c, n, p, etc.
```

## File Structure

### 📋 Core Documentation

1. **SKILL.md** (16 KB)
   - Main instructions Claude reads
   - Automated debugging workflow explained
   - PTY concepts and requirements
   - Command reference

2. **AUTOMATED_DEBUGGING_GUIDE.md** (12 KB) ⭐ **ESSENTIAL**
   - Complete examples of Claude's debugging process
   - Shows hypothesis formation → session design → output analysis → iteration
   - 4 detailed walkthroughs with real bugs
   - Command patterns for different scenarios

3. **README.md** (9.5 KB)
   - Installation and quick start
   - Explains both automated and interactive modes
   - Use case guidelines

### 🛠️ Core Tools

4. **scripts/debug_session.py** (6 KB) ⭐ **PRIMARY TOOL**
   - Enables automated debugging
   - Takes pdb commands as arguments
   - Captures output for Claude to analyze
   - Usage: `python scripts/debug_session.py script.py "b func" "c" "p var" "q"`

5. **scripts/pdb_runner.py** (2 KB)
   - For interactive debugging
   - Creates PTY automatically
   - Usage: `python scripts/pdb_runner.py script.py`

6. **scripts/auto_postmortem.py** (3.5 KB)
   - Automatically enters debugger on crash
   - Alternative to setting breakpoints

### 📚 Additional Documentation

7. **EXAMPLES.md** (12 KB)
   - Traditional debugging examples
   - Flask, Django, CLI tools
   - Both interactive and automated examples

8. **QUICK_REFERENCE.md** (7.5 KB)
   - pdb command reference
   - Tables and cheat sheets
   - Quick lookup for experienced users

9. **TESTING.md** (7 KB)
   - How to verify the skill works
   - Test scenarios
   - Expected outcomes

10. **demo.py** (2 KB)
    - Sample buggy script for testing

## The PTY Problem & Solution

### The Problem
pdb requires a pseudo-terminal (PTY) for:
- Character-by-character input
- Line editing
- Signal handling (Ctrl+C, Ctrl+D)

Running `python -m pdb` in non-interactive environments fails.

### The Solution
All our scripts use `pty.spawn()` or `pty.openpty()` to create PTYs automatically:

```python
import pty
pdb_cmd = [sys.executable, '-m', 'pdb'] + args
pty.spawn(pdb_cmd)  # Creates PTY transparently
```

Users/Claude never need to worry about PTYs - it "just works".

## How Claude Uses This Skill

### The Debugging Workflow

**User**: "This script calculates wrong averages"

**Claude's Internal Process**:

1. **Analyze Code**
   - Looks for division, edge cases
   - Hypothesis: Might not handle empty lists

2. **Design Session 1** - Confirm crash location
   ```bash
   python scripts/debug_session.py script.py "c" "w" "q"
   ```

3. **Analyze Output 1**
   - Sees crash on line 4: `return total / count`
   - Hypothesis refined: Division by zero when count=0

4. **Design Session 2** - Inspect state
   ```bash
   python scripts/debug_session.py script.py "b calculate" "c" "c" "c" "p numbers" "p count" "q"
   ```

5. **Analyze Output 2**
   - Third call: `numbers=[]`, `count=0`
   - Hypothesis confirmed!

6. **Report to User**
   - "The bug is on line 4. When `calculate()` receives an empty list, `count` becomes 0, causing division by zero."
   - Suggests fix: Add empty list check

### Key Advantages for Claude

1. **Data-Driven**: Makes decisions based on actual runtime values, not guesses
2. **Iterative**: Can run 3-5 focused sessions to narrow down issues
3. **Systematic**: Follows hypothesis → test → refine pattern
4. **Concrete**: Shows exact variable values at exact execution points

## Command Translation Examples

The skill teaches Claude to translate any Python command to run under debugger:

```bash
# Original → Automated Debugging
python server.py --port 8000
→ python scripts/debug_session.py server.py "b handle_request" "c" "p request" "q"

python -m flask run
→ python scripts/debug_session.py -m flask "b view_function" "c" "a" "q"

python manage.py runserver
→ python scripts/debug_session.py manage.py runserver "b view" "c" "l" "q"
```

## Installation

### Claude.ai
1. Download pdb-debugger.zip
2. Settings > Features > Upload
3. Claude automatically uses when asked to debug

### Claude API
```bash
curl https://api.anthropic.com/v1/skills \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-beta: skills-2025-10-02" \
  -F "file=@pdb-debugger.zip"
```

### Claude Code
```bash
cp -r pdb-debugger ~/.claude/skills/
```

## Real-World Example

**User uploads code**:
```python
def factorial(n):
    result = n
    for i in range(n-1, 1, -1):
        result *= i
    return result

print(f"0! = {factorial(0)}")  # Gets 0, should be 1
```

**Claude's Debugging Sessions**:

**Session 1**:
```bash
python scripts/debug_session.py factorial.py "b factorial" "c" "p n" "a" "q" --output s1.log
```
Output: `n = 0`, `result = 0`

**Session 2**:
```bash
python scripts/debug_session.py factorial.py "b factorial" "c" "n" "n" "p list(range(n-1, 1, -1))" "q" --output s2.log
```
Output: `range(n-1, 1, -1) = []` (empty range!)

**Claude's Analysis**:
"When n=0, the loop doesn't execute because range(-1, 1, -1) is empty. So `result` stays 0. The fix is to handle n=0 as a special case that returns 1."

## Use Cases

### Excels At:
- Finding calculation errors
- Debugging edge cases
- Tracing variable changes
- Understanding control flow
- Investigating crashes
- Validating hypotheses

### Works With:
- Simple scripts
- Web servers (Flask, Django, FastAPI)
- CLI tools
- Data processing pipelines
- Test failures
- Async code

## Metrics

- **Total size**: ~150 KB uncompressed, ~40 KB zipped
- **Load time**: Metadata always loaded (~100 tokens), SKILL.md loaded on trigger (~6,000 tokens)
- **Dependencies**: None (Python standard library only)
- **Platform**: Linux/macOS (requires PTY support)

## What Sets This Apart

1. **First automated Python debugging skill** for Claude
2. **Hypothesis-driven workflow** matches how expert debuggers think
3. **Multiple iterations encouraged** - collect data, refine, repeat
4. **Real runtime data** not theoretical analysis
5. **Works with any Python code** - scripts, servers, modules, tests

## Testing the Skill

Quick verification:
```bash
cd pdb-debugger
python scripts/debug_session.py demo.py "b calculate_factorial" "c" "p n" "q"
```

If you see pdb output with variable values, it's working!

## Design Philosophy

### Progressive Disclosure
- Metadata: Always loaded (lightweight)
- SKILL.md: Loaded when debugging request detected
- AUTOMATED_DEBUGGING_GUIDE.md: Loaded when Claude needs detailed examples
- Other docs: Loaded as referenced

### Hypothesis-Driven
- Form hypothesis → Design session → Collect data → Analyze → Refine
- Matches expert debugging methodology
- Each session tests one specific hypothesis

### Iteration-Friendly
- Multiple short sessions > one long session
- Each session provides new data
- Claude gets better with each iteration

## Future Enhancements

Potential additions:
- Breakpoint injection (modify files to add breakpoints)
- Pattern detection (common bug patterns)
- Regression testing (run debugger on test suites)
- Remote debugging support
- Integration with more frameworks

## License

Provided as-is for use with Claude products.

---

**Version**: 2.0 (Automated Debugging Edition)  
**Created**: December 2024  
**Skill ID**: pdb-debugger  
**Category**: Development Tools  
**Primary Language**: Python 3.6+  

**Key Innovation**: Claude can now autonomously debug Python code through automated pdb sessions
