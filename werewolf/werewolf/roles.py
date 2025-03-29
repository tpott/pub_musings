from enum import Enum, auto
from typing import List, Dict, Any, Optional, Set

class Role(Enum):
    """Enumeration of possible player roles."""
    VILLAGER = auto()
    WEREWOLF = auto()
    SEER = auto()
    
    def __str__(self) -> str:
        return self.name.capitalize()

class Player:
    """Represents a player in the game."""
    
    def __init__(self, name: str, role: Role):
        """Initialize a player.
        
        Args:
            name: The player's name
            role: The player's role
        """
        self.name = name
        self.role = role
        self.alive = True
        self.memory: List[str] = []  # Memory of game events
        
    def add_memory(self, event: str) -> None:
        """Add an event to the player's memory.
        
        Args:
            event: Description of the event
        """
        self.memory.append(event)
        
    def get_memory(self) -> str:
        """Get the player's full memory as a string."""
        return "\n".join(self.memory)
    
    def generate_prompt_context(self) -> str:
        """Generate the basic context for this player's prompts."""
        context = f"You are {self.name}, a {self.role} in a game of Werewolf.\n"
        
        if self.role == Role.VILLAGER:
            context += (
                "As a Villager, your goal is to identify and eliminate the Werewolves hiding among you.\n"
                "During the day, participate in discussions and vote to eliminate suspicious players.\n"
                "You win when all Werewolves are eliminated.\n"
            )
        elif self.role == Role.WEREWOLF:
            context += (
                "As a Werewolf, your goal is to eliminate the Villagers without being discovered.\n"
                "During the night, you and other Werewolves choose a Villager to eliminate.\n"
                "During the day, blend in and avoid suspicion.\n"
                "You win when the number of Werewolves equals or exceeds the number of Villagers.\n"
            )
        elif self.role == Role.SEER:
            context += (
                "As a Seer, your goal is to help identify and eliminate the Werewolves.\n"
                "During the night, you can check one player to determine if they are a Werewolf.\n"
                "During the day, use this information carefully to help the village without making yourself a target.\n"
                "You win when all Werewolves are eliminated.\n"
            )
            
        return context
