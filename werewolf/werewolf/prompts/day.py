from typing import List, Dict, Any, Optional
from werewolf.roles import Player, Role
from werewolf.state import GameState, GamePhase
from werewolf.prompts.player import create_player_context


def create_day_discussion_prompt(player: Player, state: GameState) -> str:
    """Create a prompt for a player to discuss during the day.

    Args:
        player: The player to generate the prompt for
        state: The current game state

    Returns:
        A prompt for the player to discuss during the day
    """
    context = create_player_context(player, state)

    # Add recent discussion history
    village_channel = state.channels["village"]
    recent_messages = village_channel.format_recent_history(10)

    prompt = f"{context}\n\n"
    prompt += "It is day time in the village. The players are discussing who might be a Werewolf.\n"

    if recent_messages:
        prompt += f"Recent messages in the village:\n{recent_messages}\n\n"

    prompt += """Please respond with a JSON object containing your next action. Valid action types during the day are:
    - SPEAK: Share your thoughts with the entire village
    - WHISPER: Privately communicate with a player next to you
    - OBSERVE: Just watch and listen without saying anything
    
    Example response formats:
    {"action_type": "SPEAK", "message": "I think we should suspect Logan because..."}
    {"action_type": "WHISPER", "target": "Alex", "message": "I think the werewolves are..."}
    {"action_type": "OBSERVE"}
    
    Choose the action that makes most sense for your character at this moment."""

    # Add more specific guidance based on role
    if player.role == Role.WEREWOLF:
        prompt += "\nRemember, you are a Werewolf pretending to be a Villager. Don't reveal your true identity."
    elif player.role == Role.SEER:
        prompt += "\nAs a Seer, you have special knowledge, but be careful how you use it to avoid becoming a target."

    return prompt


def create_day_voting_prompt(player: Player, state: GameState) -> str:
    """Create a prompt for a player to vote during the day.

    Args:
        player: The player to generate the prompt for
        state: The current game state

    Returns:
        A prompt for the player to vote during the day
    """
    context = create_player_context(player, state)

    # Add recent discussion history
    village_channel = state.channels["village"]
    recent_messages = village_channel.format_recent_history(10)

    prompt = f"{context}\n\n"
    prompt += "It is time to vote on who to eliminate from the village.\n"

    if recent_messages:
        prompt += f"Recent messages in the village:\n{recent_messages}\n\n"

    # List living players except self
    living_players = [p for p in state.get_living_players() if p.name != player.name]
    player_list = ", ".join(p.name for p in living_players)

    prompt += f"The living players besides yourself are: {player_list}\n\n"
    prompt += """Please respond with a JSON object containing your vote.
    
    Example response format:
    {"action_type": "VOTE", "target": "Alex"}
    
    You must vote for one of the living players."""

    return prompt


def create_day_reaction_prompt(player: Player, state: GameState) -> str:
    """Generate prompt for player to react to recent events.

    Args:
        player: The player to generate the prompt for
        state: The current game state

    Returns:
        A prompt for the player to react to recent events
    """
    context = create_player_context(player, state)

    # Add very recent discussion history (last 3-5 messages)
    village_channel = state.channels["village"]
    recent_messages = village_channel.format_recent_history(5)

    prompt = f"{context}\n\n"
    prompt += (
        "Something has just happened in the village that you may want to react to.\n"
    )

    if recent_messages:
        prompt += f"Recent events:\n{recent_messages}\n\n"

    prompt += """Please respond with a JSON object containing your reaction.
    
    Example response formats:
    {"action_type": "SPEAK", "message": "I don't believe what Logan just said because..."}
    {"action_type": "WHISPER", "target": "Alex", "message": "Did you notice what Bailey just said?"}
    {"action_type": "OBSERVE"}
    
    Choose the action that makes most sense for your character based on these recent events."""

    # Add role-specific guidance
    if player.role == Role.WEREWOLF:
        prompt += "\nAs a werewolf, be careful not to reveal your true identity while reacting."
    elif player.role == Role.SEER:
        prompt += "\nAs a seer, consider how to use your knowledge without making yourself a target."

    return prompt


def create_neighbor_whisper_prompt(
    player: Player, state: GameState, target: Player
) -> str:
    """Generate prompt for player to whisper to a neighbor.

    Args:
        player: The player to generate the prompt for
        state: The current game state
        target: The target player to whisper to

    Returns:
        A prompt for the player to whisper to a neighbor
    """
    context = create_player_context(player, state)

    # Get whisper history if it exists
    whisper_key = (
        f"whisper_{min(player.name, target.name)}_{max(player.name, target.name)}"
    )
    whisper_history = ""

    if whisper_key in state.channels:
        whisper_channel = state.channels[whisper_key]
        whisper_history = whisper_channel.format_recent_history(10)

    prompt = f"{context}\n\n"
    prompt += f"You have a chance to whisper to {target.name}, who is sitting next to you in the village circle.\n"
    prompt += "Whispers are private conversations that only the two of you can hear.\n"

    if whisper_history:
        prompt += (
            f"Previous whispers between you and {target.name}:\n{whisper_history}\n\n"
        )

    prompt += """Please respond with a JSON object containing your whisper message.
    
    Example response format:
    {"action_type": "WHISPER", "target": "TARGET_NAME", "message": "I think the werewolves might be..."}
    
    What would you like to whisper to your neighbor? Keep it brief and relevant to the game."""

    # Add role-specific guidance for whispering
    if player.role == Role.WEREWOLF:
        if target.role == Role.WEREWOLF:
            prompt += "\nYou are both werewolves, so you can talk strategy privately, but be careful that other players don't notice your coordination."
        else:
            prompt += "\nYou are a werewolf talking to a villager or seer. Be very careful not to reveal your true identity."

    return prompt