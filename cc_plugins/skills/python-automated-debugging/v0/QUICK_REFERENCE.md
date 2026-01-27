# pdb Quick Reference

A concise reference for Python debugger (pdb) commands.

## Starting the Debugger

```bash
# With PTY support (recommended)
python scripts/pdb_runner.py script.py [args]
python scripts/pdb_runner.py -m module [args]

# Automatic post-mortem on crash
python scripts/auto_postmortem.py script.py [args]

# Programmatically in code
import pdb; pdb.set_trace()

# At a specific condition
if error_condition:
    import pdb; pdb.set_trace()
```

## Navigation Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `next` | `n` | Execute current line, step over functions |
| `step` | `s` | Execute current line, step into functions |
| `continue` | `c` | Continue execution until next breakpoint |
| `return` | `r` | Continue until current function returns |
| `until [line]` | `unt` | Continue until line number (or greater) |
| `jump [line]` | `j` | Set next line to execute (dangerous!) |

## Inspection Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `print <expr>` | `p` | Evaluate and print expression |
| `pp <expr>` | `pp` | Pretty-print expression |
| `args` | `a` | Print arguments of current function |
| `whatis <expr>` | | Print type of expression |
| `display <expr>` | | Auto-display expression on each step |
| `undisplay [num]` | | Stop auto-displaying expression |

## Code Display Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `list [first[,last]]` | `l` | List source code around current line |
| `longlist` | `ll` | List entire current function |
| `source <obj>` | | Show source code for object |
| `where` | `w`, `bt` | Print stack trace |
| `up` | `u` | Move up one stack frame |
| `down` | `d` | Move down one stack frame |

## Breakpoint Commands

| Command | Description | Example |
|---------|-------------|---------|
| `break` | List all breakpoints | `b` |
| `break <line>` | Set breakpoint at line in current file | `b 42` |
| `break <file>:<line>` | Set breakpoint at line in specific file | `b app.py:123` |
| `break <function>` | Set breakpoint at function | `b my_function` |
| `break <line>, <condition>` | Set conditional breakpoint | `b 42, x > 10` |
| `tbreak <args>` | Set temporary breakpoint (removed after first hit) | `tbreak 42` |
| `clear [bpnum]` | Clear breakpoint(s) | `cl 1` |
| `disable [bpnum]` | Disable breakpoint(s) | `disable 1` |
| `enable [bpnum]` | Enable breakpoint(s) | `enable 1` |
| `ignore <bpnum> <count>` | Ignore breakpoint for count times | `ignore 1 5` |
| `condition <bpnum> <expr>` | Add condition to existing breakpoint | `condition 1 x > 10` |
| `commands [bpnum]` | Set commands to execute at breakpoint | See below |

### Commands at Breakpoints

Execute specific pdb commands automatically when a breakpoint is hit:

```
(Pdb) commands 1
(com) print(f"x = {x}")
(com) print(f"y = {y}")
(com) continue
(com) end
```

## Execution Control

| Command | Alias | Description |
|---------|-------|-------------|
| `quit` | `q` | Quit debugger (terminates program) |
| `exit` | | Same as quit |
| `restart` | `run` | Restart program |
| `interact` | | Start interactive interpreter |

## Evaluation Commands

| Command | Description |
|---------|-------------|
| `!<statement>` | Execute Python statement in current context |
| `alias [name [command]]` | Create alias for pdb command |
| `unalias <name>` | Remove alias |

## Useful Patterns

### Print All Local Variables
```python
(Pdb) p locals()
# Or pretty-print
(Pdb) pp locals()
```

### Print Object Attributes
```python
(Pdb) pp vars(obj)
# Or
(Pdb) pp obj.__dict__
```

### Conditional Execution
```python
(Pdb) !if x > 10: print("High value")
```

### Quick Stack Navigation
```python
(Pdb) w          # See where you are
(Pdb) u          # Go up one frame
(Pdb) p var      # Check variable in parent scope
(Pdb) d          # Go back down
```

