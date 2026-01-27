#!/usr/bin/env python3
"""
debug_assistant.py - Helper for Claude to generate and analyze debug plans

This module provides utilities for Claude to:
1. Generate strategic debug plans based on error analysis
2. Analyze collected debug data
3. Form new hypotheses and iterate debugging

Typical workflow:
1. Analyze error/symptom → generate initial debug plan
2. Run automated_debugger.py with the plan
3. Analyze results → form new hypothesis
4. Generate refined debug plan
5. Repeat until bug found
"""

import json
import ast
import os
import re
from typing import List, Dict, Any, Optional


class DebugPlanGenerator:
    """Generate debug plans based on code analysis and error information."""
    
    @staticmethod
    def analyze_traceback(traceback_text: str) -> Dict[str, Any]:
        """
        Analyze a Python traceback to identify key information.
        
        Returns dict with:
            - error_type: Exception class name
            - error_message: Exception message
            - error_location: (filename, line_number) of error
            - call_stack: List of frames in reverse order
        """
        lines = traceback_text.strip().split('\n')
        
        result = {
            'error_type': None,
            'error_message': None,
            'error_location': None,
            'call_stack': []
        }
        
        # Parse error type and message (last line)
        if lines:
            last_line = lines[-1]
            if ':' in last_line:
                error_type, error_message = last_line.split(':', 1)
                result['error_type'] = error_type.strip()
                result['error_message'] = error_message.strip()
        
        # Parse stack frames
        i = 0
        while i < len(lines):
            line = lines[i]
            if line.strip().startswith('File "'):
                # Extract filename and line number
                match = re.search(r'File "([^"]+)", line (\d+)', line)
                if match:
                    filename = match.group(1)
                    lineno = int(match.group(2))
                    
                    # Next line might have the function name
                    func_name = None
                    if i + 1 < len(lines):
                        next_line = lines[i + 1].strip()
                        if next_line.startswith('in '):
                            func_name = next_line[3:]
                    
                    # Line after might have source code
                    source = None
                    if i + 2 < len(lines):
                        source = lines[i + 2].strip()
                    
                    frame = {
                        'filename': filename,
                        'line_number': lineno,
                        'function': func_name,
                        'source': source
                    }
                    result['call_stack'].append(frame)
                    
                    # The error location is the last frame
                    result['error_location'] = (filename, lineno)
            
            i += 1
        
        return result
    
    @staticmethod
    def generate_initial_plan(
        script_path: str,
        error_info: Optional[Dict[str, Any]] = None,
        symptom_description: Optional[str] = None,
        suspected_functions: Optional[List[str]] = None
    ) -> Dict[str, Any]:
        """
        Generate an initial debug plan.
        
        Args:
            script_path: Path to script being debugged
            error_info: Dict from analyze_traceback() if there's an error
            symptom_description: Free-text description of the problem
            suspected_functions: List of function names to investigate
        
        Returns:
            Debug plan dict suitable for automated_debugger.py
        """
        plan = {
            'breakpoints': [],
            'collect_vars': [],
            'commands': ['c'],  # Continue to next breakpoint
            'hypothesis': '',
            'strategy': ''
        }
        
        # Strategy based on error info
        if error_info and error_info.get('error_location'):
            filename, lineno = error_info['error_location']
            error_type = error_info.get('error_type', 'Error')
            
            # Set breakpoint a few lines before the error
            plan['breakpoints'].append({
                'file': filename,
                'line': max(1, lineno - 3),
            })
            
            # Also set breakpoint at error line to see state just before
            plan['breakpoints'].append({
                'file': filename,
                'line': lineno,
            })
            
            plan['hypothesis'] = (
                f"Investigating {error_type} at {filename}:{lineno}. "
                f"Will examine state leading up to the error."
            )
            plan['strategy'] = (
                "Set breakpoints before and at error location to inspect "
                "variable state and execution flow."
            )
            
            # Collect variables from error frame if we can identify them
            if error_info.get('call_stack'):
                frame = error_info['call_stack'][-1]
                if frame.get('source'):
                    # Try to extract variable names from source line
                    vars_in_line = DebugPlanGenerator._extract_variables(
                        frame['source']
                    )
                    plan['collect_vars'].extend(vars_in_line[:5])
        
        # Strategy based on suspected functions
        elif suspected_functions:
            plan['hypothesis'] = (
                f"Investigating functions: {', '.join(suspected_functions)}. "
                f"Will examine inputs and outputs."
            )
            plan['strategy'] = (
                "Set breakpoints at entry to suspected functions to trace "
                "execution flow and inspect parameters."
            )
            
            # Would need to parse code to find line numbers of functions
            # For now, just note them
            plan['target_functions'] = suspected_functions
        
        # Generic exploration strategy
        else:
            plan['hypothesis'] = symptom_description or "General investigation"
            plan['strategy'] = (
                "Run with minimal breakpoints to understand execution flow. "
                "Will refine based on initial findings."
            )
            plan['collect_vars'] = []  # Collect all locals
        
        return plan
    
    @staticmethod
    def _extract_variables(source_line: str) -> List[str]:
        """Extract variable names from a line of Python code."""
        # Simple regex-based extraction
        # Matches: word characters not followed by (
        var_pattern = r'\b([a-zA-Z_][a-zA-Z0-9_]*)\b(?!\s*\()'
        variables = re.findall(var_pattern, source_line)
        
        # Filter out Python keywords
        keywords = {'if', 'else', 'for', 'while', 'def', 'class', 'return',
                   'import', 'from', 'True', 'False', 'None', 'and', 'or', 'not'}
        variables = [v for v in variables if v not in keywords]
        
        return list(dict.fromkeys(variables))  # Remove duplicates, preserve order
    
    @staticmethod
    def find_function_line_numbers(script_path: str, function_names: List[str]) -> Dict[str, int]:
        """
        Find line numbers where functions are defined.
        
        Returns dict mapping function_name -> line_number
        """
        result = {}
        
        try:
            with open(script_path, 'r') as f:
                tree = ast.parse(f.read(), filename=script_path)
            
            for node in ast.walk(tree):
                if isinstance(node, ast.FunctionDef):
                    if node.name in function_names:
                        result[node.name] = node.lineno
        except Exception as e:
            print(f"Warning: Could not parse {script_path}: {e}")
        
        return result


