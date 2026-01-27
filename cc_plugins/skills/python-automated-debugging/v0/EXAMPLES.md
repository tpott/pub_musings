# pdb Debugger Examples

This document provides complete, working examples of debugging various types of Python applications using the pdb skill.

## Example 1: Debugging a Simple Script

### The Script (calculate.py)

```python
def calculate_average(numbers):
    """Calculate average of a list of numbers."""
    total = sum(numbers)
    count = len(numbers)
    average = total / count
    return average

def main():
    data = [10, 20, 30, 40, 50]
    result = calculate_average(data)
    print(f"Average: {result}")
    
    # This will cause a division by zero error
    empty_data = []
    result2 = calculate_average(empty_data)
    print(f"Average of empty: {result2}")

if __name__ == "__main__":
    main()
```

### Debugging Session

```bash
$ python scripts/pdb_runner.py calculate.py
> /path/to/calculate.py(1)<module>()
-> def calculate_average(numbers):
(Pdb) b calculate_average
Breakpoint 1 at /path/to/calculate.py:1
(Pdb) c
> /path/to/calculate.py(2)calculate_average()
-> """Calculate average of a list of numbers."""
(Pdb) n
> /path/to/calculate.py(3)calculate_average()
-> total = sum(numbers)
(Pdb) p numbers
[10, 20, 30, 40, 50]
(Pdb) n
> /path/to/calculate.py(4)calculate_average()
-> count = len(numbers)
(Pdb) p total
150
(Pdb) c
Average: 30.0
> /path/to/calculate.py(2)calculate_average()
-> """Calculate average of a list of numbers."""
(Pdb) p numbers
[]
(Pdb) # We can see the empty list that will cause the error
(Pdb) c
ZeroDivisionError: division by zero
```

## Example 2: Debugging a Flask Web Server

### The Server (app.py)

```python
from flask import Flask, request, jsonify

app = Flask(__name__)

users = {
    "1": {"name": "Alice", "email": "alice@example.com"},
    "2": {"name": "Bob", "email": "bob@example.com"}
}

@app.route('/users/<user_id>')
def get_user(user_id):
    # Set breakpoint here to inspect requests
    import pdb; pdb.set_trace()
    
    user = users.get(user_id)
    if user:
        return jsonify(user)
    else:
        return jsonify({"error": "User not found"}), 404

@app.route('/users', methods=['POST'])
def create_user():
    data = request.get_json()
    
    # Debug point to inspect incoming data
    import pdb; pdb.set_trace()
    
    user_id = str(len(users) + 1)
    users[user_id] = {
        "name": data.get("name"),
        "email": data.get("email")
    }
    return jsonify({"id": user_id, **users[user_id]}), 201

if __name__ == '__main__':
    app.run(debug=True, port=5000)
```

### Debugging Session

```bash
$ python scripts/pdb_runner.py app.py
> /path/to/app.py(1)<module>()
-> from flask import Flask, request, jsonify
(Pdb) c
 * Serving Flask app 'app'
 * Debug mode: on
 * Running on http://127.0.0.1:5000

# In another terminal, make a request:
# curl http://localhost:5000/users/1

# Back in debugger:
> /path/to/app.py(13)get_user()
-> user = users.get(user_id)
(Pdb) p user_id
'1'
(Pdb) p users
{'1': {'name': 'Alice', 'email': 'alice@example.com'}, '2': {'name': 'Bob', 'email': 'bob@example.com'}}
(Pdb) n
> /path/to/app.py(14)get_user()
-> if user:
(Pdb) p user
{'name': 'Alice', 'email': 'alice@example.com'}
(Pdb) c

# Make POST request:
# curl -X POST -H "Content-Type: application/json" -d '{"name":"Charlie","email":"charlie@example.com"}' http://localhost:5000/users

# Debugger breaks at second breakpoint:
> /path/to/app.py(25)create_user()
-> user_id = str(len(users) + 1)
(Pdb) p data
{'name': 'Charlie', 'email': 'charlie@example.com'}
(Pdb) p request.method
'POST'
(Pdb) c
```

## Example 3: Debugging a CLI Tool

### The CLI Tool (data_processor.py)

