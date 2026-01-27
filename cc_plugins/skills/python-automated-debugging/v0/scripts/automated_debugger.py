#!/usr/bin/env python3
"""
automated_debugger.py - Programmatic pdb interface for automated debugging

This enables Claude to automate debugging by:
1. Setting breakpoints based on hypotheses
2. Running the program and collecting state at breakpoints
3. Making decisions about where to investigate next
4. Gathering diagnostic information systematically

Usage:
    python automated_debugger.py script.py [args...]
    python automated_debugger.py -m module [args...]

The script runs the target program under pdb control and allows commands to be
scripted or provided via a command file.
"""

import sys
import os
import pdb
import bdb
import traceback
import json
from io import StringIO
from contextlib import redirect_stdout, redirect_stderr


class AutomatedDebugger(pdb.Pdb):
    """
    Extended Pdb class that can be controlled programmatically.
    
    This allows Claude to:
    - Set breakpoints programmatically
    - Capture variable state at breakpoints
    - Make decisions about execution flow
    - Collect diagnostic data
    """
    
    def __init__(self, commands=None, collect_vars=None):
        """
        Initialize automated debugger.
        
        Args:
            commands: List of pdb commands to execute in sequence
            collect_vars: List of variable names to collect at each breakpoint
        """
        super().__init__()
        self.commands_to_run = commands or []
        self.vars_to_collect = collect_vars or []
        self.command_index = 0
        self.collected_data = []
        self.breakpoint_hits = []
        
    def user_line(self, frame):
        """Called when debugger stops at a line."""
        # Collect data at this point
        self._collect_state(frame, 'line')
        
        # Run automated commands if any
        if self.command_index < len(self.commands_to_run):
            cmd = self.commands_to_run[self.command_index]
            self.command_index += 1
            print(f"[AUTO] Executing: {cmd}")
            self.onecmd(cmd)
        else:
            # No more automated commands, call parent
            super().user_line(frame)
    
    def user_return(self, frame, return_value):
        """Called when a function returns."""
        self._collect_state(frame, 'return', return_value=return_value)
        super().user_return(frame, return_value)
    
    def user_exception(self, frame, exc_info):
        """Called when an exception occurs."""
        self._collect_state(frame, 'exception', exc_info=exc_info)
        super().user_exception(frame, exc_info)
    
    def _collect_state(self, frame, event_type, **extra):
        """
        Collect state information at current execution point.
        
        Args:
            frame: Current execution frame
            event_type: Type of event ('line', 'return', 'exception')
            **extra: Additional event-specific data
        """
        state = {
            'event': event_type,
            'filename': frame.f_code.co_filename,
            'line_number': frame.f_lineno,
            'function': frame.f_code.co_name,
            'locals': {},
            'source_line': self._get_source_line(frame),
        }
        
        # Collect specified variables
        for var_name in self.vars_to_collect:
            if var_name in frame.f_locals:
                try:
                    value = frame.f_locals[var_name]
                    # Try to serialize; if not, use repr
                    state['locals'][var_name] = self._serialize_value(value)
                except Exception as e:
                    state['locals'][var_name] = f"<Error: {e}>"
        
        # If no specific vars requested, collect all locals (limited)
        if not self.vars_to_collect:
            for name, value in list(frame.f_locals.items())[:20]:  # Limit to 20
                try:
                    state['locals'][name] = self._serialize_value(value)
                except:
                    pass
        
        # Add extra data
        state.update(extra)
        
        self.collected_data.append(state)
    
    def _get_source_line(self, frame):
        """Get the source code line at current position."""
        try:
            import linecache
            filename = frame.f_code.co_filename
            lineno = frame.f_lineno
            line = linecache.getline(filename, lineno).strip()
            return line
        except:
            return "<source unavailable>"
    
    def _serialize_value(self, value):
        """Attempt to serialize a value for storage."""
        # Try direct JSON serialization first
        try:
            json.dumps(value)
            return value
        except (TypeError, ValueError):
            pass
        
        # For common types, use repr
        if isinstance(value, (list, tuple, set, frozenset)):
            if len(value) <= 10:
                return repr(value)
            return f"{type(value).__name__}[{len(value)} items]"
        
        if isinstance(value, dict):
            if len(value) <= 10:
                return repr(value)
            return f"dict[{len(value)} items]"
        
        # For objects, try to get useful representation
        if hasattr(value, '__dict__'):
            return f"<{type(value).__name__} instance>"
        
        return repr(value)[:100]  # Limit length
    
    def get_collected_data(self):
        """Return all collected data as JSON-serializable dict."""
        return {
            'breakpoint_hits': len(self.collected_data),
            'data': self.collected_data
        }


