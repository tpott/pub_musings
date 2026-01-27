# Automated Debugging Guide

This guide shows how Claude can automate debugging workflows using systematic, hypothesis-driven approaches.

## Overview

Automated debugging allows Claude to:
- **Form hypotheses** about where bugs occur
- **Design experiments** (set strategic breakpoints)
- **Collect data** systematically
- **Analyze results** to identify issues
- **Iterate** until the bug is found

This is more efficient than manual stepping and enables debugging of complex, intermittent, or hard-to-reproduce issues.

## The Automated Debugging Workflow

```
┌─────────────────────────────────────┐
│  1. Analyze Problem                 │
│     - Error message/traceback       │
│     - Unexpected behavior           │
│     - User description              │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  2. Form Hypothesis                 │
│     - Where might bug be?           │
│     - What data to inspect?         │
│     - What should happen vs what is?│
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  3. Generate Debug Plan             │
│     - Set strategic breakpoints     │
│     - Specify vars to collect       │
│     - Define success criteria       │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  4. Run Automated Debugger          │
│     - Execute with debug plan       │
│     - Collect data at breakpoints   │
│     - Export results to JSON        │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  5. Analyze Results                 │
│     - Review collected data         │
│     - Identify suspicious values    │
│     - Check if hypothesis confirmed │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│  6. Refine or Conclude              │
│     - Bug found? → Fix it!          │
│     - Not found? → New hypothesis   │
│     - Repeat from step 2            │
└─────────────────────────────────────┘
```

## Complete Example: Division By Zero Bug

### Scenario
User reports: "My calculator script crashes with 'division by zero' error"

### Step 1: Save the Error

```bash
$ python calculator.py
...
ZeroDivisionError: division by zero
  File "calculator.py", line 23, in calculate_average
    average = total / count
```

Save traceback to `error.txt`

### Step 2: Generate Initial Debug Plan

```bash
$ python scripts/debug_assistant.py plan-from-traceback calculator.py error.txt -o plan1.json
```

Generated plan (`plan1.json`):
```json
{
  "breakpoints": [
    {"file": "calculator.py", "line": 20},
    {"file": "calculator.py", "line": 23}
  ],
  "collect_vars": ["total", "count", "numbers"],
  "commands": ["c"],
  "hypothesis": "Investigating ZeroDivisionError at calculator.py:23. Will examine state leading up to the error.",
  "strategy": "Set breakpoints before and at error location to inspect variable state."
}
```

### Step 3: Run Automated Debugger

```bash
$ python scripts/automated_debugger.py calculator.py --plan plan1.json -o results1.json

[AUTO] Breakpoint set: calculator.py:20
[AUTO] Breakpoint set: calculator.py:23
[AUTO] Running calculator.py under automated debugger...

[AUTO] Executing: c
[AUTO] Executing: c
[AUTO] Results saved to: results1.json
```

### Step 4: Analyze Results

```bash
$ python scripts/debug_assistant.py analyze results1.json

Debug Session Summary
==================================================
Breakpoint hits: 2
Data points: 2

Execution trace:
--------------------------------------------------
1. [line] calculator.py:20 in calculate_average
   Source: total = sum(numbers)
   Variables:
     numbers = []
     total = 0

2. [line] calculator.py:23 in calculate_average
   Source: average = total / count
   Variables:
     numbers = []
     total = 0
     count = 0

Suspicious values:
--------------------------------------------------
calculator.py:20: numbers = []: Empty collection
calculator.py:20: count = 0: Negative value for size/count/index
calculator.py:23: numbers = []: Empty collection
calculator.py:23: count = 0: Negative value for size/count/index

Suggested next steps:
--------------------------------------------------
Found 4 suspicious values:
  - numbers at calculator.py:20: Empty collection
  - count = 0 at calculator.py:20: Negative value for size/count/index
  ... and 2 more

Next steps:
  - Set breakpoints before the lines where suspicious values appear
  - Trace back to see where these values are assigned
  - Check the logic that produces these values
```

### Step 5: Analysis & Conclusion

**Finding**: `numbers` is an empty list, causing `count = 0` and division by zero.

**Root cause**: Function is called with empty list, no validation.

**Fix**: Add input validation:
```python
def calculate_average(numbers):
    if not numbers:
        raise ValueError("Cannot calculate average of empty list")
    total = sum(numbers)
    count = len(numbers)
    return total / count
```

## Example 2: Logic Error in Algorithm

### Scenario
"My sorting function returns wrong results for certain inputs"

### Step 1: Form Hypothesis

**Hypothesis**: "The comparison logic might be inverted or the swap logic incorrect"

### Step 2: Create Targeted Debug Plan

Claude creates plan focusing on the sorting function:

```json
{
  "breakpoints": [
    {"file": "sort.py", "line": 15, "comment": "Start of main loop"},
    {"file": "sort.py", "line": 18, "condition": "i == 2", "comment": "Third iteration"}
  ],
  "collect_vars": ["arr", "i", "j", "current", "temp"],
  "commands": ["c"],
  "hypothesis": "Comparison or swap logic may be incorrect",
  "strategy": "Monitor array state and comparison results during sorting"
}
```

