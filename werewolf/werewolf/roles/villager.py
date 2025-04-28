from werewolf.roles.base import Player
from werewolf.roles.role_enum import Role


class VillagerPlayer(Player):
    """A player with the Villager role."""

    def __init__(self, name: str):
        """Initialize a villager player.

        Args:
            name: The player's name
        """
        super().__init__(name, Role.VILLAGER)
