# chatgpt.py

import json
import os
from typing import List

import requests


def chatCompletitions(messages: List[str]) -> None:
	api_key_file = os.environ.get("OPENAI_API_KEY_FILE")
	openai_api_key = open(api_key_file).read()
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
	print(resp.json()['choices'][0]['message']['content'])
	return resp.json()['choices'][0]['message']['content']


def main() -> None:
	chatCompletitions([
		{"role": "user", "content": "Say this is a test!"},
	])


if __name__ == "__main__":
	main()
