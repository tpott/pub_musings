# chatgpt.py

import json
import os
from typing import (Dict, List)

import requests


def chatCompletitions(messages: List[Dict[str, str]]) -> str:
    api_key_file = os.environ.get("OPENAI_API_KEY_FILE")
    assert api_key_file is not None, "Missing env var: OPENAI_API_KEY_FILE"
    openai_api_key = open(api_key_file).read().strip()
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {openai_api_key}",
    }
    data = {
        "model": "gpt-3.5-turbo",
        "messages": messages,
    }
    resp = requests.post("https://api.openai.com/v1/chat/completions", headers=headers, data=json.dumps(data))
    resp.raise_for_status()
    print(resp.json())
    return resp.json()['choices'][0]['message']['content']


def main() -> None:
    print(chatCompletitions([
        {"role": "user", "content": "Say this is a test!"},
    ]))


if __name__ == "__main__":
    main()
