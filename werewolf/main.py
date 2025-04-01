#!/usr/bin/env python3
"""
Werewolf Game with AI Players

A command-line implementation of the Werewolf social deduction game
where all players are AI agents using either the OpenAI or Anthropic API.
"""

import os
import stat
import sys
from werewolf.game import Game
from werewolf.config import GameConfig

def check_api_key_file(file_path):
    """
    Check if the API key file exists and has appropriate permissions.
    File should have 400 or 600 permissions (read-only for owner).
    """
    if not os.path.exists(file_path):
        print(f"Error: API key file '{file_path}' not found.")
        return False
        
    # Check file permissions
    file_mode = os.stat(file_path).st_mode
    if not (file_mode & stat.S_IRUSR and not file_mode & stat.S_IRWXG and not file_mode & stat.S_IRWXO):
        print(f"Error: API key file '{file_path}' has incorrect permissions.")
        print(f"Current: {oct(file_mode & 0o777)}, Expected: 400 or 600")
        print("Please run: chmod 600 <api-key-file>")
        return False
        
    return True

def main():
    """
    Main entry point for the Werewolf game.
    Parses command line arguments and starts a new game.
    """
    import argparse
    
    parser = argparse.ArgumentParser(description="Run a Werewolf game with AI players")
    parser.add_argument("--players", type=int, default=7, help="Number of players (default: 7)")
    parser.add_argument("--werewolves", type=int, default=2, help="Number of werewolves (default: 2)")
    parser.add_argument("--seers", type=int, default=1, help="Number of seers (default: 1)")
    
    # API options - only one should be provided
    api_group = parser.add_mutually_exclusive_group(required=True)
    api_group.add_argument("--openai-key-file", type=str, help="File containing OpenAI API key")
    api_group.add_argument("--anthropic-key-file", type=str, help="File containing Anthropic API key")
    
    parser.add_argument("--model", type=str, help="Model to use (defaults based on API choice)")
    parser.add_argument("--max-words", type=int, default=80, help="Maximum number of words per AI response (default: 80)")
    parser.add_argument("--verbose", "-v", action="store_true", help="Enable verbose output")
    parser.add_argument("--log-to-file", action="store_true", help="Log detailed prompts and responses to logs.txt")
    args = parser.parse_args()
    
    # Determine API type based on which key file was provided
    api_type = "openai" if args.openai_key_file else "anthropic"
    key_file = args.openai_key_file if api_type == "openai" else args.anthropic_key_file
    
    # Check if API key file is valid
    openai_api_key = None
    anthropic_api_key = None
    
    if check_api_key_file(key_file):
        with open(key_file, 'r') as f:
            api_key = f.read().strip()
            if api_type == "openai":
                openai_api_key = api_key
            else:
                anthropic_api_key = api_key
    else:
        sys.exit(1)
    
    # Create game configuration
    config = GameConfig(
        total_players=args.players,
        num_werewolves=args.werewolves,
        num_seers=args.seers,
        openai_api_key=openai_api_key,
        anthropic_api_key=anthropic_api_key,
        api_type=api_type,
        model_name=args.model if args.model else None,  # Will use default if None
        verbose=args.verbose,
        log_to_file=args.log_to_file,
        max_words=args.max_words
    )
    
    # Set default model if none provided
    if not config.model_name:
        config.model_name = config.default_model_name
    
    # Create and run game
    game = Game(config)
    game.run()

if __name__ == "__main__":
    main()
