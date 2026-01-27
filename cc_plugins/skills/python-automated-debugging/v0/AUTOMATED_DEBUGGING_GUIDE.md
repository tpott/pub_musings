# Automated Debugging Examples for Claude

This document shows how Claude should use the automated debugging capabilities to investigate bugs systematically.

## Example 1: Division by Zero Bug

### User Request
"This script crashes when calculating averages. Can you debug it?"

### Code (average.py)
```python
def calculate_average(numbers):
    total = sum(numbers)
    count = len(numbers)
    return total / count

data = [[10, 20, 30], [5, 15, 25], []]
for dataset in data:
    print(f"Average: {calculate_average(dataset)}")
```

### Claude's Debugging Process

**Step 1: Form Initial Hypothesis**
- The code likely crashes on the empty list (division by zero)
- Need to confirm this is the issue and locate exactly where

**Step 2: First Debugging Session - Locate the Crash**
```bash
python scripts/debug_session.py average.py "c" "w" "q" --output debug1.log
```

**Output Analysis (debug1.log)**:
```
(Pdb) c
Average: 20.0
Average: 15.0
ZeroDivisionError: division by zero
> /path/to/average.py(4)calculate_average()
-> return total / count
(Pdb) w
...
```

**Finding**: Confirmed - crashes on line 4 when count is 0

**Step 3: Second Session - Inspect the State**
```bash
python scripts/debug_session.py average.py "b calculate_average" "c" "c" "c" "p numbers" "p count" "q" --output debug2.log
```

**Output Analysis (debug2.log)**:
```
(Pdb) b calculate_average
Breakpoint 1 at /path/to/average.py:1
(Pdb) c
Average: 20.0
> /path/to/average.py(2)calculate_average()
(Pdb) c
Average: 15.0
> /path/to/average.py(2)calculate_average()
(Pdb) c
> /path/to/average.py(2)calculate_average()
(Pdb) p numbers
[]
(Pdb) p count
0
(Pdb) q
```

**Finding**: The third call receives an empty list, resulting in count=0

**Claude's Conclusion**:
The bug is confirmed. The function doesn't handle empty lists. Fix: Add a check for empty lists before division.

```python
def calculate_average(numbers):
    if not numbers:  # Handle empty list
        return 0  # or raise ValueError("Cannot average empty list")
    total = sum(numbers)
    count = len(numbers)
    return total / count
```

## Example 2: Wrong Calculation Result

### User Request
"My factorial function returns wrong results for some inputs."

### Code (factorial.py)
```python
def factorial(n):
    if n < 0:
        raise ValueError("Negative factorial undefined")
    result = n
    for i in range(n-1, 1, -1):
        result *= i
    return result

print(f"5! = {factorial(5)}")  # Expected: 120
print(f"0! = {factorial(0)}")  # Expected: 1, but gets 0
print(f"1! = {factorial(1)}")  # Expected: 1
```

### Claude's Debugging Process

**Step 1: Hypothesis**
- factorial(5) probably works (simple case)
- factorial(0) and factorial(1) likely have issues (edge cases)
- Need to trace execution for edge cases

**Step 2: Test with Multiple Inputs**
```bash
python scripts/debug_session.py factorial.py "b factorial" "c" "p n" "c" "p n" "c" "p n" "q" --output debug1.log
```

**Output Analysis**:
```
(Pdb) b factorial
Breakpoint 1 at /path/to/factorial.py:1
(Pdb) c
> /path/to/factorial.py(2)factorial()
(Pdb) p n
5
(Pdb) c
5! = 120
> /path/to/factorial.py(2)factorial()
(Pdb) p n
0
(Pdb) c
0! = 0
> /path/to/factorial.py(2)factorial()
(Pdb) p n
1
(Pdb) q
```

**Finding**: n=0 returns 0 (wrong), need to investigate why

**Step 3: Step Through factorial(0)**
```bash
python scripts/debug_session.py factorial.py "b factorial" "c" "c" "a" "l" "n" "n" "p result" "n" "p result" "q" --output debug2.log
```