def run_with_automated_debugger(script_path, script_args, debug_plan=None):
    """
    Run a script under automated debugger control.
    
    Args:
        script_path: Path to script to debug
        script_args: Arguments to pass to script
        debug_plan: Dict with 'breakpoints', 'commands', 'collect_vars'
    """
    debug_plan = debug_plan or {}
    
    # Extract debug plan
    breakpoints = debug_plan.get('breakpoints', [])
    commands = debug_plan.get('commands', [])
    collect_vars = debug_plan.get('collect_vars', [])
    
    # Create automated debugger
    debugger = AutomatedDebugger(commands=commands, collect_vars=collect_vars)
    
    # Set breakpoints
    for bp in breakpoints:
        if isinstance(bp, dict):
            filename = bp.get('file', script_path)
            lineno = bp.get('line')
            condition = bp.get('condition')
            
            if lineno:
                debugger.set_break(filename, lineno, cond=condition)
                print(f"[AUTO] Breakpoint set: {filename}:{lineno}" + 
                      (f" if {condition}" if condition else ""))
        elif isinstance(bp, int):
            # Just a line number
            debugger.set_break(script_path, bp)
            print(f"[AUTO] Breakpoint set: {script_path}:{bp}")
    
    # Set up sys.argv for the script
    sys.argv = [script_path] + script_args
    
    # Run the script under debugger
    try:
        with open(script_path) as f:
            code = compile(f.read(), script_path, 'exec')
        
        # Create globals for the script
        script_globals = {
            '__name__': '__main__',
            '__file__': script_path,
        }
        
        print(f"\n[AUTO] Running {script_path} under automated debugger...\n")
        debugger.run(code, script_globals)
        
    except SystemExit:
        pass
    except Exception as e:
        print(f"\n[AUTO] Exception during execution: {e}")
        traceback.print_exc()
    
    # Return collected data
    return debugger.get_collected_data()


def load_debug_plan(plan_file):
    """Load debug plan from JSON file."""
    with open(plan_file, 'r') as f:
        return json.load(f)


def save_debug_results(results, output_file):
    """Save debug results to JSON file."""
    with open(output_file, 'w') as f:
        json.dump(results, f, indent=2)
    print(f"\n[AUTO] Results saved to: {output_file}")


def main():
    """Main entry point."""
    import argparse
    
    parser = argparse.ArgumentParser(
        description='Run Python script under automated debugger control'
    )
    parser.add_argument('script', help='Python script to debug')
    parser.add_argument('args', nargs='*', help='Arguments to pass to script')
    parser.add_argument('--plan', '-p', 
                       help='JSON file with debug plan (breakpoints, commands, vars to collect)')
    parser.add_argument('--output', '-o', default='debug_results.json',
                       help='Output file for results (default: debug_results.json)')
    parser.add_argument('--breakpoint', '-b', action='append',
                       help='Set breakpoint at line number (can be used multiple times)')
    parser.add_argument('--var', '-v', action='append',
                       help='Variable to collect at breakpoints (can be used multiple times)')
    
    args = parser.parse_args()
    
    # Load debug plan from file or command line
    if args.plan:
        debug_plan = load_debug_plan(args.plan)
    else:
        debug_plan = {
            'breakpoints': [int(bp) for bp in (args.breakpoint or [])],
            'collect_vars': args.var or [],
            'commands': ['c']  # Default: continue to next breakpoint
        }
    
    # Run with automated debugger
    results = run_with_automated_debugger(
        args.script,
        args.args,
        debug_plan
    )
    
    # Save results
    save_debug_results(results, args.output)
    
    # Print summary
    print(f"\n[AUTO] Summary:")
    print(f"  Breakpoint hits: {results['breakpoint_hits']}")
    print(f"  Data points collected: {len(results['data'])}")
    
    return results


if __name__ == "__main__":
    main()
