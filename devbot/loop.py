# loop.py
# Thu Dec 11 06:57:20 PST 2025

import argparse
import time
from typing import Any, Dict, Optional

import asyncio

from chatgpt import chat_completions
from serve_config import get_config


class ConversationManager:
    def __init__(self):
        self.history = []
        self.context = {}
    
    def add_message(self, role: str, content: str):
        self.history.append({"role": role, "content": content, "timestamp": time.time()})
    
    async def process_input(self, user_input: str):
        self.add_message("user", user_input)
        # Process with full history available
        # response = await your_ai_agent.process(self.history, self.context)
        response = await chat_completions(self.history, self.context)
        self.add_message("assistant", response["content"])
        return response["content"]


def get_system_prompt() -> str:
    return "You are a helpful developer assistant" # TODO template-ize prompt


def parse_args():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(
        description="ChatGPT devbot"
    )
    parser.add_argument(
        "-m",
        "--model",
        default="gpt-5",
        help="Specify the OpenAI model to use (e.g., gpt-5, gpt-4o, gpt-4.1, gpt-4o-mini)",
    )
    parser.add_argument(
        "-v", 
        "--verbose", 
        action="count", 
        default=0,
        help="Increase output verbosity (can be specified multiple times for more detail)"
    )
    return parser.parse_args()


async def main():
    """Loop to continuously prompt the user, query ChatGPT, and process responses."""
    args = parse_args()
    manager = ConversationManager()
    if args.model is not None:
        manager.context["model"] = args.model
    manager.context["verbose"] = args.verbose

    _ = get_config() # assert that we can load SERVE_CONFIG
    print(f"ChatGPT Interactive using model: {args.model} (type 'exit' to quit)\n")

    manager.add_message("system", get_system_prompt())

    while True:
        # Get user input
        try:
            user_input = input("> ")
        except EOFError:
            print("Exiting")
            break

        if user_input == "":
            continue

        # Exit condition
        if user_input.lower() in ["exit", "quit"]:
            print("Goodbye!")
            break

        res = await manager.process_input(user_input)
        print(res)


if __name__ == "__main__":
    asyncio.run(main())
