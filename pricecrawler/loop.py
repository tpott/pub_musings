# loop.py
# Mon Feb 24 20:44:17 PST 2025

import argparse
from pricechecker import priceSummaries


def parse_args():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(
        description="ChatGPT price checker interactive loop"
    )
    parser.add_argument(
        "-m",
        "--model",
        default="gpt-5",
        help="Specify the OpenAI model to use (e.g., gpt-5, gpt-4o, gpt-4.1, gpt-4o-mini)",
    )
    return parser.parse_args()


def main():
    """Loop to continuously prompt the user, query ChatGPT, and process responses."""
    args = parse_args()
    print(f"ChatGPT Interactive using model: {args.model} (type 'exit' to quit)\n")

    while True:
        # Get user input
        try:
            user_input = input("product? ")
        except EOFError:
            print("Exiting")
            break

        # Exit condition
        if user_input.lower() in ["exit", "quit"]:
            print("Goodbye!")
            break

        priceSumary = priceSummaries(user_input, model=args.model)


if __name__ == "__main__":
    main()
