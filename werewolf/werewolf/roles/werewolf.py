from werewolf.roles.base import Player
from werewolf.roles.role_enum import Role


class WerewolfPlayer(Player):
    """A player with the Werewolf role."""

    def __init__(self, name: str):
        """Initialize a werewolf player.

        Args:
            name: The player's name
        """
        super().__init__(name, Role.WEREWOLF)
