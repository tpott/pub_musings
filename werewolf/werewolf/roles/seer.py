from werewolf.roles.base import Player
from werewolf.roles.role_enum import Role


class SeerPlayer(Player):
    """A player with the Seer role."""

    def __init__(self, name: str):
        """Initialize a seer player.

        Args:
            name: The player's name
        """
        super().__init__(name, Role.SEER)
