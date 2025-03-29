#!/usr/bin/env python3
"""
Werewolf Game with AI Players

A command-line implementation of the Werewolf social deduction game
where all players are AI agents using the OpenAI API directly via requests.
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
    parser.add_argument("--players", type=int, default=6, help="Number of players (default: 6)")
    parser.add_argument("--werewolves", type=int, default=1, help="Number of werewolves (default: 1)")
    parser.add_argument("--seers", type=int, default=1, help="Number of seers (default: 1)")
    parser.add_argument("--api-key-file", type=str, help="File containing OpenAI API key")
    parser.add_argument("--model", type=str, default="gpt-4o", help="OpenAI model to use (default: gpt-4o)")
    parser.add_argument("--verbose", "-v", action="store_true", help="Enable verbose output")
    args = parser.parse_args()
    
    # Check if API key file is valid
    api_key = None
    if args.api_key_file:
        if check_api_key_file(args.api_key_file):
            with open(args.api_key_file, 'r') as f:
                api_key = f.read().strip()
        else:
            sys.exit(1)
    
    # Create game configuration
    config = GameConfig(
        total_players=args.players,
        num_werewolves=args.werewolves,
        num_seers=args.seers,
        openai_api_key=api_key,
        model_name=args.model,
        verbose=args.verbose
    )
    
    # Create and run game
    game = Game(config)
    game.run()

if __name__ == "__main__":
    main()
