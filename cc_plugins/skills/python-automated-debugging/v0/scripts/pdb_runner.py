#!/usr/bin/env python3
"""
pdb_runner.py - Run Python programs under pdb with proper PTY support

This script creates a pseudo-terminal (PTY) to run programs under pdb,
which is required for pdb's interactive features to work correctly.

Usage:
    python pdb_runner.py script.py [args...]
    python pdb_runner.py -m module [args...]

Examples:
    python pdb_runner.py server.py --port 8000
    python pdb_runner.py -m flask run --debug
    python pdb_runner.py manage.py runserver
"""

import sys
import os
import pty
import subprocess
import shlex


def run_with_pdb(args):
    """
    Run a Python command under pdb with PTY support.
    
    Args:
        args: Command-line arguments (script/module and its arguments)
    """
    if not args:
        print("Usage: pdb_runner.py script.py [args...]", file=sys.stderr)
        print("   or: pdb_runner.py -m module [args...]", file=sys.stderr)
        sys.exit(1)
    
    # Build the pdb command
    if args[0] == '-m':
        # Module execution: python -m pdb -m module args...
        if len(args) < 2:
            print("Error: -m flag requires a module name", file=sys.stderr)
            sys.exit(1)
        pdb_cmd = [sys.executable, '-m', 'pdb', '-m'] + args[1:]
    else:
        # Script execution: python -m pdb script.py args...
        pdb_cmd = [sys.executable, '-m', 'pdb'] + args
    
    # Run the command in a PTY
    # pty.spawn() creates a pseudo-terminal and runs the command in it
    # This gives pdb the terminal it needs for interactive debugging
    try:
        pty.spawn(pdb_cmd)
    except OSError as e:
        print(f"Error running command: {e}", file=sys.stderr)
        sys.exit(1)


def main():
    """Main entry point."""
    # Get all arguments after this script name
    args = sys.argv[1:]
    
    # Show usage if no arguments
    if not args:
        print(__doc__)
        sys.exit(1)
    
    # Run the command under pdb with PTY
    run_with_pdb(args)


if __name__ == "__main__":
    main()
