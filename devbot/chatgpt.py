# chatgpt.py

from typing import Optional, TypedDict

import asyncio
from openai import AsyncOpenAI

from serve_config import get_config


# Price per million tokens in USD (as of April 2025)
# Source: https://platform.openai.com/docs/pricing
MODEL_PRICING = {
    "gpt-4o": {"input": 2.50, "output": 10.00},
    "gpt-4o-mini": {"input": 0.15, "output": 0.60},
    "gpt-4.1": {"input": 2.00, "output": 8.00},
    "gpt-4.1-mini": {"input": 0.40, "output": 1.60},
    "gpt-5": {"input": 1.25, "output": 10.00},
    "gpt-5-mini": {"input": 0.25, "output": 2.00},
}


class CompletionResult(TypedDict):
    """Structured return type for chat completions."""

    content: str
    usage: dict[str, int]
    cost: dict[str, float]


class Context(TypedDict):
    model: Optional[str]
    verbose: int


# gpt-4o was the default as of 2025-03-01
# gpt-4.1 was the default for writing tools/agents as of 2025-04-22
# gpt-5 was the default as of 2025-10-28
async def chat_completions(
    messages: list[dict[str, str]],
    context: Context,
) -> CompletionResult:
    """Get chat completions from OpenAI with cost tracking."""
    # default to gpt-5, or $1.25 / 1M tokens
    if context["model"] is None:
        context["model"] = "gpt-5"

    config = get_config()
    client = AsyncOpenAI(api_key=config["open_api_key"])

    response = await client.chat.completions.create(
        model=context["model"],
        messages=messages,
    )

    # Extract usage from response object
    usage = {
        "prompt_tokens": response.usage.prompt_tokens,
        "completion_tokens": response.usage.completion_tokens,
        "total_tokens": response.usage.total_tokens,
    }

    # Calculate cost: divide by 1e6 (1M) because the price is $ / 1M tokens
    model_pricing = MODEL_PRICING.get(
        context["model"], {"input": 2.00, "output": 8.00}
    )  # Default to gpt-4.1 pricing
    input_cost = usage["prompt_tokens"] * model_pricing["input"] / 1e6
    output_cost = usage["completion_tokens"] * model_pricing["output"] / 1e6

    cost = {
        "input_cost": input_cost,
        "output_cost": output_cost,
        "total_cost": input_cost + output_cost,
    }

    if context["verbose"] > 0:
        print(
            f"Usage: {usage['prompt_tokens']} prompt tokens, {usage['completion_tokens']} completion tokens, {usage['total_tokens']} total tokens"
        )
        print(
            f"Estimated cost: ${cost['total_cost']:.6f} (input: ${cost['input_cost']:.6f}, output: ${cost['output_cost']:.6f})"
        )

    return {
        "content": response.choices[0].message.content,
        "usage": usage,
        "cost": cost,
    }


async def main() -> None:
    result = await chat_completions(
        [
            {"role": "user", "content": "Say this is a test!"},
        ]
    )
    print(result["content"])


if __name__ == "__main__":
    asyncio.run(main())