**Output Analysis**:
```
(Pdb) b factorial
Breakpoint 1 at /path/to/factorial.py:1
(Pdb) c
5! = 120
> /path/to/factorial.py(2)factorial()
(Pdb) c
> /path/to/factorial.py(2)factorial()
(Pdb) a
n = 0
(Pdb) l
  1     def factorial(n):
  2         if n < 0:
  3             raise ValueError("Negative factorial undefined")
  4  ->     result = n
  5         for i in range(n-1, 1, -1):
  6             result *= i
  7         return result
(Pdb) n
> /path/to/factorial.py(5)factorial()
(Pdb) n
> /path/to/factorial.py(7)factorial()
(Pdb) p result
0
(Pdb) n
0! = 0
> /path/to/factorial.py(2)factorial()
(Pdb) p result
0
(Pdb) q
```

**Finding**: 
- Line 4: `result = n` sets result to 0
- Line 5-6: The loop doesn't execute (range(0-1, 1, -1) = range(-1, 1, -1) is empty)
- Returns 0 instead of 1

**Step 4: Verify Loop Range**
```bash
python scripts/debug_session.py factorial.py "b factorial" "c" "c" "p n" "p list(range(n-1, 1, -1))" "q" --output debug3.log
```

**Output Analysis**:
```
(Pdb) p n
0
(Pdb) p list(range(n-1, 1, -1))
[]
(Pdb) q
```

**Claude's Conclusion**:
The bug is in the initialization. When n=0, the loop doesn't execute and result stays 0. Fix: Handle n=0 and n=1 as special cases.

```python
def factorial(n):
    if n < 0:
        raise ValueError("Negative factorial undefined")
    if n == 0 or n == 1:  # Special case
        return 1
    result = n
    for i in range(n-1, 1, -1):
        result *= i
    return result
```

## Example 3: Web Server Request Handler Bug

### User Request
"My Flask API returns wrong user data. It seems to return the same user for different IDs."

### Code (server.py)
```python
from flask import Flask, jsonify

app = Flask(__name__)

users = {
    "1": {"name": "Alice", "email": "alice@example.com"},
    "2": {"name": "Bob", "email": "bob@example.com"},
}

current_user = None

@app.route('/users/<user_id>')
def get_user(user_id):
    global current_user
    current_user = users.get("1")  # Bug: hardcoded "1"
    return jsonify(current_user) if current_user else ("Not found", 404)

if __name__ == '__main__':
    app.run(debug=True, port=5000)
```

### Claude's Debugging Process

**Step 1: Hypothesis**
- The function should use `user_id` parameter but might not be
- Need to check what value is actually being used

**Step 2: Inspect Function Execution**
```bash
python scripts/debug_session.py server.py "b get_user" "c" "a" "p user_id" "l" "n" "p current_user" "q" --output debug1.log
```

Note: For server debugging, Claude would note that the server needs to be started and a request made. Alternative approach:

**Step 3: Test the Logic Directly**
Create a test script to isolate the function:

```python
# test_get_user.py
users = {
    "1": {"name": "Alice", "email": "alice@example.com"},
    "2": {"name": "Bob", "email": "bob@example.com"},
}

def get_user(user_id):
    global current_user
    current_user = users.get("1")
    return current_user

print("Testing user 1:", get_user("1"))
print("Testing user 2:", get_user("2"))
```

```bash
python scripts/debug_session.py test_get_user.py "b get_user" "c" "p user_id" "n" "p current_user" "c" "p user_id" "n" "p current_user" "q" --output debug1.log
```

**Output Analysis**:
```
(Pdb) b get_user
Breakpoint 1 at /path/to/test_get_user.py:7
(Pdb) c
> /path/to/test_get_user.py(9)get_user()
(Pdb) p user_id
'1'
(Pdb) n
> /path/to/test_get_user.py(10)get_user()
(Pdb) p current_user
{'name': 'Alice', 'email': 'alice@example.com'}
(Pdb) c
Testing user 1: {'name': 'Alice', 'email': 'alice@example.com'}
> /path/to/test_get_user.py(9)get_user()
(Pdb) p user_id
'2'
(Pdb) n
> /path/to/test_get_user.py(10)get_user()
(Pdb) p current_user
{'name': 'Alice', 'email': 'alice@example.com'}
(Pdb) q
```