### Step 3: Run & Analyze

```bash
$ python scripts/automated_debugger.py sort.py --plan sort_plan.json
$ python scripts/debug_assistant.py analyze debug_results.json --var arr
```

Output shows:
```
Variable tracking: arr
--------------------------------------------------
sort.py:15: arr = [5, 2, 8, 1, 9]
sort.py:18: arr = [5, 2, 8, 1, 9]  # No change!
sort.py:15: arr = [5, 2, 8, 1, 9]  # Still no change
```

**Finding**: Array never changes → swap logic isn't executing or is broken.

Review swap code, find bug: `arr[i], arr[j] = arr[i], arr[j]` (swapping with itself!)

## Example 3: Intermittent Server Error

### Scenario
"API occasionally returns 500 error, hard to reproduce"

### Strategy: Automated Data Collection

```json
{
  "breakpoints": [
    {"file": "api.py", "line": 45, "condition": "status_code >= 400"}
  ],
  "collect_vars": ["request", "response", "data", "user_id", "status_code"],
  "commands": ["c"],
  "hypothesis": "Error occurs under specific conditions with certain request data",
  "strategy": "Only break when error conditions occur, collect request context"
}
```

Run server with automated debugger, wait for error to occur naturally:

```bash
$ python scripts/automated_debugger.py api_server.py --plan error_capture.json &
# Server runs, collecting data only when error occurs
```

When 500 error finally happens, results show the exact state that caused it.

## Working with Debug Plans

### Creating Plans Manually

```json
{
  "breakpoints": [
    {
      "file": "myapp.py",
      "line": 42,
      "condition": "user_id == 'debug_user'"  // Optional
    },
    {
      "file": "utils.py", 
      "line": 15
    }
  ],
  "collect_vars": [
    "request_data",
    "config",
    "result"
  ],
  "commands": ["c"],  // 'c' = continue to next breakpoint
  "hypothesis": "What you think is wrong",
  "strategy": "How you'll test the hypothesis"
}
```

### Quick Command-Line Debugging

Without creating a plan file:

```bash
# Set breakpoints at specific lines
python scripts/automated_debugger.py script.py -b 42 -b 56

# Collect specific variables
python scripts/automated_debugger.py script.py -b 42 -v data -v result

# Combined
python scripts/automated_debugger.py script.py -b 42 -b 56 -v data -v result -o my_results.json
```

## Advanced Techniques

### Conditional Breakpoints for Rare Conditions

```json
{
  "breakpoints": [
    {
      "file": "process.py",
      "line": 30,
      "condition": "len(items) > 1000 and error_count > 5"
    }
  ]
}
```

Only breaks when both conditions are true → catches rare edge cases.

### Multi-Stage Debugging

**Stage 1**: Broad exploration
```json
{
  "breakpoints": [
    {"file": "main.py", "line": 10},
    {"file": "main.py", "line": 50},
    {"file": "main.py", "line": 100}
  ],
  "collect_vars": [],  // Collect all locals
  "hypothesis": "Initial exploration - where does execution go?"
}
```

**Stage 2**: Focused investigation (based on Stage 1 findings)
```json
{
  "breakpoints": [
    {"file": "main.py", "line": 47},  // Just before interesting area
    {"file": "main.py", "line": 48},
    {"file": "main.py", "line": 49}
  ],
  "collect_vars": ["specific_var", "another_var"],
  "hypothesis": "Narrow down to exact line where bug occurs"
}
```

**Stage 3**: Root cause analysis
```json
{
  "breakpoints": [
    {"file": "utils.py", "line": 23}  // Function that produces bad value
  ],
  "collect_vars": ["input_param", "intermediate", "output"],
  "hypothesis": "Find where incorrect value originates"
}
```

### Tracking Variable Evolution

To see how a variable changes through execution:

```bash
python scripts/debug_assistant.py analyze results.json --var my_variable
```

Shows:
```
Variable tracking: my_variable
--------------------------------------------------
module.py:15: my_variable = []
module.py:23: my_variable = [1, 2, 3]
module.py:45: my_variable = [1, 2, 3, 4]
module.py:67: my_variable = None  # Aha! Lost the data here
```

### Finding the Exact Problem Line

When you know the general area but not the exact line:

```json
{
  "breakpoints": [
    {"file": "module.py", "line": 100},
    {"file": "module.py", "line": 101},
    {"file": "module.py", "line": 102},
    {"file": "module.py", "line": 103},
    {"file": "module.py", "line": 104}
  ],
  "collect_vars": ["critical_var"]
}
```

Results show exactly where `critical_var` becomes incorrect.

## Claude's Debugging Strategies

### Strategy 1: Error-Location Analysis

```python
# From traceback:
#   File "app.py", line 142, in process_payment
#     amount = Decimal(raw_amount)
#   ValueError: Invalid literal for Decimal

# Claude's hypothesis:
"raw_amount contains invalid data for Decimal conversion"

# Debug plan:
{
  "breakpoints": [
    {"file": "app.py", "line": 140}  # Before the error
  ],
  "collect_vars": ["raw_amount", "request_data"],
  "hypothesis": "raw_amount has unexpected format or type"
}
```

