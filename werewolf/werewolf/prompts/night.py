from typing import List, Dict, Any, Optional
from werewolf.roles import Player, Role
from werewolf.state import GameState, GamePhase
from werewolf.prompts.player import create_player_context


def create_werewolf_night_prompt(player: Player, state: GameState) -> str:
    """Create a prompt for a werewolf to choose a victim during the night.

    Args:
        player: The werewolf player to generate the prompt for
        state: The current game state

    Returns:
        A prompt for the werewolf to choose a victim
    """
    context = create_player_context(player, state)

    # Add werewolf chat history if there are multiple werewolves
    werewolves = state.get_living_werewolves()
    if len(werewolves) > 1:
        werewolf_channel = state.channels["werewolf"]
        werewolf_chat = werewolf_channel.format_recent_history(10)  # Last 10 messages
        if werewolf_chat:
            context += f"\nRecent werewolf discussions:\n{werewolf_chat}\n"

    prompt = f"{context}\n\n"
    prompt += "It is night time. The Werewolves must choose a victim to eliminate.\n"

    # List potential victims (living non-werewolves)
    potential_victims = state.get_living_villagers()
    victim_list = ", ".join(p.name for p in potential_victims)

    prompt += f"Potential victims: {victim_list}\n\n"

    if len(werewolves) > 1:
        prompt += """Please respond with a JSON object containing your action.
        
        Example response format:
        {"action_type": "WEREWOLF_CHAT", "message": "I think we should kill Bailey because..."}
        
        You can discuss with your fellow werewolves about who to eliminate."""
    else:
        prompt += """Please respond with a JSON object containing your kill choice.
        
        Example response format:
        {"action_type": "KILL", "target": "Bailey"}
        
        You must choose one of the living non-werewolf players to eliminate."""

    return prompt


def create_werewolf_kill_prompt(player: Player, state: GameState) -> str:
    """Create a prompt for a werewolf to finalize their kill choice.

    Args:
        player: The werewolf player to generate the prompt for
        state: The current game state

    Returns:
        A prompt for the werewolf to finalize their kill choice
    """
    context = create_player_context(player, state)

    # Add werewolf chat history
    werewolf_channel = state.channels["werewolf"]
    werewolf_chat = werewolf_channel.format_recent_history(20)  # Last 20 messages

    prompt = f"{context}\n\n"
    prompt += "It is time to make the final decision on who to eliminate tonight.\n"

    if werewolf_chat:
        prompt += f"Werewolf discussion:\n{werewolf_chat}\n\n"

    # List potential victims (living non-werewolves)
    potential_victims = state.get_living_villagers()
    victim_list = ", ".join(p.name for p in potential_victims)

    prompt += f"Potential victims: {victim_list}\n\n"
    prompt += """Please respond with a JSON object containing your kill choice.
    
    Example response format:
    {"action_type": "KILL", "target": "Bailey"}
    
    You must choose one of the living non-werewolf players to eliminate."""

    return prompt


def create_seer_night_prompt(player: Player, state: GameState) -> str:
    """Create a prompt for a seer to choose a player to investigate during the night.

    Args:
        player: The seer player to generate the prompt for
        state: The current game state

    Returns:
        A prompt for the seer to choose a player to investigate
    """
    context = create_player_context(player, state)

    prompt = f"{context}\n\n"
    prompt += "It is night time. As the Seer, you can check one player to determine if they are a Werewolf.\n"

    # List potential players to investigate (all living players except self)
    potential_targets = [p for p in state.get_living_players() if p.name != player.name]
    target_list = ", ".join(p.name for p in potential_targets)

    prompt += f"Potential players to investigate: {target_list}\n\n"
    prompt += """Please respond with a JSON object containing your investigation choice.
    
    Example response format:
    {"action_type": "INVESTIGATE", "target": "Bailey"}
    
    You must choose one of the living players (except yourself) to investigate."""

    return prompt