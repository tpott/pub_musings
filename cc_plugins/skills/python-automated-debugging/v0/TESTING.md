# Testing the pdb Debugger Skill

This guide shows how to test and verify the pdb debugger skill is working correctly.

## Quick Test

Run the included demo script:

```bash
cd pdb-debugger
python scripts/pdb_runner.py demo.py
```

Expected behavior:
1. The debugger starts and shows the first line of code
2. You can use pdb commands to navigate and inspect
3. The demo script has a bug (0! returns 0 instead of 1)

### Test Session Example

```
$ python scripts/pdb_runner.py demo.py
> /path/to/demo.py(1)<module>()
-> #!/usr/bin/env python3
(Pdb) b calculate_factorial
Breakpoint 1 at /path/to/demo.py:8
(Pdb) c
Factorial Calculator Demo
========================================

Calculating factorials for: [5, 3, 0, -1, 7]

Processing: 5
> /path/to/demo.py(9)calculate_factorial()
-> """Calculate factorial of n."""
(Pdb) n
> /path/to/demo.py(10)calculate_factorial()
-> if n < 0:
(Pdb) p n
5
(Pdb) c
Processing: 3
> /path/to/demo.py(9)calculate_factorial()
-> """Calculate factorial of n."""
(Pdb) c
Processing: 0
> /path/to/demo.py(9)calculate_factorial()
-> """Calculate factorial of n."""
(Pdb) p n
0
(Pdb) c
Processing: -1
> /path/to/demo.py(9)calculate_factorial()
-> """Calculate factorial of n."""
(Pdb) c
  Error: Factorial is not defined for negative numbers
Processing: 7
> /path/to/demo.py(9)calculate_factorial()
-> """Calculate factorial of n."""
(Pdb) q
```

## Testing Different Scenarios

### Test 1: Basic Stepping

```bash
python scripts/pdb_runner.py demo.py
```

At the pdb prompt:
```
(Pdb) b main
(Pdb) c
(Pdb) s  # Step into process_numbers
(Pdb) l  # List source code
(Pdb) n  # Step through a few lines
(Pdb) q
```

### Test 2: Breakpoint and Variable Inspection

```bash
python scripts/pdb_runner.py demo.py
```

At the pdb prompt:
```
(Pdb) b calculate_factorial
(Pdb) c
(Pdb) p n
(Pdb) p result
(Pdb) l
(Pdb) q
```

### Test 3: Post-Mortem Debugging

First, modify demo.py to add a crash:

```python
# In main(), add this line:
result = 10 / 0  # This will crash
```

Then run:
```bash
python scripts/auto_postmortem.py demo.py
```

The debugger should automatically start when the division by zero occurs.

### Test 4: Conditional Breakpoint

```bash
python scripts/pdb_runner.py demo.py
```

At the pdb prompt:
```
(Pdb) b calculate_factorial, n == 0
(Pdb) c
# Should break only when n=0
(Pdb) p n
0
(Pdb) q
```

## Testing with Real Applications

### Test with a Simple Web Server

Create a test Flask app (`test_server.py`):

```python
from flask import Flask, jsonify

app = Flask(__name__)

@app.route('/')
def home():
    import pdb; pdb.set_trace()
    return jsonify({"message": "Hello from debugger!"})

if __name__ == '__main__':
    app.run(debug=True, port=5000)
```

Run it:
```bash
python scripts/pdb_runner.py test_server.py
(Pdb) c
# Server starts
# In another terminal: curl http://localhost:5000
# Debugger breaks in home()
```

### Test with a CLI Tool

Create a test CLI (`test_cli.py`):

```python
import argparse

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--name', default='World')
    args = parser.parse_args()
    
    import pdb; pdb.set_trace()
    
    print(f"Hello, {args.name}!")

if __name__ == '__main__':
    main()
```

Run it:
```bash
python scripts/pdb_runner.py test_cli.py --name Claude
(Pdb) p args
(Pdb) p args.name
(Pdb) c
```

## Verifying PTY Functionality

The key feature of this skill is proper PTY handling. To verify it works:

1. **Without PTY** (should fail or be very limited):
```bash
python -m pdb demo.py < /dev/null
# This typically fails or has limited functionality
```

2. **With PTY** (should work perfectly):
```bash
python scripts/pdb_runner.py demo.py
# This should work smoothly with full interactivity
```

## Expected Outcomes

✅ **Success indicators:**
- Debugger starts without errors
- Can set breakpoints with `b` command
- Can step through code with `n` and `s`
- Can print variables with `p`
- Can see source code with `l`
- Can continue execution with `c`
- Can quit with `q`

❌ **Failure indicators:**
- "Not a terminal" errors
- Inability to input commands
- Commands not responding
- Garbled output

## Common Testing Issues

### Issue: "No module named pdb"
**Solution:** This shouldn't happen with Python 3.x, but if it does, your Python installation is broken.

### Issue: Permission denied when running scripts
**Solution:** Make scripts executable:
```bash
chmod +x scripts/*.py
```

### Issue: Can't find demo.py
**Solution:** Make sure you're in the pdb-debugger directory:
```bash
cd pdb-debugger
python scripts/pdb_runner.py demo.py
```

### Issue: Script runs but doesn't break
**Solution:** Remember to use `c` (continue) after setting breakpoints:
```
(Pdb) b main
(Pdb) c  # This is required to run to the breakpoint
```

## Automated Testing

While pdb is interactive, you can create automated tests to verify basic functionality:

```python
# test_pdb_runner.py
import subprocess
import sys

def test_pdb_runner_starts():
    """Test that pdb_runner can start the demo script."""
    proc = subprocess.Popen(
        [sys.executable, 'scripts/pdb_runner.py', 'demo.py'],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    
    # Send 'q' to quit immediately
    stdout, stderr = proc.communicate(input='q\n', timeout=5)
    
    # Check that pdb prompt appeared
    assert '(Pdb)' in stdout or '(Pdb)' in stderr
    print("✓ pdb_runner can start debugger")

def test_auto_postmortem():
    """Test that auto_postmortem can handle crashes."""
    # Create a script that will crash
    crash_script = 'test_crash.py'
    with open(crash_script, 'w') as f:
        f.write('raise ValueError("Test crash")\n')
    
    proc = subprocess.Popen(
        [sys.executable, 'scripts/auto_postmortem.py', crash_script],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    
    stdout, stderr = proc.communicate(input='q\n', timeout=5)
    combined = stdout + stderr
    
    assert 'EXCEPTION OCCURRED' in combined
    assert 'post-mortem debugger' in combined
    print("✓ auto_postmortem handles crashes")

if __name__ == '__main__':
    test_pdb_runner_starts()
    test_auto_postmortem()
    print("\nAll tests passed!")
```

Run automated tests:
```bash
python test_pdb_runner.py
```

## Next Steps

After verifying the skill works:

1. Try it with your own Python projects
2. Practice the common workflows in EXAMPLES.md
3. Reference QUICK_REFERENCE.md for command reminders
4. Explore advanced features like conditional breakpoints
5. Integrate pdb into your regular debugging workflow
