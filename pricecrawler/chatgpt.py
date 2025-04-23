# chatgpt.py

import json
import os
from typing import (Dict, List)

import requests


# Price per million tokens in USD (as of April 2025)
MODEL_PRICING = {
    "gpt-3.5-turbo": {"input": 0.50, "output": 1.50},
    "gpt-4o": {"input": 2.50, "output": 10.00},
    "gpt-4o-mini": {"input": 0.60, "output": 2.40},
    "gpt-4.1": {"input": 2.00, "output": 8.00},
    "gpt-4.1-mini": {"input": 0.40, "output": 1.60},
}


# gpt-4o was the default as of 2025-03-01
# gpt-4.1 was the default for writing tools/agents as of 2025-04-22
def chatCompletitions(messages: List[Dict[str, str]], model: str | None) -> str:
    # default to gpt-4.1, or $2 / 1M tokens
    if model is None:
        model = "gpt-4.1"

    api_key_file = os.environ.get("OPENAI_API_KEY_FILE")
    assert api_key_file is not None, "Missing env var: OPENAI_API_KEY_FILE"
    openai_api_key = open(api_key_file).read().strip()
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {openai_api_key}",
    }
    data = {
        "model": model,
        "messages": messages,
    }
    resp = requests.post("https://api.openai.com/v1/chat/completions", headers=headers, data=json.dumps(data))
    resp.raise_for_status()

    response_json = resp.json()
    print(response_json)

    # Extract token usage data
    usage = response_json.get('usage', {})
    prompt_tokens = usage.get('prompt_tokens', 0)
    completion_tokens = usage.get('completion_tokens', 0)
    total_tokens = usage.get('total_tokens', 0)

    # Calculate cost: divide by 1e6 (1M) because the price is $ / 1M tokens
    model_pricing = MODEL_PRICING.get(model, {"input": 2.00, "output": 8.00})  # Default to gpt-4.1 pricing
    input_cost = prompt_tokens * model_pricing["input"] / 1e6
    output_cost = completion_tokens * model_pricing["output"] / 1e6
    total_cost = input_cost + output_cost

    print(f"Usage: {prompt_tokens} prompt tokens, {completion_tokens} completion tokens, {total_tokens} total tokens")
    print(f"Estimated cost: ${total_cost:.6f} (input: ${input_cost:.6f}, output: ${output_cost:.6f})")

    return response_json['choices'][0]['message']['content']


def main() -> None:
    print(chatCompletitions([
        {"role": "user", "content": "Say this is a test!"},
    ]))


if __name__ == "__main__":
    main()