**Finding**: 
- First call: user_id='1', returns Alice ✓
- Second call: user_id='2', but still returns Alice ✗
- The issue is on line 9: `users.get("1")` is hardcoded!

**Step 4: Verify the Source Code**
```bash
python scripts/debug_session.py test_get_user.py "b get_user" "c" "l" "q" --output debug2.log
```

**Output Analysis**:
```
(Pdb) l
  7     def get_user(user_id):
  8         global current_user
  9  ->     current_user = users.get("1")  # Should be user_id!
 10         return current_user
```

**Claude's Conclusion**:
The bug is clear: `users.get("1")` should be `users.get(user_id)`.

## Example 4: Iterative Hypothesis Refinement

### User Request
"This data processing script gives inconsistent results."

### Code (process.py)
```python
def process_data(items):
    results = []
    total = 0
    for item in items:
        total += item['value']
        avg = total / len(items)  # Bug: should be len(results) + 1
        results.append({
            'item': item['name'],
            'running_avg': avg
        })
    return results

data = [
    {'name': 'A', 'value': 10},
    {'name': 'B', 'value': 20},
    {'name': 'C', 'value': 30}
]

print(process_data(data))
```

### Claude's Debugging Process

**Initial Hypothesis**: "Inconsistent results" suggests the calculation changes unexpectedly

**Session 1: See the Output**
```bash
python scripts/debug_session.py process.py "c" "q" --output debug1.log
```

**Analysis**: Output shows running averages, but need to verify they're correct

**Session 2: Trace the Calculation**
```bash
python scripts/debug_session.py process.py "b process_data" "c" "n" "n" "n" "p item" "p total" "p avg" "n" "p results" "q" --output debug2.log
```

**Analysis**: 
- After 1st iteration: total=10, avg=10/3=3.33 (should be 10/1=10)
- Hypothesis updated: Division by wrong value - using total length instead of current position

**Session 3: Verify Loop Iterations**
```bash
python scripts/debug_session.py process.py "b process_data" "c" "n" "n" "n" "n" "p len(items)" "p len(results)" "n" "p total" "p avg" "n" "n" "n" "p len(results)" "p total" "p avg" "q" --output debug3.log
```

**Analysis**:
- Iteration 1: len(items)=3, len(results)=0, total=10, avg=10/3 ✗ (should be 10/1)
- Iteration 2: len(items)=3, len(results)=1, total=30, avg=30/3 ✗ (should be 30/2)
- Confirmed: Using `len(items)` (constant 3) instead of current count

**Claude's Conclusion**:
Bug confirmed. Line 6 should use the current number of processed items, not total items.

Fix:
```python
avg = total / (len(results) + 1)  # Current position in the loop
```

## Key Principles for Claude's Automated Debugging

1. **Start Broad, Then Narrow**
   - First session: Locate the problem ("c" "w" "q")
   - Subsequent sessions: Zoom into specific functions/lines

2. **One Hypothesis Per Session**
   - Design each session to test a specific hypothesis
   - Don't try to investigate everything at once

3. **Collect Concrete Data**
   - Always use "p" to print actual values
   - Compare actual vs expected explicitly

4. **Iterate Freely**
   - Multiple short sessions > one complex session
   - Each session should clarify or refine understanding

5. **Document Findings**
   - Save session output (--output flag)
   - Note what each session revealed
   - Build understanding incrementally

6. **Verify Before Concluding**
   - After forming a hypothesis, run one more session to confirm
   - Check edge cases that might reveal alternative explanations

## Command Patterns for Different Scenarios

### Finding Where Code Crashes
```bash
"c" "w" "l" "q"
```

### Inspecting Function Arguments
```bash
"b function_name" "c" "a" "l" "q"
```

### Tracing Variable Values
```bash
"b start_point" "c" "p var" "n" "p var" "n" "p var" "q"
```

### Understanding Control Flow
```bash
"b decision_point" "c" "p condition" "l" "n" "w" "q"
```

### Examining Data Structures
```bash
"b function" "c" "p type(obj)" "p len(obj)" "pp obj" "q"
```

### Verifying Loop Behavior
```bash
"b loop_start" "c" "p loop_var" "n" "n" "p loop_var" "n" "n" "p loop_var" "q"
```
