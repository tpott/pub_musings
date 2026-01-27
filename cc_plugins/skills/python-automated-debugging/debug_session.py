#!/usr/bin/env python3
"""
debug_session.py - Simple interface for Claude to run automated debugging sessions

This is a streamlined tool for Claude to:
1. Specify pdb commands inline or from a file
2. Run a debugging session
3. Get structured output to analyze

Usage:
    # With inline commands
    python debug_session.py script.py "b main" "c" "p variable" "q"
    
    # With command file
    python debug_session.py script.py --file commands.txt
    
    # Module execution
    python debug_session.py -m mymodule "b function" "c" "p x" "q"
    
    # Save output for analysis
    python debug_session.py script.py "b 42" "c" "w" "l" "q" --output debug.log

Examples for Claude:
    # Hypothesis: Bug is in calculate() function
    python debug_session.py buggy.py "b calculate" "c" "p input_data" "n" "p result" "q"
    
    # Investigate crash location
    python debug_session.py crash.py "c" "w" "p locals()" "q"
    
    # Step through specific section
    python debug_session.py app.py "b app.py:50" "c" "n" "n" "n" "p state" "q"
"""

import sys
import os
import pty
import select
import time
import tempfile


def run_pdb_session(target_args, commands):
    """
    Run a pdb debugging session with the given commands.
    
    Args:
        target_args: List of args for the program to debug
        commands: List of pdb commands to execute
    
    Returns:
        String containing the complete session output
    """
    # Build pdb command
    if target_args[0] == '-m':
        pdb_cmd = [sys.executable, '-m', 'pdb', '-m'] + target_args[1:]
    else:
        pdb_cmd = [sys.executable, '-m', 'pdb'] + target_args
    
    # Create PTY
    master_fd, slave_fd = pty.openpty()
    
    pid = os.fork()
    
    if pid == 0:  # Child
        os.close(master_fd)
        os.dup2(slave_fd, 0)
        os.dup2(slave_fd, 1)
        os.dup2(slave_fd, 2)
        os.close(slave_fd)
        os.execvp(pdb_cmd[0], pdb_cmd)
    
    # Parent
    os.close(slave_fd)
    
    # Wait for pdb to start
    time.sleep(0.5)
    
    all_output = []
    
    for cmd in commands:
        # Send command
        os.write(master_fd, (cmd + '\n').encode())
        time.sleep(0.3)
        
        # Read response
        output = read_until_prompt(master_fd)
        all_output.append(f"(Pdb) {cmd}\n{output}")
    
    os.close(master_fd)
    os.waitpid(pid, 0)
    
    return ''.join(all_output)


def read_until_prompt(fd, timeout=1.0):
    """Read from fd until we see a prompt or timeout."""
    output = []
    deadline = time.time() + timeout
    
    while time.time() < deadline:
        ready, _, _ = select.select([fd], [], [], 0.1)
        if ready:
            try:
                chunk = os.read(fd, 4096).decode('utf-8', errors='replace')
                if chunk:
                    output.append(chunk)
                    # If we see a prompt, we're done
                    if '(Pdb)' in chunk and chunk.rstrip().endswith('(Pdb)'):
                        break
            except OSError:
                break
    
    return ''.join(output)


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)
    
    args = sys.argv[1:]
    
    # Parse arguments
    target_args = []
    commands = []
    output_file = None
    command_file = None
    
    i = 0
    while i < len(args):
        if args[i] == '--output' or args[i] == '-o':
            output_file = args[i + 1]
            i += 2
        elif args[i] == '--file' or args[i] == '-f':
            command_file = args[i + 1]
            i += 2
        elif args[i] == '-m':
            target_args.append('-m')
            i += 1
            if i < len(args) and not args[i].startswith('-'):
                target_args.append(args[i])
                i += 1
            break
        elif not args[i].startswith('-'):
            target_args.append(args[i])
            i += 1
            break
        else:
            i += 1
    
    # Collect remaining args as either target args or commands
    while i < len(args):
        if args[i] in ['--output', '-o', '--file', '-f']:
            if args[i] in ['--output', '-o']:
                output_file = args[i + 1]
            else:
                command_file = args[i + 1]
            i += 2
        else:
            # Check if this looks like a pdb command
            arg = args[i]
            if (arg.startswith('b ') or arg.startswith('c') or arg.startswith('n') or 
                arg.startswith('s') or arg.startswith('p ') or arg.startswith('l') or
                arg.startswith('w') or arg.startswith('q') or arg.startswith('r') or
                arg in ['b', 'c', 'n', 's', 'l', 'w', 'q', 'r', 'll', 'a']):
                commands.append(arg)
            else:
                target_args.append(arg)
            i += 1
    
    # Load commands from file if specified
    if command_file:
        with open(command_file, 'r') as f:
            file_commands = [line.strip() for line in f if line.strip() and not line.startswith('#')]
        commands.extend(file_commands)
    
    # Ensure we have quit at the end
    if not commands or commands[-1] not in ['q', 'quit', 'exit']:
        commands.append('q')
    
    if not target_args:
        print("Error: No target script or module specified", file=sys.stderr)
        print(__doc__)
        sys.exit(1)
    
    # Run debugging session
    try:
        output = run_pdb_session(target_args, commands)
        
        # Save or print
        if output_file:
            with open(output_file, 'w') as f:
                f.write(output)
            print(f"Debug session output saved to: {output_file}")
            print("\nSession summary:")
            print("-" * 60)
            # Print last 20 lines as preview
            lines = output.split('\n')
            for line in lines[-20:]:
                print(line)
        else:
            print(output)
            
    except Exception as e:
        print(f"Error during debugging session: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()
