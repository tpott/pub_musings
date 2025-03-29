- Game Overview:
   - All players are AI agents that use the OpenAI API
   - Different roles (Villager, Werewolf, Seer, etc.) have different behaviors
   - The game progresses through day/night phases
   - AI agents can chat in village and role-specific channels
   - A narrator agent provides storyline and describes events

- Implementation Plan:
   1. Create the necessary prompt templates for:
      - Different player roles (Werewolf, Villager, Seer)
      - Chat message generation
      - Decision-making
      - Narrator event descriptions
   
   2. Implementation Details:
      - Use Python's object-oriented features to create a clean architecture
      - Store game state in memory (no database required)
      - Implement the full game loop with proper phase transitions
      - Support different game configurations (player count, role distribution)
      - Ensure OpenAI API integration is robust
      - Create a simple command-line interface for running games
   
   3. Specific Implementation Tasks:
      - Create the main class structure with proper object references
      - Implement the core game logic (phases, voting, elimination)
      - Create the AI agent framework with OpenAI API integration
      - Implement role-specific behaviors
      - Create the narrator functionality for storytelling
      - Build a CLI for running and configuring games
      - Add proper logging and game history tracking
   
   4. Code Structure Guidelines:
      - Keep the implementation modular and clean
      - Use typing hints for better code clarity
      - Document classes and methods thoroughly
      - Handle errors gracefully, especially around API calls
      - Make the code extensible for adding new roles or features
   
   5. Testing Requirements:
      - Create test scenarios with different player counts
      - Verify that all roles function correctly
      - Test the full game loop to ensure proper game resolution
      - Ensure the narrator provides appropriate narrative at each stage