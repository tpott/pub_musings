from typing import List, Dict, Any, Optional
from werewolf.roles import Player, Role
from werewolf.state import GameState, GamePhase


def create_player_context(player: Player, state: GameState) -> str:
    """Create the context for a player's prompts.

    Args:
        player: The player to create context for
        state: The current game state

    Returns:
        The context string for the player's prompts
    """
    # Basic role description
    context = player.generate_prompt_context()

    # Add current game state
    alive_players = state.get_living_players()
    context += f"\nIt is currently {state.phase}, turn {state.turn + 1}.\n"
    context += f"There are {len(alive_players)} players still alive: {', '.join(p.name for p in alive_players)}\n"

    # Add role-specific information
    if player.role == Role.WEREWOLF:
        living_werewolves = state.get_living_werewolves()
        context += f"\nYou are one of {len(living_werewolves)} living werewolves.\n"
        if len(living_werewolves) > 1:
            others = [w.name for w in living_werewolves if w.name != player.name]
            context += f"Your fellow werewolves are: {', '.join(others)}\n"

    # Add player's memory
    if player.memory:
        context += "\nYour memories from the game so far:\n"
        context += player.get_memory()

    return context