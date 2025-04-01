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
    
    def _get_retry_time_from_header(self, response: requests.Response) -> float:
        """Extract retry time from response headers.
        
        Args:
            response: The HTTP response object
            
        Returns:
            The number of seconds to wait before retrying
        """
        retry_after = response.headers.get('retry-after')
        if retry_after is None:
            return 1.0
            
        try:
            wait_time = float(retry_after)
            print(f"Rate limited. Waiting for {wait_time} seconds as specified by retry-after header.")
            return wait_time
        except (ValueError, TypeError):
            return 1.0
    
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
            # Make the API request
            try:
                response = requests.post(
                    self.API_URL,
                    headers=self.headers,
                    json=payload,
                    timeout=30
                )
            except requests.exceptions.RequestException as e:
                print(f"Request error (attempt {attempt+1}/{max_retries}): {str(e)}")
                
                # Don't retry on the last attempt
                if attempt == max_retries - 1:
                    break
                    
                time.sleep(retry_delay)
                retry_delay *= 2
                continue
                
            # Check response status
            if response.status_code == 429:
                print(f"Rate limit exceeded (attempt {attempt+1}/{max_retries})")
                
                # Don't retry on the last attempt
                if attempt == max_retries - 1:
                    break
                    
                # Get retry time from headers
                wait_time = self._get_retry_time_from_header(response)
                time.sleep(wait_time)
                continue
                
            # Handle other error status codes
            if response.status_code != 200:
                print(f"HTTP error (attempt {attempt+1}/{max_retries}): Status {response.status_code}")
                
                # Don't retry on the last attempt
                if attempt == max_retries - 1:
                    break
                    
                time.sleep(retry_delay)
                retry_delay *= 2
                continue
            
            # Parse the JSON response
            try:
                result = response.json()
            except ValueError as e:
                print(f"JSON parsing error: {str(e)}")
                return None
                
            # Extract the text from the response
            if "content" not in result or not result["content"]:
                return None
                
            # Get the first text block
            for block in result["content"]:
                if block["type"] == "text":
                    return block["text"]
                    
            # No text blocks found
            return None
        
        print("Maximum retry attempts reached. API request failed.")
        return None