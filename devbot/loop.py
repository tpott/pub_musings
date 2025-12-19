# loop.py
# Thu Dec 11 06:57:20 PST 2025

import argparse
import asyncio
import time
import traceback
from typing import Optional

from dev_agent import DevAgent
from serve_config import get_config


class ConversationManager:
    def __init__(self):
        self.history = []  # Local history (mirrors what DevAgent tracks)
        self.context = {}
        self._agent: Optional[DevAgent] = None

    def _get_agent(self) -> DevAgent:
        """Lazy initialization of DevAgent."""
        if self._agent is None:
            config = get_config()
            config["verbose"] = self.context.get("verbose", 0)
            config["model"] = self.context.get("model")
            self._agent = DevAgent(config)
        return self._agent

    def add_message(self, role: str, content: str):
        self.history.append(
            {"role": role, "content": content, "timestamp": time.time()}
        )

    async def process_input(self, user_input: str) -> str:
        """All input goes through DevAgent."""
        self.add_message("user", user_input)

        agent = self._get_agent()
        response = await agent.process_input(user_input)

        self.add_message("assistant", response)
        return response


def parse_args():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(
        description="DevAgent - Claude-powered developer assistant"
    )
    parser.add_argument(
        "-m",
        "--model",
        default="claude-sonnet-4-5-20250929",
        help="Specify the Anthropic model to use (e.g., claude-sonnet-4-5-20250929, claude-3-5-haiku-20241022)",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="count",
        default=0,
        help="Increase output verbosity (can be specified multiple times for more detail)",
    )
    parser.add_argument(
        "-p",
        "--prompt",
        type=str,
        help="Execute a single prompt and exit (instead of interactive mode)",
    )
    return parser.parse_args()


async def main():
    """Loop to continuously prompt the user, query DevAgent, and process responses."""
    args = parse_args()
    manager = ConversationManager()

    if args.model is not None:
        manager.context["model"] = args.model
    manager.context["verbose"] = args.verbose

    _ = get_config()  # Assert that we can load config

    # Single prompt mode: execute and exit
    if args.prompt:
        try:
            res = await manager.process_input(args.prompt)
            print(res)
        except Exception as e:
            print(f"Error: {e}")
            if args.verbose > 0:
                traceback.print_exc()
        return

    # Interactive mode
    print(
        f"DevAgent Interactive using model: {args.model} (type 'exit' or '/help' for commands)\n"
    )

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

        try:
            res = await manager.process_input(user_input)
            print(res)
        except Exception as e:
            print(f"Error: {e}")
            if args.verbose > 0:
                traceback.print_exc()


if __name__ == "__main__":
    asyncio.run(main())
