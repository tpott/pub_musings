from typing import List, Dict, Set, Optional
from werewolf.roles import Player

class Channel:
    """Represents a communication channel between players."""
    
    def __init__(self, name: str, description: str, participants: List[Player] = None):
        """Initialize a channel.
        
        Args:
            name: The name of the channel
            description: A description of the channel's purpose
            participants: The players who can participate in this channel
        """
        self.name = name
        self.description = description
        self.participants = participants or []
        self.messages: List[Dict[str, str]] = []
    
    def add_message(self, sender: Player, content: str) -> None:
        """Add a message to the channel.
        
        Args:
            sender: The player sending the message
            content: The message content
        """
        message = {
            "sender": sender.name,
            "content": content
        }
        self.messages.append(message)
        
        # Add this message to each participant's memory
        for participant in self.participants:
            if participant.alive:  # Only living players receive messages
                participant.add_memory(f"[{self.name}] {sender.name}: {content}")
    
    def add_narrator_message(self, content: str) -> None:
        """Add a message from the narrator.
        
        Args:
            content: The message content
        """
        message = {
            "sender": "Narrator",
            "content": content
        }
        self.messages.append(message)
        
        # Add this message to each participant's memory
        for participant in self.participants:
            if participant.alive:  # Only living players receive messages
                participant.add_memory(f"[{self.name}] Narrator: {content}")
    
    def get_recent_messages(self, count: int = 10) -> List[Dict[str, str]]:
        """Get the most recent messages in the channel.
        
        Args:
            count: The maximum number of messages to return
            
        Returns:
            A list of the most recent messages, up to count
        """
        return self.messages[-count:]
    
    def get_all_messages(self) -> List[Dict[str, str]]:
        """Get all messages in the channel.
        
        Returns:
            A list of all messages in the channel
        """
        return self.messages
    
    def format_recent_history(self, count: int = 10) -> str:
        """Format the recent message history as a string.
        
        Args:
            count: The maximum number of messages to include
            
        Returns:
            A formatted string of recent messages
        """
        recent = self.get_recent_messages(count)
        formatted = [f"{msg['sender']}: {msg['content']}" for msg in recent]
        return "\n".join(formatted)