class DebugResultsAnalyzer:
    """Analyze results from automated debugging runs."""
    
    @staticmethod
    def load_results(results_file: str) -> Dict[str, Any]:
        """Load debug results from JSON file."""
        with open(results_file, 'r') as f:
            return json.load(f)
    
    @staticmethod
    def summarize_results(results: Dict[str, Any]) -> str:
        """
        Generate human-readable summary of debug results.
        
        Returns multi-line string summary.
        """
        lines = []
        lines.append(f"Debug Session Summary")
        lines.append("=" * 50)
        lines.append(f"Breakpoint hits: {results.get('breakpoint_hits', 0)}")
        lines.append(f"Data points: {len(results.get('data', []))}")
        lines.append("")
        
        data_points = results.get('data', [])
        if data_points:
            lines.append("Execution trace:")
            lines.append("-" * 50)
            
            for i, point in enumerate(data_points, 1):
                event = point.get('event', 'unknown')
                location = f"{point.get('filename', '?')}:{point.get('line_number', '?')}"
                func = point.get('function', '?')
                source = point.get('source_line', '')
                
                lines.append(f"{i}. [{event}] {location} in {func}")
                if source:
                    lines.append(f"   Source: {source}")
                
                # Show variables
                locals_dict = point.get('locals', {})
                if locals_dict:
                    lines.append(f"   Variables:")
                    for var_name, var_value in list(locals_dict.items())[:5]:
                        lines.append(f"     {var_name} = {var_value}")
                
                lines.append("")
        
        return '\n'.join(lines)
    
    @staticmethod
    def analyze_variable_changes(results: Dict[str, Any], var_name: str) -> List[Dict[str, Any]]:
        """
        Track how a variable changes throughout execution.
        
        Returns list of dicts with {location, value} for each point where var is seen.
        """
        changes = []
        
        for point in results.get('data', []):
            locals_dict = point.get('locals', {})
            if var_name in locals_dict:
                changes.append({
                    'location': f"{point.get('filename')}:{point.get('line_number')}",
                    'function': point.get('function'),
                    'value': locals_dict[var_name],
                    'source': point.get('source_line')
                })
        
        return changes
    
    @staticmethod
    def identify_suspicious_values(results: Dict[str, Any]) -> List[Dict[str, Any]]:
        """
        Identify potentially suspicious values (None, empty, negative when shouldn't be, etc.).
        
        Returns list of findings.
        """
        findings = []
        
        for point in results.get('data', []):
            location = f"{point.get('filename')}:{point.get('line_number')}"
            
            for var_name, var_value in point.get('locals', {}).items():
                # Check for common suspicious patterns
                suspicious = False
                reason = None
                
                if var_value is None:
                    suspicious = True
                    reason = "Variable is None"
                elif var_value == "" and var_name not in ['line', 'text', 'message']:
                    suspicious = True
                    reason = "Empty string"
                elif var_value == [] or var_value == {}:
                    suspicious = True
                    reason = "Empty collection"
                elif isinstance(var_value, (int, float)) and var_value < 0:
                    # Negative numbers might be suspicious in some contexts
                    if any(keyword in var_name.lower() for keyword in ['count', 'length', 'size', 'index']):
                        suspicious = True
                        reason = "Negative value for size/count/index"
                
                if suspicious:
                    findings.append({
                        'location': location,
                        'variable': var_name,
                        'value': var_value,
                        'reason': reason,
                        'source': point.get('source_line')
                    })
        
        return findings
    
    @staticmethod
    def suggest_next_steps(results: Dict[str, Any], original_hypothesis: str) -> str:
        """
        Based on results, suggest what to investigate next.
        
        Returns text description of next debugging steps.
        """
        suggestions = []
        
        # Check if we have any data
        if not results.get('data'):
            suggestions.append(
                "No data collected. The breakpoints may not have been hit. "
                "Consider:\n"
                "  - Verify breakpoints are in code that actually executes\n"
                "  - Check if program exits early\n"
                "  - Add breakpoints at program entry point"
            )
            return '\n'.join(suggestions)
        
        # Analyze suspicious values
        suspicious = DebugResultsAnalyzer.identify_suspicious_values(results)
        if suspicious:
            suggestions.append(f"Found {len(suspicious)} suspicious values:")
            for finding in suspicious[:3]:  # Show top 3
                suggestions.append(
                    f"  - {finding['variable']} = {finding['value']} "
                    f"at {finding['location']}: {finding['reason']}"
                )
            if len(suspicious) > 3:
                suggestions.append(f"  ... and {len(suspicious) - 3} more")
            
            suggestions.append("\nNext steps:")
            suggestions.append(
                "  - Set breakpoints before the lines where suspicious values appear\n"
                "  - Trace back to see where these values are assigned\n"
                "  - Check the logic that produces these values"
            )
        else:
            suggestions.append(
                "No obviously suspicious values found. Consider:\n"
                "  - Examining the execution flow more carefully\n"
                "  - Looking for unexpected function calls or branches\n"
                "  - Checking if values are logically correct (not just non-null)"
            )
        
        return '\n'.join(suggestions)