### List Comprehension in Debugger
```python
(Pdb) p [item for item in data if item > 5]
```

### Debug Specific Iteration
```python
# In your code:
for i, item in enumerate(data):
    if i == 42:
        import pdb; pdb.set_trace()
    process(item)
```

### Set Multiple Breakpoints
```python
(Pdb) b function1
(Pdb) b module.py:123
(Pdb) b 456, x > 100
(Pdb) b  # List all breakpoints
```

## Common Workflows

### Basic Debugging Session
```
1. Start debugger: python scripts/pdb_runner.py script.py
2. Set breakpoint: (Pdb) b problematic_function
3. Run to breakpoint: (Pdb) c
4. Inspect state: (Pdb) p variable
5. Step through: (Pdb) n
6. Continue or quit: (Pdb) c or (Pdb) q
```

### Finding Where Exception Occurs
```
1. Run with auto_postmortem.py
2. Program crashes and enters debugger
3. Check stack: (Pdb) w
4. Move to relevant frame: (Pdb) u or (Pdb) d
5. Inspect state: (Pdb) p locals()
```

### Debugging Web Request
```
1. Add breakpoint in view/handler: import pdb; pdb.set_trace()
2. Start server: python scripts/pdb_runner.py server.py
3. Continue past startup: (Pdb) c
4. Make HTTP request in browser/curl
5. Debugger breaks in your handler
6. Inspect request: (Pdb) p request.method, request.headers
7. Step through handler: (Pdb) n
8. Continue to handle next request: (Pdb) c
```

## Tips and Tricks

### Avoid Typing Full Variable Names
```python
# Create short aliases for long expressions
(Pdb) !config = app.config
(Pdb) p config['DEBUG']
```

### Re-run Last Command
Press Enter (empty command) to repeat the last navigation command (`n`, `s`, `c`, etc.)

### Sticky Mode (Python 3.7+)
```python
(Pdb) sticky
# Shows source code persistently as you step
```

### Track Variable Changes
```python
(Pdb) display x
(Pdb) n  # x is shown automatically each step
```

### Catch Exceptions
```python
# In your code
try:
    risky_operation()
except Exception as e:
    import pdb; pdb.set_trace()
    raise
```

### Interactive Debugging
```python
(Pdb) interact
# Drops into full Python REPL with local context
# Exit with Ctrl+D to return to pdb
```

## Environment-Specific Notes

### Django
- Use `python scripts/pdb_runner.py manage.py runserver --noreload` to avoid restart issues
- Breakpoints in views trigger on HTTP requests

### Flask
- Set `FLASK_DEBUG=1` for better error messages
- Use `python scripts/pdb_runner.py -m flask run --no-reload`

### Async Code
- pdb works with async/await
- Breakpoints in async functions work normally
- Be aware of event loop implications

### Tests
- Use `python scripts/pdb_runner.py -m pytest --pdb`
- Or use post-mortem: `python scripts/auto_postmortem.py -m pytest`

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Enter | Repeat last command |
| Ctrl+C | Interrupt (back to pdb prompt) |
| Ctrl+D | Exit debugger |
| Up/Down | Navigate command history |

## Common Pitfalls

1. **Forgetting to continue**: Set breakpoint and immediately use `q` instead of `c`
2. **Wrong frame**: Inspecting variables in wrong stack frame - use `w`, `u`, `d`
3. **Variable name collision**: If variable name matches pdb command, use `!` prefix: `!c = 5`
4. **Breaking too late**: Breakpoint after the problem - set earlier in code flow

## Getting Help

```python
(Pdb) h          # List all commands
(Pdb) h command  # Help for specific command
(Pdb) h b        # Help for break command
```

## Online Resources

- Official docs: https://docs.python.org/3/library/pdb.html
- PDB tutorial: https://realpython.com/python-debugging-pdb/
- PDB cheatsheet: https://kapeli.com/cheat_sheets/Python_Debugger.docset/Contents/Resources/Documents/index
