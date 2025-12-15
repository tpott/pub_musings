# anthropic_client.py

from typing import Any, Dict, List, Optional, TypedDict

from anthropic import Anthropic

from serve_config import get_config


# Anthropic pricing (as of Dec 2025) - per 1M tokens
# Source 1: https://platform.claude.com/docs/en/about-claude/pricing
# Source 2: https://platform.claude.com/docs/en/about-claude/models/overview
MODEL_PRICING = {
    "claude-haiku-4-5-20251001": {"input": 1.00, "output": 5.00},
    "claude-opus-4-5-20251101": {"input": 5.00, "output": 25.00},
    "claude-sonnet-4-5-20250929": {"input": 3.00, "output": 15.00},
}

DEFAULT_MODEL = "claude-sonnet-4-5-20250929"


class CompletionResult(TypedDict):
    content: str
    response: Any  # Full response for tool_use handling
    usage: Dict[str, int]  # input_tokens, output_tokens
    cost: Dict[str, float]  # input_cost, output_cost, total


def _extract_text(response: Any) -> str:
    """Extract text content from Anthropic response."""
    text_parts = []
    for block in response.content:
        if hasattr(block, "text"):
            text_parts.append(block.text)
    return "".join(text_parts)


def _extract_tool_use(response: Any) -> Optional[Dict[str, Any]]:
    """Extract tool_use block from response if present."""
    for block in response.content:
        if block.type == "tool_use":
            return {
                "id": block.id,
                "name": block.name,
                "input": block.input,
            }
    return None


def anthropic_completion(
    messages: List[Dict[str, Any]],
    context: Dict[str, Any],
    tools: Optional[List[Dict]] = None,
    system_prompt: Optional[str] = None,
) -> CompletionResult:
    """
    Call Anthropic API with token tracking.
    Returns structured result with content, usage, and cost.
    """
    config = get_config()
    client = Anthropic(api_key=config["anthropic_api_key"])
    model = context.get("model", DEFAULT_MODEL)

    # Build request kwargs
    kwargs: Dict[str, Any] = {
        "model": model,
        "max_tokens": context.get("max_tokens", 4096),
        "messages": messages,
    }

    if system_prompt:
        kwargs["system"] = system_prompt

    if tools:
        kwargs["tools"] = tools

    response = client.messages.create(**kwargs)

    # Calculate cost
    pricing = MODEL_PRICING.get(model, {"input": 3.00, "output": 15.00})
    input_cost = (response.usage.input_tokens / 1_000_000) * pricing["input"]
    output_cost = (response.usage.output_tokens / 1_000_000) * pricing["output"]

    # Verbose logging
    if context.get("verbose", 0) > 0:
        print(
            f"Usage: {response.usage.input_tokens} input tokens, "
            f"{response.usage.output_tokens} output tokens"
        )
        print(
            f"Estimated cost: ${input_cost + output_cost:.6f} "
            f"(input: ${input_cost:.6f}, output: ${output_cost:.6f})"
        )

    return {
        "content": _extract_text(response),
        "response": response,
        "tool_use": _extract_tool_use(response),
        "stop_reason": response.stop_reason,
        "usage": {
            "input_tokens": response.usage.input_tokens,
            "output_tokens": response.usage.output_tokens,
        },
        "cost": {
            "input": input_cost,
            "output": output_cost,
            "total": input_cost + output_cost,
        },
    }


def anthropic_tool_result(
    messages: List[Dict[str, Any]],
    tool_use_id: str,
    tool_result: str,
    context: Dict[str, Any],
    tools: Optional[List[Dict]] = None,
    system_prompt: Optional[str] = None,
) -> CompletionResult:
    """
    Continue conversation with a tool result.
    Appends the tool_result message and calls the API again.
    """
    # Append tool result to messages
    messages_with_result = messages + [
        {
            "role": "user",
            "content": [
                {
                    "type": "tool_result",
                    "tool_use_id": tool_use_id,
                    "content": tool_result,
                }
            ],
        }
    ]

    return anthropic_completion(
        messages_with_result,
        context,
        tools=tools,
        system_prompt=system_prompt,
    )