```python
import argparse
import json
import sys

def load_data(filename):
    """Load JSON data from file."""
    with open(filename, 'r') as f:
        return json.load(f)

def process_data(data, operation):
    """Process data based on operation."""
    if operation == 'sum':
        # Bug: not handling nested structures
        return sum(data)
    elif operation == 'average':
        return sum(data) / len(data)
    elif operation == 'count':
        return len(data)
    else:
        raise ValueError(f"Unknown operation: {operation}")

def main():
    parser = argparse.ArgumentParser(description='Process data from JSON file')
    parser.add_argument('file', help='Input JSON file')
    parser.add_argument('--operation', '-o', 
                       choices=['sum', 'average', 'count'],
                       default='sum',
                       help='Operation to perform')
    parser.add_argument('--verbose', '-v', action='store_true',
                       help='Verbose output')
    
    args = parser.parse_args()
    
    if args.verbose:
        print(f"Loading data from {args.file}...")
    
    # Set breakpoint to inspect parsed arguments
    import pdb; pdb.set_trace()
    
    data = load_data(args.file)
    
    if args.verbose:
        print(f"Processing {len(data)} items...")
    
    result = process_data(data, args.operation)
    print(f"Result: {result}")

if __name__ == '__main__':
    main()
```

### Sample Data (data.json)

```json
[10, 20, 30, 40, 50]
```

### Debugging Session

```bash
$ python scripts/pdb_runner.py data_processor.py data.json --operation average --verbose
> /path/to/data_processor.py(1)<module>()
-> import argparse
(Pdb) c
Loading data from data.json...
> /path/to/data_processor.py(37)main()
-> data = load_data(args.file)
(Pdb) p args
Namespace(file='data.json', operation='average', verbose=True)
(Pdb) p args.operation
'average'
(Pdb) n
Processing 5 items...
> /path/to/data_processor.py(42)main()
-> result = process_data(data, args.operation)
(Pdb) p data
[10, 20, 30, 40, 50]
(Pdb) s
--Call--
> /path/to/data_processor.py(11)process_data()
-> def process_data(data, operation):
(Pdb) n
> /path/to/data_processor.py(13)process_data()
-> if operation == 'sum':
(Pdb) n
> /path/to/data_processor.py(16)process_data()
-> return sum(data) / len(data)
(Pdb) p sum(data)
150
(Pdb) p len(data)
5
(Pdb) c
Result: 30.0
```

## Example 4: Debugging Django Management Commands

### Running Django Management Command Under pdb

```bash
# Standard Django development server
$ python scripts/pdb_runner.py manage.py runserver

# Django management command with arguments
$ python scripts/pdb_runner.py manage.py migrate --fake-initial

# Custom management command
$ python scripts/pdb_runner.py manage.py process_users --batch-size 100
```

### Setting Breakpoints in Django Views

```python
# myapp/views.py
from django.http import JsonResponse
from django.views import View

class UserDetailView(View):
    def get(self, request, user_id):
        # Breakpoint to debug request handling
        import pdb; pdb.set_trace()
        
        # ... your view logic
        return JsonResponse({'user_id': user_id})
```

Then run:
```bash
$ python scripts/pdb_runner.py manage.py runserver
(Pdb) c
# Server starts, make request to trigger breakpoint
```

## Example 5: Debugging an Async Application

### Async Server (async_server.py)

```python
import asyncio
import aiohttp
from aiohttp import web

async def fetch_data(session, url):
    """Fetch data from URL."""
    # Breakpoint in async function
    import pdb; pdb.set_trace()
    
    async with session.get(url) as response:
        return await response.text()

async def handle_request(request):
    """Handle HTTP request."""
    url = request.query.get('url', 'http://example.com')
    
    async with aiohttp.ClientSession() as session:
        data = await fetch_data(session, url)
        return web.Response(text=f"Fetched {len(data)} bytes")

app = web.Application()
app.router.add_get('/', handle_request)

if __name__ == '__main__':
    web.run_app(app, port=8080)
```

### Debugging Session

