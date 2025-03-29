import json
import requests
import time
from typing import Dict, List, Any, Optional

from werewolf.ai import AIClient

class AnthropicClient(AIClient):
    """A client for communicating with the Anthropic API via direct HTTP requests."""
    
    API_URL = "https://api.anthropic.com/v1/messages"
    
    def __init__(self, api_key: str, model: str = "claude-3-7-sonnet-20250219"):
        """Initialize the Anthropic client.
        
        Args:
            api_key: The Anthropic API key
            model: The model to use (default: claude-3-7-sonnet-20250219)
        """
        self.api_key = api_key
        self.model = model
        self.headers = {
            "Content-Type": "application/json",
            "x-api-key": api_key,
            "anthropic-version": "2023-06-01"
        }
    
    def complete(self, messages: List[Dict[str, str]], temperature: float = 0.7, 
                max_tokens: int = 500) -> Optional[str]:
        """Send a completion request to the Anthropic API.
        
        Args:
            messages: List of message dictionaries with 'role' and 'content' keys
            temperature: Sampling temperature (0-1)
            max_tokens: Maximum number of tokens to generate
            
        Returns:
            The generated text, or None if an error occurred
        """
        # Format messages for Anthropic API
        formatted_messages = []
        system_content = None
        
        for msg in messages:
            if msg["role"] == "system":
                system_content = msg["content"]
            else:
                # Convert 'user' to 'user' and 'assistant' to 'assistant'
                role = "user" if msg["role"] == "user" else "assistant"
                formatted_messages.append({"role": role, "content": msg["content"]})
        
        payload = {
            "model": self.model,
            "messages": formatted_messages,
            "temperature": temperature,
            "max_tokens": max_tokens,
        }
        
        # Add system content if provided
        if system_content:
            payload["system"] = system_content
        
        # Retry mechanism for API requests
        max_retries = 3
        retry_delay = 1  # Initial delay in seconds
        
        for attempt in range(max_retries):
            try:
                response = requests.post(
                    self.API_URL,
                    headers=self.headers,
                    json=payload,
                    timeout=30
                )
                
                response.raise_for_status()
                result = response.json()
                
                if "content" in result and len(result["content"]) > 0:
                    # Get the text from the content (first block of type 'text')
                    for block in result["content"]:
                        if block["type"] == "text":
                            return block["text"]
                return None
                
            except requests.exceptions.RequestException as e:
                print(f"API request error (attempt {attempt+1}/{max_retries}): {str(e)}")
                if attempt < max_retries - 1:  # Don't sleep on the last attempt
                    time.sleep(retry_delay)
                    retry_delay *= 2  # Exponential backoff
        
        print("Maximum retry attempts reached. API request failed.")
        return None