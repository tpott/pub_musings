from typing import Optional
from werewolf.roles import Player, Role
from werewolf.state import GameState


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