```bash
$ python scripts/pdb_runner.py async_server.py
> /path/to/async_server.py(1)<module>()
-> import asyncio
(Pdb) c
======== Running on http://0.0.0.0:8080 ========

# Make request: curl "http://localhost:8080/?url=http://httpbin.org/json"

# Debugger breaks in async function:
> /path/to/async_server.py(11)fetch_data()
-> async with session.get(url) as response:
(Pdb) p url
'http://httpbin.org/json'
(Pdb) p session
<aiohttp.client.ClientSession object at 0x...>
(Pdb) c
```

## Example 6: Debugging Module Execution

Many Python tools are run as modules with `-m`:

```bash
# Debug HTTP server module
$ python scripts/pdb_runner.py -m http.server 8000

# Debug pytest test runner
$ python scripts/pdb_runner.py -m pytest tests/test_myapp.py -v

# Debug uvicorn ASGI server
$ python scripts/pdb_runner.py -m uvicorn main:app --reload --port 8000

# Debug custom package main
$ python scripts/pdb_runner.py -m mypackage.cli --config settings.json
```

## Example 7: Post-Mortem Debugging

### Script with Error Handler (crash_handler.py)

```python
import sys
import pdb

def divide_numbers(a, b):
    """Divide two numbers."""
    return a / b

def process_list(numbers):
    """Process a list of number pairs."""
    results = []
    for i, (a, b) in enumerate(numbers):
        print(f"Processing pair {i}: {a} / {b}")
        result = divide_numbers(a, b)
        results.append(result)
    return results

def main():
    pairs = [
        (10, 2),
        (15, 3),
        (20, 0),  # This will crash
        (25, 5)
    ]
    
    try:
        results = process_list(pairs)
        print(f"Results: {results}")
    except Exception as e:
        print(f"\nError occurred: {e}")
        print("Entering post-mortem debugger...")
        pdb.post_mortem(sys.exc_info()[2])

if __name__ == '__main__':
    main()
```

### Post-Mortem Session

```bash
$ python crash_handler.py
Processing pair 0: 10 / 2
Processing pair 1: 15 / 3
Processing pair 2: 20 / 0

Error occurred: division by zero
Entering post-mortem debugger...
> /path/to/crash_handler.py(7)divide_numbers()
-> return a / b
(Pdb) p a
20
(Pdb) p b
0
(Pdb) w
  /path/to/crash_handler.py(30)main()
-> results = process_list(pairs)
  /path/to/crash_handler.py(14)process_list()
-> result = divide_numbers(a, b)
> /path/to/crash_handler.py(7)divide_numbers()
-> return a / b
(Pdb) up
> /path/to/crash_handler.py(14)process_list()
-> result = divide_numbers(a, b)
(Pdb) p i
2
(Pdb) p numbers[i]
(20, 0)
```

## Tips for Effective Debugging

### 1. Strategic Breakpoint Placement

Place breakpoints where state changes or before suspected problem areas:

```python
# Before complex logic
import pdb; pdb.set_trace()
result = complex_calculation(data)

# In exception handlers
except ValueError as e:
    import pdb; pdb.set_trace()
    handle_error(e)

# In loops for specific iterations
for i, item in enumerate(items):
    if i == 42:  # Debug specific iteration
        import pdb; pdb.set_trace()
    process(item)
```

### 2. Conditional Breakpoints

Use pdb's conditional breakpoint feature:

```
(Pdb) b myfunction, x > 100 and y < 0
(Pdb) b module.py:42, user.is_admin == False
```

### 3. Quick Variable Inspection

Use `pp` (pretty-print) for complex objects:

```
(Pdb) pp request.__dict__
(Pdb) pp vars(obj)
(Pdb) pp {k: v for k, v in data.items() if v is not None}
```

### 4. Navigate Efficiently

- Use `unt` (until) to run until a specific line
- Use `r` (return) to run until function returns
- Use `j` (jump) to skip lines (use carefully!)

```
(Pdb) unt 150  # Run until line 150
(Pdb) r        # Run until function returns
```

### 5. Debugging Tests

```bash
# Debug specific test
$ python scripts/pdb_runner.py -m pytest tests/test_auth.py::test_login -s

# Debug on first failure
$ python scripts/pdb_runner.py -m pytest --pdb

# Debug on error
$ python scripts/pdb_runner.py -m pytest --pdbcls=IPython.terminal.debugger:TerminalPdb
```
