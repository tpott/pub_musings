from enum import Enum, auto


class Role(Enum):
    """Enumeration of possible player roles."""

    VILLAGER = auto()
    WEREWOLF = auto()
    SEER = auto()

    def __str__(self) -> str:
        return self.name.capitalize()
