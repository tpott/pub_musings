from abc import ABC, abstractmethod
from typing import Dict, List, Optional, Any


class AIClient(ABC):
    """Abstract base class for AI API clients."""

    @abstractmethod
    def __init__(self, api_key: str, model: str):
        """Initialize the AI client.

        Args:
            api_key: The API key
            model: The model to use
        """
        pass

    @abstractmethod
    def complete(
        self,
        messages: List[Dict[str, str]],
        temperature: float = 0.7,
        max_tokens: int = 500,
    ) -> Optional[str]:
        """Send a completion request to the AI API.

        Args:
            messages: List of message dictionaries with 'role' and 'content' keys
            temperature: Sampling temperature (0-1)
            max_tokens: Maximum number of tokens to generate

        Returns:
            The generated text, or None if an error occurred
        """
        pass
