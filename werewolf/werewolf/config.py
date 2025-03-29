from dataclasses import dataclass
from typing import Optional

@dataclass
class GameConfig:
    """Configuration for a Werewolf game."""
    
    # Game parameters
    total_players: int = 6
    num_werewolves: int = 1
    num_seers: int = 1
    
    # API parameters
    openai_api_key: Optional[str] = None
    model_name: str = "gpt-4o"
    
    # Other settings
    verbose: bool = False
    max_turns: int = 20  # Max number of day/night cycles before force ending
    
    @property
    def num_villagers(self) -> int:
        """Calculate the number of regular villagers based on other role counts."""
        return self.total_players - self.num_werewolves - self.num_seers
    
    def validate(self) -> bool:
        """Validate the configuration."""
        # Check if the player distribution is valid
        if self.num_villagers < 0:
            return False
        
        # Ensure there's at least one werewolf
        if self.num_werewolves < 1:
            return False
        
        # Ensure there are enough players
        if self.total_players < 3:
            return False
            
        return True