### Strategy 2: Function-Entry Inspection

```python
# User: "calculate_discount returns wrong values"

# Claude's approach:
{
  "breakpoints": [
    {"file": "pricing.py", "line": 56}  # First line of calculate_discount
  ],
  "collect_vars": ["price", "discount_rate", "user_tier"],
  "hypothesis": "Inputs to function may be incorrect, or calculation logic wrong"
}
```

### Strategy 3: State-Evolution Tracking

```python
# User: "User session gets corrupted somewhere"

# Claude's multi-point plan:
{
  "breakpoints": [
    {"file": "session.py", "line": 20, "comment": "Session creation"},
    {"file": "session.py", "line": 45, "comment": "Session update"},
    {"file": "session.py", "line": 78, "comment": "Session save"}
  ],
  "collect_vars": ["session_data", "user_id"],
  "hypothesis": "Session corruption occurs during one of these operations"
}
```

### Strategy 4: Conditional Edge-Case Hunting

```python
# User: "Sometimes returns NaN, can't reproduce"

# Claude's targeted plan:
{
  "breakpoints": [
    {
      "file": "math_utils.py",
      "line": 34,
      "condition": "result != result"  # NaN check (NaN != NaN is True)
    }
  ],
  "collect_vars": ["all"],  # Collect everything when condition hits
  "hypothesis": "Catch the exact state when NaN is produced"
}
```

## Tips for Effective Automated Debugging

### 1. Start Broad, Then Focus

First run: Few breakpoints to understand flow
Second run: More breakpoints in the suspicious area
Third run: Pinpoint the exact problem

### 2. Use Conditional Breakpoints Wisely

Only break when the problem actually occurs → saves time analyzing irrelevant data.

### 3. Collect Relevant Variables

Too few: Might miss the key information
Too many: Results become overwhelming
Sweet spot: 3-7 most relevant variables

### 4. Iterate Quickly

Each debug run should test a specific hypothesis. If hypothesis is wrong, adjust and re-run immediately.

### 5. Document Hypotheses

Include "hypothesis" and "strategy" in plans → helps track what you've already tried.

## Comparing Approaches

### Interactive (pdb_runner.py)
**Best for:**
- Initial exploration
- Learning code flow
- Quick investigations
- One-off debugging

**Process:**
- Start debugger
- Manually step and inspect
- Real-time decisions

### Automated (automated_debugger.py)
**Best for:**
- Systematic data collection
- Intermittent bugs
- Testing specific hypotheses
- Multiple debug runs

**Process:**
- Create plan
- Run automatically
- Analyze results
- Refine plan

### When to Use Each

**Use Interactive when:**
- Don't know where to start
- Need to explore interactively
- Quick check of a theory
- Learning unfamiliar code

**Use Automated when:**
- Know general problem area
- Need to collect data from multiple points
- Debugging intermittent issues
- Want to test specific hypothesis systematically

**Use Both:**
1. Interactive exploration to form hypothesis
2. Automated data collection to test hypothesis
3. Interactive investigation of findings
4. Automated verification of fix

## Integration with Development Workflow

### In Development

```bash
# Quick check during development
python scripts/pdb_runner.py new_feature.py

# Automated testing of edge cases
python scripts/automated_debugger.py new_feature.py --plan edge_cases.json
```

### In Testing

```bash
# When test fails
python scripts/debug_assistant.py plan-from-traceback test_app.py test_failure.txt
python scripts/automated_debugger.py test_app.py --plan debug_plan.json
```

### In Production (Debugging)

```bash
# Create plan based on production error logs
# Run in staging with exact production data
python scripts/automated_debugger.py app.py --plan prod_error.json
```

## Limitations and Considerations

### What Automated Debugging Can't Do

- **Fix the bug** (it only collects data)
- **Understand business logic** (Claude provides this)
- **Handle infinite loops** (will hang like regular execution)
- **Debug GUI interactions** (console apps only)

### Performance Impact

- Minimal: Breakpoints only add microseconds
- Each breakpoint hit collects data → some overhead
- Conditional breakpoints evaluated on every line → use sparingly

### Data Collection Limits

- JSON serialization limits (can't capture complex objects fully)
- Very large data structures summarized
- Some object types shown as `<TypeName instance>`

## Next Steps

1. **Try the examples** in this guide
2. **Practice hypothesis formation** - what would you investigate first?
3. **Create your own debug plans** for your projects
4. **Combine with interactive debugging** for maximum effectiveness
5. **Share findings** - debug results can document complex bugs

## Appendix: Debug Plan Template

```json
{
  "breakpoints": [
    {
      "file": "path/to/file.py",
      "line": 42,
      "condition": "optional_condition"  // Remove if not needed
    }
  ],
  "collect_vars": [
    "var1",
    "var2",
    "var3"
  ],
  "commands": ["c"],  // Usually just "c" for continue
  "hypothesis": "Clear statement of what you think is wrong",
  "strategy": "Explanation of how this plan tests the hypothesis",
  "notes": "Any additional context or observations"
}
```
