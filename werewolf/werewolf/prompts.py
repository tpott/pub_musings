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

    prompt = f"{context}\n\n"
    prompt += "It is day time in the village. The players are discussing who might be a Werewolf.\n"

    prompt += "What would you like to say to the village? Express your thoughts, suspicions, or defend yourself if needed."

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

    prompt = f"{context}\n\n"
    prompt += "It is time to vote on who to eliminate from the village.\n"

    # List living players except self
    living_players = [p for p in state.get_living_players() if p.name != player.name]
    player_list = ", ".join(p.name for p in living_players)

    prompt += f"The living players besides yourself are: {player_list}\n\n"
    prompt += (
        "Who do you vote to eliminate? Please respond with just the name of a player."
    )

    return prompt


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
        prompt += "Discuss with your fellow werewolves and agree on a victim.\n"
        prompt += "What would you like to say to the other werewolves?"
    else:
        prompt += "Who do you choose to eliminate? Please respond with just the name of a player."

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
    prompt += (
        "Who do you choose to eliminate? Please respond with just the name of a player."
    )

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
    prompt += "Who do you choose to investigate? Please respond with just the name of a player."

    return prompt


def create_narrator_day_start_message(
    state: GameState, eliminated_player: Optional[Player] = None
) -> str:
    """Create a narrator message for the start of a new day.

    Args:
        state: The current game state
        eliminated_player: The player eliminated during the night (if any)

    Returns:
        A narrator message for the start of a new day
    """
    day_num = state.turn + 1

    if day_num == 1:
        message = (
            "Dawn breaks over the sleepy village. The villagers gather in the town square, "
            "unaware that werewolves lurk among them. It's time to discuss your suspicions "
            "and try to identify the threats before night falls again."
        )
    else:
        if eliminated_player:
            message = (
                f"The village awakens to another day, but {eliminated_player.name} doesn't join the gathering. "
                f"Their body was found during the night, another victim of the werewolves. "
                f"With heavy hearts, the remaining villagers must continue their hunt for the beasts before more lives are lost."
            )
        else:
            message = (
                f"Day {day_num} begins in the village. The night passed peacefully with no attacks, "
                "but the werewolves still lurk among you. Time is running out to identify them."
            )

    return message


def create_narrator_day_voting_message(state: GameState) -> str:
    """Create a narrator message for the day voting phase.

    Args:
        state: The current game state

    Returns:
        A narrator message for the day voting phase
    """
    return (
        "The discussions have gone on long enough. It's time to vote on who to eliminate from the village. "
        "Each player must cast their vote. The player with the most votes will be eliminated."
    )


def create_narrator_day_end_message(
    state: GameState, eliminated_player: Optional[Player] = None
) -> str:
    """Create a narrator message for the end of the day.

    Args:
        state: The current game state
        eliminated_player: The player eliminated during the day (if any)

    Returns:
        A narrator message for the end of the day
    """
    if eliminated_player:
        if eliminated_player.role == Role.WEREWOLF:
            return (
                f"The votes are in. The village has decided to eliminate {eliminated_player.name}. "
                f"As they are dragged to the gallows, their body contorts and shifts, revealing their true werewolf form! "
                f"The village has successfully eliminated a werewolf, but are there more hiding among you?"
            )
        else:
            return (
                f"The votes are in. The village has decided to eliminate {eliminated_player.name}. "
                f"As they meet their fate, it becomes clear they were an innocent {eliminated_player.role}. "
                f"The real werewolves remain hidden, celebrating another successful misdirection."
            )
    else:
        return (
            "The village is deadlocked and cannot reach a decision on who to eliminate. "
            "No one will be eliminated today, but the werewolves still lurk among you. "
            "Night begins to fall..."
        )


def create_narrator_night_start_message(state: GameState) -> str:
    """Create a narrator message for the start of the night.

    Args:
        state: The current game state

    Returns:
        A narrator message for the start of the night
    """
    return (
        "Night falls over the village. The villagers return to their homes and lock their doors, "
        "but for some, the night's activities are just beginning. The werewolves prowl, seeking their next victim, "
        "while the Seer attempts to divine the truth about one of the villagers."
    )


def create_narrator_game_over_message(state: GameState) -> str:
    """Create a narrator message for the end of the game.

    Args:
        state: The current game state

    Returns:
        A narrator message for the end of the game
    """
    if state.winner == "villagers":
        return (
            "The last werewolf has been eliminated! The village is safe once more. "
            "The villagers celebrate their victory, though they mourn those lost in the struggle. "
            "\n\nVillagers win!"
        )
    elif state.winner == "werewolves":
        werewolves = [p.name for p in state.players if p.role == Role.WEREWOLF]
        return (
            "The werewolves now equal or outnumber the remaining villagers. With nothing to stop them, "
            "they reveal themselves and overrun the village in a night of terror and bloodshed. "
            f"\n\nWerewolves win! The werewolves were: {', '.join(werewolves)}"
        )
    else:  # stalemate
        werewolves = [p.name for p in state.players if p.role == Role.WEREWOLF]
        return (
            "After many days and nights, neither the villagers nor the werewolves have prevailed. "
            "The weary villagers leave, abandoning the cursed settlement to the shadows. "
            f"\n\nStalemate! The werewolves were: {', '.join(werewolves)}"
        )
