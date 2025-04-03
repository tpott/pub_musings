from enum import Enum, auto
from typing import List, Dict, Set, Optional, Tuple
import random

from werewolf.roles import Player, Role
from werewolf.channel import Channel


class GamePhase(Enum):
    """Enumeration of game phases."""

    SETUP = auto()
    DAY_DISCUSSION = auto()
    DAY_VOTING = auto()
    NIGHT = auto()
    GAME_OVER = auto()

    def __str__(self) -> str:
        return self.name.replace("_", " ").title()


class GameState:
    """Manages the state of a Werewolf game."""

    def __init__(self, config):
        """Initialize the game state.

        Args:
            config: The game configuration
        """
        self.config = config
        self.turn = 0
        self.phase = GamePhase.SETUP
        self.players: List[Player] = []
        self.channels: Dict[str, Channel] = {}
        self.votes: Dict[str, str] = {}  # voter_name -> votee_name
        self.night_actions: Dict[str, str] = {}  # actor_name -> target_name
        self.game_over = False
        self.winner = None  # 'villagers' or 'werewolves'

        # Create the players
        self._create_players()

        # Create the channels
        self._create_channels()

    def _create_players(self) -> None:
        """Create the players for the game."""
        player_names = [
            "Alex",
            "Bailey",
            "Casey",
            "Dakota",
            "Ellis",
            "Finley",
            "Gray",
            "Harper",
            "Indigo",
            "Jordan",
            "Kendall",
            "Logan",
            "Morgan",
            "Nico",
            "Quinn",
            "Riley",
            "Skyler",
            "Taylor",
        ]

        # Shuffle the names to randomize player creation
        random.shuffle(player_names)

        # Assign roles
        roles = []
        roles.extend([Role.WEREWOLF] * self.config.num_werewolves)
        roles.extend([Role.SEER] * self.config.num_seers)
        roles.extend([Role.VILLAGER] * self.config.num_villagers)

        # Create the players
        for i in range(self.config.total_players):
            player = Player(player_names[i], roles[i])
            self.players.append(player)

        # Shuffle the player list to avoid role inference based on order
        random.shuffle(self.players)

    def _create_channels(self) -> None:
        """Create the communication channels for the game."""
        # Village channel (all players)
        village_channel = Channel(
            "Village",
            "The main village channel where all players can speak during the day.",
            self.players,
        )
        self.channels["village"] = village_channel

        # Werewolf channel (werewolves only)
        werewolves = [p for p in self.players if p.role == Role.WEREWOLF]
        werewolf_channel = Channel(
            "Werewolf Pack",
            "A private channel for werewolves to communicate during the night.",
            werewolves,
        )
        self.channels["werewolf"] = werewolf_channel

        # Individual role channels for night actions
        for player in self.players:
            if player.role in [Role.SEER, Role.WEREWOLF]:
                channel = Channel(
                    f"{player.name}'s Actions",
                    f"A private channel for {player.name} to perform night actions.",
                    [player],
                )
                self.channels[f"action_{player.name}"] = channel

    def get_living_players(self) -> List[Player]:
        """Get all living players.

        Returns:
            A list of all players who are still alive
        """
        return [p for p in self.players if p.alive]

    def get_player_by_name(self, name: str) -> Optional[Player]:
        """Get a player by their name.

        Args:
            name: The name of the player to get

        Returns:
            The player with the given name, or None if no such player exists
        """
        for player in self.players:
            if player.name == name:
                return player
        return None

    def get_living_werewolves(self) -> List[Player]:
        """Get all living werewolves.

        Returns:
            A list of all werewolves who are still alive
        """
        return [p for p in self.players if p.alive and p.role == Role.WEREWOLF]

    def get_living_villagers(self) -> List[Player]:
        """Get all living villagers (including seers).

        Returns:
            A list of all villagers and seers who are still alive
        """
        return [p for p in self.players if p.alive and p.role != Role.WEREWOLF]

    def eliminate_player(self, player: Player) -> None:
        """Eliminate a player from the game.

        Args:
            player: The player to eliminate
        """
        player.alive = False

        # Check if the game is over
        self.check_game_over()

    def check_game_over(self) -> bool:
        """Check if the game is over and set the winner.

        Returns:
            True if the game is over, False otherwise
        """
        werewolves = self.get_living_werewolves()
        villagers = self.get_living_villagers()

        # Game over if all werewolves are dead
        if not werewolves:
            self.game_over = True
            self.winner = "villagers"
            return True

        # Game over if werewolves equal or outnumber villagers
        if len(werewolves) >= len(villagers):
            self.game_over = True
            self.winner = "werewolves"
            return True

        # Game over if maximum turns reached
        if self.turn >= self.config.max_turns:
            self.game_over = True
            self.winner = "stalemate"
            return True

        return False

    def advance_phase(self) -> None:
        """Advance to the next game phase."""
        if self.phase == GamePhase.SETUP:
            self.phase = GamePhase.DAY_DISCUSSION
        elif self.phase == GamePhase.DAY_DISCUSSION:
            self.phase = GamePhase.DAY_VOTING
        elif self.phase == GamePhase.DAY_VOTING:
            self.phase = GamePhase.NIGHT
        elif self.phase == GamePhase.NIGHT:
            self.phase = GamePhase.DAY_DISCUSSION
            self.turn += 1  # Increment turn counter after a full day/night cycle

        # Reset votes and night actions when moving to a new phase
        if self.phase in [GamePhase.DAY_DISCUSSION, GamePhase.NIGHT]:
            self.votes = {}
        if self.phase == GamePhase.DAY_DISCUSSION:
            self.night_actions = {}

    def count_votes(self) -> Tuple[Optional[Player], Dict[str, int]]:
        """Count the votes and determine who gets eliminated.

        Returns:
            A tuple containing the player to eliminate (or None if tie)
            and a dictionary mapping player names to vote counts
        """
        if not self.votes:
            return None, {}

        # Count votes
        vote_counts: Dict[str, int] = {}
        for votee_name in self.votes.values():
            vote_counts[votee_name] = vote_counts.get(votee_name, 0) + 1

        # Find player with most votes
        max_votes = 0
        max_voted_players = []

        for player_name, count in vote_counts.items():
            if count > max_votes:
                max_votes = count
                max_voted_players = [player_name]
            elif count == max_votes:
                max_voted_players.append(player_name)

        # Handle tie (random selection)
        if len(max_voted_players) > 1:
            # Print verbose message about the tie if verbosity is enabled
            if hasattr(self.config, 'verbose') and self.config.verbose:
                tied_players = ", ".join(max_voted_players)
                print(f"There is a tie between {tied_players} with {max_votes} votes each.")
                print(f"Randomly selecting one player to eliminate...")
                
            selected = random.choice(max_voted_players)
            return self.get_player_by_name(selected), vote_counts
        elif max_voted_players:
            return self.get_player_by_name(max_voted_players[0]), vote_counts

        return None, vote_counts