def save_debug_plan(plan: Dict[str, Any], filename: str):
    """Save a debug plan to JSON file."""
    with open(filename, 'w') as f:
        json.dump(plan, f, indent=2)
    print(f"Debug plan saved to: {filename}")


def main():
    """Command-line interface for debug assistant."""
    import argparse
    
    parser = argparse.ArgumentParser(description='Debug planning and analysis assistant')
    subparsers = parser.add_subparsers(dest='command', help='Command to run')
    
    # Generate plan from traceback
    plan_tb = subparsers.add_parser('plan-from-traceback', 
                                     help='Generate debug plan from traceback')
    plan_tb.add_argument('script', help='Script to debug')
    plan_tb.add_argument('traceback_file', help='File containing traceback')
    plan_tb.add_argument('--output', '-o', default='debug_plan.json',
                        help='Output file for debug plan')
    
    # Generate plan for function
    plan_func = subparsers.add_parser('plan-for-functions',
                                      help='Generate plan to debug specific functions')
    plan_func.add_argument('script', help='Script to debug')
    plan_func.add_argument('functions', nargs='+', help='Function names to debug')
    plan_func.add_argument('--output', '-o', default='debug_plan.json',
                          help='Output file for debug plan')
    
    # Analyze results
    analyze = subparsers.add_parser('analyze', help='Analyze debug results')
    analyze.add_argument('results_file', help='JSON file with debug results')
    analyze.add_argument('--var', help='Track specific variable changes')
    
    args = parser.parse_args()
    
    if args.command == 'plan-from-traceback':
        # Read traceback
        with open(args.traceback_file, 'r') as f:
            tb_text = f.read()
        
        # Analyze
        error_info = DebugPlanGenerator.analyze_traceback(tb_text)
        print("Traceback analysis:")
        print(f"  Error: {error_info['error_type']}: {error_info['error_message']}")
        print(f"  Location: {error_info['error_location']}")
        print()
        
        # Generate plan
        plan = DebugPlanGenerator.generate_initial_plan(args.script, error_info=error_info)
        save_debug_plan(plan, args.output)
        
        print("\nGenerated plan:")
        print(f"  Hypothesis: {plan['hypothesis']}")
        print(f"  Strategy: {plan['strategy']}")
        print(f"  Breakpoints: {len(plan['breakpoints'])}")
        
    elif args.command == 'plan-for-functions':
        # Find line numbers
        line_nums = DebugPlanGenerator.find_function_line_numbers(
            args.script, args.functions
        )
        
        # Generate plan
        plan = DebugPlanGenerator.generate_initial_plan(
            args.script,
            suspected_functions=args.functions
        )
        
        # Add breakpoints at function definitions
        for func, lineno in line_nums.items():
            plan['breakpoints'].append({
                'file': args.script,
                'line': lineno,
            })
        
        save_debug_plan(plan, args.output)
        
        print("\nGenerated plan:")
        print(f"  Target functions: {', '.join(args.functions)}")
        print(f"  Breakpoints set: {len(plan['breakpoints'])}")
        
    elif args.command == 'analyze':
        # Load and analyze results
        results = DebugResultsAnalyzer.load_results(args.results_file)
        
        # Print summary
        summary = DebugResultsAnalyzer.summarize_results(results)
        print(summary)
        
        # Track specific variable if requested
        if args.var:
            print(f"\nVariable tracking: {args.var}")
            print("-" * 50)
            changes = DebugResultsAnalyzer.analyze_variable_changes(results, args.var)
            for change in changes:
                print(f"{change['location']}: {args.var} = {change['value']}")
        
        # Identify suspicious values
        print("\nSuspicious values:")
        print("-" * 50)
        suspicious = DebugResultsAnalyzer.identify_suspicious_values(results)
        if suspicious:
            for finding in suspicious:
                print(f"{finding['location']}: {finding['variable']} = {finding['value']}")
                print(f"  Reason: {finding['reason']}")
        else:
            print("None found")
        
        # Suggest next steps
        print("\nSuggested next steps:")
        print("-" * 50)
        suggestions = DebugResultsAnalyzer.suggest_next_steps(results, "")
        print(suggestions)


if __name__ == "__main__":
    main()
