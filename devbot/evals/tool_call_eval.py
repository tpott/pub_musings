# evals/tool_call_eval.py

"""
Live API evaluation script to verify the model always produces tool calls.
Runs adversarial test cases against the Anthropic API.
"""

import argparse
import sys
from collections import Counter
from typing import Any, Dict, List

# Add parent directory to path for imports
sys.path.insert(0, sys.path[0] + "/..")

from anthropic_client import anthropic_completion, DEFAULT_MODEL
from dev_agent import SYSTEM_PROMPT, TOOLS
from test_cases import ADVERSARIAL_CASES, SESSION_TOOL_CASES


def run_single_case(
    test_input: str,
    context: Dict[str, Any],
    verbose: bool = False,
) -> Dict[str, Any]:
    """
    Run a single test case through the API.
    Returns result dict with tool_use info, stop_reason, etc.
    """
    messages = [{"role": "user", "content": test_input}]

    result = anthropic_completion(
        messages=messages,
        context=context,
        tools=TOOLS,
        system_prompt=SYSTEM_PROMPT,
    )

    tool_use = result.get("tool_use")
    stop_reason = result.get("stop_reason")

    output = {
        "input": test_input,
        "stop_reason": stop_reason,
        "tool_called": tool_use["name"] if tool_use else None,
        "has_tool_use": stop_reason == "tool_use",
        "content": result.get("content", ""),
        "usage": result.get("usage", {}),
        "cost": result.get("cost", {}),
    }

    if verbose:
        print(f"\n  Input: {test_input[:50]}...")
        print(f"  Stop reason: {stop_reason}")
        print(f"  Tool called: {output['tool_called']}")
        if output["content"]:
            print(f"  Content: {output['content'][:100]}...")

    return output


def run_adversarial_eval(
    context: Dict[str, Any],
    verbose: bool = False,
) -> Dict[str, Any]:
    """
    Run all adversarial test cases.
    Returns summary with pass/fail counts and failures list.
    """
    results = []
    failures = []

    print("\nRunning adversarial cases...")
    for case in ADVERSARIAL_CASES:
        if verbose:
            print(f"\n[{case['category']}]", end="")

        result = run_single_case(case["input"], context, verbose)
        result["category"] = case["category"]
        results.append(result)

        if not result["has_tool_use"]:
            failures.append(result)

    passed = len(results) - len(failures)
    total = len(results)
    success_rate = (passed / total * 100) if total > 0 else 0

    return {
        "total": total,
        "passed": passed,
        "failed": len(failures),
        "success_rate": success_rate,
        "failures": failures,
        "results": results,
    }


def run_session_tool_eval(
    context: Dict[str, Any],
    verbose: bool = False,
) -> Dict[str, Any]:
    """
    Run session tool selection cases.
    Verifies the model selects the correct tool for session management inputs.
    """
    results = []
    failures = []

    print("\nRunning session tool selection cases...")
    for case in SESSION_TOOL_CASES:
        if verbose:
            print(f"\n[{case['expected_tool']}]", end="")

        result = run_single_case(case["input"], context, verbose)
        result["expected_tool"] = case["expected_tool"]
        results.append(result)

        # Check if correct tool was selected
        if result["tool_called"] != case["expected_tool"]:
            failures.append(result)

    passed = len(results) - len(failures)
    total = len(results)
    success_rate = (passed / total * 100) if total > 0 else 0

    return {
        "total": total,
        "passed": passed,
        "failed": len(failures),
        "success_rate": success_rate,
        "failures": failures,
        "results": results,
    }


def get_tool_distribution(results: List[Dict[str, Any]]) -> Dict[str, int]:
    """Calculate distribution of tool calls across all results."""
    tools = [r["tool_called"] for r in results if r["tool_called"]]
    return dict(Counter(tools))


def print_report(
    adversarial: Dict[str, Any],
    session_tool: Dict[str, Any],
    verbose: bool = False,
) -> None:
    """Print the evaluation report."""
    print("\n" + "=" * 60)
    print("Tool Call Eval Results")
    print("=" * 60)

    # Adversarial cases
    print(f"\nADVERSARIAL CASES (tool_use required)")
    print(f"Total: {adversarial['total']} | Passed: {adversarial['passed']} | "
          f"Failed: {adversarial['failed']} | Success Rate: {adversarial['success_rate']:.1f}%")

    if adversarial["failures"]:
        print("\nFailures:")
        for f in adversarial["failures"]:
            input_preview = f["input"][:40] + "..." if len(f["input"]) > 40 else f["input"]
            if not input_preview:
                input_preview = "(empty string)"
            print(f'  [FAIL] "{input_preview}" -> stop_reason={f["stop_reason"]} (no tool call)')

    # Session tool selection
    print(f"\nSESSION TOOL SELECTION (correct tool required)")
    print(f"Total: {session_tool['total']} | Passed: {session_tool['passed']} | "
          f"Failed: {session_tool['failed']} | Success Rate: {session_tool['success_rate']:.1f}%")

    if session_tool["failures"]:
        print("\nFailures:")
        for f in session_tool["failures"]:
            input_preview = f["input"][:40] + "..." if len(f["input"]) > 40 else f["input"]
            print(f'  [FAIL] "{input_preview}" -> expected {f["expected_tool"]}, got {f["tool_called"]}')

    # Tool distribution
    all_results = adversarial["results"] + session_tool["results"]
    distribution = get_tool_distribution(all_results)
    total_tool_calls = sum(distribution.values())

    print(f"\nTOOL DISTRIBUTION (all cases):")
    for tool, count in sorted(distribution.items(), key=lambda x: -x[1]):
        pct = (count / total_tool_calls * 100) if total_tool_calls > 0 else 0
        print(f"  {tool}: {count} ({pct:.1f}%)")

    # Cost summary
    total_cost = sum(r["cost"].get("total", 0) for r in all_results)
    total_input_tokens = sum(r["usage"].get("input_tokens", 0) for r in all_results)
    total_output_tokens = sum(r["usage"].get("output_tokens", 0) for r in all_results)

    print(f"\nCOST SUMMARY:")
    print(f"  Total tokens: {total_input_tokens + total_output_tokens:,} "
          f"({total_input_tokens:,} input, {total_output_tokens:,} output)")
    print(f"  Estimated cost: ${total_cost:.4f}")

    print("\n" + "=" * 60)


def main():
    parser = argparse.ArgumentParser(
        description="Evaluate tool call robustness of the dev agent"
    )
    parser.add_argument(
        "--verbose", "-v",
        action="store_true",
        help="Show detailed output for each test case"
    )
    parser.add_argument(
        "--model", "-m",
        type=str,
        default=DEFAULT_MODEL,
        help=f"Model to test (default: {DEFAULT_MODEL})"
    )

    args = parser.parse_args()

    context = {
        "verbose": 1 if args.verbose else 0,
        "model": args.model,
    }

    print(f"Running eval with model: {args.model}")
    print(f"System prompt length: {len(SYSTEM_PROMPT)} chars")
    print(f"Number of tools: {len(TOOLS)}")

    # Run evaluations
    adversarial_results = run_adversarial_eval(context, args.verbose)
    session_tool_results = run_session_tool_eval(context, args.verbose)

    # Print report
    print_report(adversarial_results, session_tool_results, args.verbose)

    # Exit with error code if any failures
    total_failures = adversarial_results["failed"] + session_tool_results["failed"]
    sys.exit(1 if total_failures > 0 else 0)


if __name__ == "__main__":
    main()
