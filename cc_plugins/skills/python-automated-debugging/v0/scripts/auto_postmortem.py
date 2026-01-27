#!/usr/bin/env python3
"""
auto_postmortem.py - Wrapper to automatically enter pdb on unhandled exceptions

This script runs a Python program and automatically enters post-mortem debugging
if the program crashes with an unhandled exception.

Usage:
    python auto_postmortem.py script.py [args...]
    python auto_postmortem.py -m module [args...]

Examples:
    python auto_postmortem.py my_script.py --input data.csv
    python auto_postmortem.py -m pytest tests/
    python auto_postmortem.py server.py --port 8000

The difference from pdb_runner.py:
- pdb_runner.py: Starts in debugger immediately, requires manual breakpoints/continue
- auto_postmortem.py: Runs normally, only enters debugger on crash
"""

import sys
import runpy
import pdb


def run_script_with_postmortem(script_path, script_args):
    """
    Run a Python script with automatic post-mortem debugging on crash.
    
    Args:
        script_path: Path to the script to run
        script_args: Arguments to pass to the script
    """
    # Set sys.argv to make it look like we ran the script directly
    sys.argv = [script_path] + script_args
    
    try:
        # Run the script in the current namespace
        # runpy.run_path executes the script as if it were __main__
        runpy.run_path(script_path, run_name='__main__')
    except Exception:
        # On any exception, enter post-mortem debugger
        print("\n" + "="*70)
        print("EXCEPTION OCCURRED - Entering post-mortem debugger")
        print("="*70 + "\n")
        
        # Get the traceback and enter post-mortem mode
        import traceback
        traceback.print_exc()
        print("\n" + "="*70)
        print("You can now inspect the state at the point of failure")
        print("Use 'w' to see the stack, 'u'/'d' to move up/down the stack")
        print("Use 'p variable' to inspect variables, 'q' to quit")
        print("="*70 + "\n")
        
        pdb.post_mortem()


def run_module_with_postmortem(module_name, module_args):
    """
    Run a Python module with automatic post-mortem debugging on crash.
    
    Args:
        module_name: Name of the module to run
        module_args: Arguments to pass to the module
    """
    # Set sys.argv for the module
    sys.argv = [module_name] + module_args
    
    try:
        # Run the module as __main__
        runpy.run_module(module_name, run_name='__main__', alter_sys=True)
    except Exception:
        print("\n" + "="*70)
        print("EXCEPTION OCCURRED - Entering post-mortem debugger")
        print("="*70 + "\n")
        
        import traceback
        traceback.print_exc()
        print("\n" + "="*70)
        print("You can now inspect the state at the point of failure")
        print("Use 'w' to see the stack, 'u'/'d' to move up/down the stack")
        print("Use 'p variable' to inspect variables, 'q' to quit")
        print("="*70 + "\n")
        
        pdb.post_mortem()


def main():
    """Main entry point."""
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)
    
    args = sys.argv[1:]
    
    # Check if we're running a module or script
    if args[0] == '-m':
        if len(args) < 2:
            print("Error: -m flag requires a module name", file=sys.stderr)
            sys.exit(1)
        
        module_name = args[1]
        module_args = args[2:]
        run_module_with_postmortem(module_name, module_args)
    else:
        script_path = args[0]
        script_args = args[1:]
        run_script_with_postmortem(script_path, script_args)


if __name__ == "__main__":
    main()
