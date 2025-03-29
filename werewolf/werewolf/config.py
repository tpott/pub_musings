from dataclasses import dataclass
from typing import Optional, Literal

@dataclass
class GameConfig:
    """Configuration for a Werewolf game."""
    
    # Game parameters
    total_players: int = 7
    num_werewolves: int = 2
    num_seers: int = 1
    
    # API parameters
    openai_api_key: Optional[str] = None
    anthropic_api_key: Optional[str] = None
    model_name: str = "gpt-4o"  # Default for OpenAI
    api_type: Literal["openai", "anthropic"] = "openai"
    
    # Other settings
    verbose: bool = False
    log_to_file: bool = False  # Whether to log detailed prompts and responses to logs.txt
    max_turns: int = 20  # Max number of day/night cycles before force ending
    max_words: int = 100  # Maximum number of words per AI response
    
    @property
    def num_villagers(self) -> int:
        """Calculate the number of regular villagers based on other role counts."""
        return self.total_players - self.num_werewolves - self.num_seers
    
    @property
    def default_model_name(self) -> str:
        """Return the default model name based on the API type."""
        return "gpt-4o" if self.api_type == "openai" else "claude-3-7-sonnet-20250219"
    
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
        
        # Ensure we have one (and only one) API key set based on the api_type
        if self.api_type == "openai" and not self.openai_api_key:
            return False
        
        if self.api_type == "anthropic" and not self.anthropic_api_key:
            return False
            
        return True
