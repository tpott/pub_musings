import json
import requests
import time
from typing import Dict, List, Any, Optional

from werewolf.ai import AIClient


class OpenAIClient(AIClient):
    """A client for communicating with the OpenAI API via direct HTTP requests."""

    API_URL = "https://api.openai.com/v1/chat/completions"

    def __init__(self, api_key: str, model: str = "gpt-4o"):
        """Initialize the OpenAI client.

        Args:
            api_key: The OpenAI API key
            model: The model to use (default: gpt-4o)
        """
        self.api_key = api_key
        self.model = model
        self.headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}",
        }

    def complete(
        self,
        messages: List[Dict[str, str]],
        temperature: float = 0.7,
        max_tokens: int = 500,
    ) -> Optional[str]:
        """Send a completion request to the OpenAI API.

        Args:
            messages: List of message dictionaries with 'role' and 'content' keys
            temperature: Sampling temperature (0-1)
            max_tokens: Maximum number of tokens to generate

        Returns:
            The generated text, or None if an error occurred
        """
        payload = {
            "model": self.model,
            "messages": messages,
            "temperature": temperature,
            "max_tokens": max_tokens,
        }

        # Retry mechanism for API requests
        max_retries = 3
        retry_delay = 1  # Initial delay in seconds

        for attempt in range(max_retries):
            try:
                response = requests.post(
                    self.API_URL, headers=self.headers, json=payload, timeout=30
                )

                response.raise_for_status()
                result = response.json()

                if "choices" in result and len(result["choices"]) > 0:
                    return result["choices"][0]["message"]["content"]
                return None

            except requests.exceptions.RequestException as e:
                print(
                    f"API request error (attempt {attempt+1}/{max_retries}): {str(e)}"
                )
                if attempt < max_retries - 1:  # Don't sleep on the last attempt
                    time.sleep(retry_delay)
                    retry_delay *= 2  # Exponential backoff

        print("Maximum retry attempts reached. API request failed.")
        return None
