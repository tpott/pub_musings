import time
import random
from typing import Dict, List, Optional, Set, Tuple

from werewolf.config import GameConfig
from werewolf.state import GameState, GamePhase
from werewolf.roles import Player, Role
from werewolf.ai import AIClient
from werewolf.openai import OpenAIClient
from werewolf.anthropic import AnthropicClient
from werewolf.prompts import *

class Game:
    """Main class for running a Werewolf game with AI players."""
    
    def __init__(self, config: GameConfig):
        """Initialize a new game.
        
        Args:
            config: The game configuration
        """
        # Validate configuration
        if not config.validate():
            raise ValueError("Invalid game configuration")
        
        self.config = config
        self.state = GameState(config)
        
        # Initialize the AI client based on configured API type
        self.ai = None
        
        if config.api_type == "openai" and config.openai_api_key:
            self.ai = OpenAIClient(config.openai_api_key, config.model_name)
        elif config.api_type == "anthropic" and config.anthropic_api_key:
            # Use default Anthropic model if model_name is still the OpenAI default
            model = config.model_name if config.model_name != "gpt-4o" else config.default_model_name
            self.ai = AnthropicClient(config.anthropic_api_key, model)
            
        # Initialize log file if logging is enabled
        if config.log_to_file:
            self._initialize_log_file()
    
    def run(self) -> None:
        """Run the game until completion."""
        if not self.ai:
            print(f"Error: No valid API key provided for {self.config.api_type} API. Cannot start game.")
            return
        
        print("\n=== Werewolf Game with AI Players ===\n")
        print(f"Players: {self.config.total_players}, "
              f"Werewolves: {self.config.num_werewolves}, "
              f"Seers: {self.config.num_seers}, "
              f"Villagers: {self.config.num_villagers}\n")
        
        # Print player roles (for debugging)
        if self.config.verbose:
            print("=== Player Roles ===")
            for player in self.state.players:
                print(f"{player.name}: {player.role}")
            print()
        
        # Run the game loop
        self._game_loop()
    
    def _game_loop(self) -> None:
        """Main game loop that runs until the game is over."""
        # Start with setup phase
        self._run_setup_phase()
        
        # Continue until game is over
        while not self.state.game_over:
            if self.state.phase == GamePhase.DAY_DISCUSSION:
                self._run_day_discussion_phase()
            elif self.state.phase == GamePhase.DAY_VOTING:
                self._run_day_voting_phase()
            elif self.state.phase == GamePhase.NIGHT:
                self._run_night_phase()
        
        # Print game over message
        self._print_game_summary()
    
    def _run_setup_phase(self) -> None:
        """Run the setup phase of the game."""
        print("=== Game Start ===\n")
        setup_message = (
            "Welcome to the village. A place that was once peaceful, but now threatened by werewolves " 
            "hiding among the population. The villagers must find and eliminate the werewolves before " 
            "they're outnumbered, while the werewolves must remain hidden and eliminate the villagers one by one."
        )
        print(f"Narrator: {setup_message}\n")
        
        # Add to village channel
        self.state.channels["village"].add_narrator_message(setup_message)
        
        # Advance to day discussion
        self.state.advance_phase()
    
    def _run_day_discussion_phase(self) -> None:
        """Run the day discussion phase."""
        day_num = self.state.turn + 1
        print(f"\n=== Day {day_num}: Discussion Phase ===\n")
        
        # Get the last eliminated player in night phase (if any)
        eliminated_player = None
        night_actions = self.state.night_actions
        if night_actions and "werewolf_kill" in night_actions:
            victim_name = night_actions["werewolf_kill"]
            eliminated_player = self.state.get_player_by_name(victim_name)
        
        # Narrator starts the day
        day_start_message = create_narrator_day_start_message(self.state, eliminated_player)
        print(f"Narrator: {day_start_message}\n")
        self.state.channels["village"].add_narrator_message(day_start_message)
        
        # Living players discuss (several rounds of discussion)
        living_players = self.state.get_living_players()
        
        # Determine number of discussion rounds based on player count
        num_rounds = min(3, max(1, len(living_players) // 2))
        
        for round_num in range(num_rounds):
            # Randomize player order for each round
            random.shuffle(living_players)
            
            for player in living_players:
                # Generate player message
                prompt = create_day_discussion_prompt(player, self.state)
                message = self._get_ai_response(prompt)
                
                # Add message to village channel
                self.state.channels["village"].add_message(player, message)
                
                # Print message
                print(f"{player.name}: {message}\n")
                
                # Small delay to avoid API rate limits
                time.sleep(0.5)
        
        # Advance to voting phase
        self.state.advance_phase()
    
    def _run_day_voting_phase(self) -> None:
        """Run the day voting phase."""
        print("\n=== Day: Voting Phase ===\n")
        
        # Narrator announces voting
        voting_message = create_narrator_day_voting_message(self.state)
        print(f"Narrator: {voting_message}\n")
        self.state.channels["village"].add_narrator_message(voting_message)
        
        # Each living player votes
        living_players = self.state.get_living_players()
        for player in living_players:
            prompt = create_day_voting_prompt(player, self.state)
            vote = self._get_ai_response(prompt)
            
            # Clean up and validate the vote
            vote = vote.strip().split('\n')[0]  # Take only the first line
            vote = ''.join(c for c in vote if c.isalnum() or c.isspace())  # Remove punctuation
            
            # Find the closest matching player name
            target_player = self._find_closest_player_match(vote)
            if target_player and target_player.alive and target_player.name != player.name:
                self.state.votes[player.name] = target_player.name
                print(f"{player.name} votes for {target_player.name}")
            else:
                # If vote is invalid, randomly select a living player other than self
                potential_targets = [p.name for p in living_players if p.name != player.name]
                if potential_targets:
                    random_vote = random.choice(potential_targets)
                    self.state.votes[player.name] = random_vote
                    print(f"{player.name} casts an unclear vote, interpreted as a vote for {random_vote}")
            
            # Small delay to avoid API rate limits
            time.sleep(0.5)
        
        # Count votes and eliminate player if applicable
        eliminated_player, vote_counts = self.state.count_votes()
        
        # Print vote counts
        print("\nVote Results:")
        for player_name, count in vote_counts.items():
            print(f"{player_name}: {count} votes")
        
        # Narrator announces results
        if eliminated_player:
            print(f"\n{eliminated_player.name} has been eliminated!")
            self.state.eliminate_player(eliminated_player)
        else:
            print("\nNo one was eliminated due to a tie or lack of votes.")
        
        # Narrator's end of day message
        day_end_message = create_narrator_day_end_message(self.state, eliminated_player)
        print(f"\nNarrator: {day_end_message}\n")
        self.state.channels["village"].add_narrator_message(day_end_message)
        
        # Check if the game is over
        if self.state.game_over:
            self.state.phase = GamePhase.GAME_OVER
        else:
            # Advance to night phase
            self.state.advance_phase()
    
    def _run_night_phase(self) -> None:
        """Run the night phase of the game."""
        print("\n=== Night Phase ===\n")
        
        # Narrator starts the night
        night_start_message = create_narrator_night_start_message(self.state)
        print(f"Narrator: {night_start_message}\n")
        self.state.channels["village"].add_narrator_message(night_start_message)
        
        # Process werewolf actions
        living_werewolves = self.state.get_living_werewolves()
        if living_werewolves:
            self._process_werewolf_actions(living_werewolves)
        
        # Process seer actions
        living_seers = [p for p in self.state.get_living_players() if p.role == Role.SEER]
        for seer in living_seers:
            self._process_seer_action(seer)
        
        # Process the werewolf kill
        killed_player = self._process_werewolf_kill()
        if killed_player:
            # Mark the player as eliminated, but don't announce it until morning
            self.state.eliminate_player(killed_player)
        
        # Check if the game is over
        if self.state.game_over:
            self.state.phase = GamePhase.GAME_OVER
        else:
            # Advance to next day
            self.state.advance_phase()
    
    def _process_werewolf_actions(self, werewolves: List[Player]) -> None:
        """Process the werewolf discussion and killing.
        
        Args:
            werewolves: List of living werewolf players
        """
        # If multiple werewolves, they discuss first
        if len(werewolves) > 1:
            # Several rounds of discussion
            for round_num in range(min(2, len(werewolves))):
                # Randomize werewolf order for each round
                random.shuffle(werewolves)
                
                for werewolf in werewolves:
                    prompt = create_werewolf_night_prompt(werewolf, self.state)
                    message = self._get_ai_response(prompt)
                    
                    # Add message to werewolf channel
                    werewolf_channel = self.state.channels["werewolf"]
                    werewolf_channel.add_message(werewolf, message)
                    
                    if self.config.verbose:
                        print(f"[Werewolf Chat] {werewolf.name}: {message}")
                    
                    # Small delay to avoid API rate limits
                    time.sleep(0.5)
        
        # Each werewolf votes on who to kill
        werewolf_votes: Dict[str, int] = {}
        for werewolf in werewolves:
            prompt = create_werewolf_kill_prompt(werewolf, self.state)
            vote = self._get_ai_response(prompt)
            
            # Clean up and validate the vote
            vote = vote.strip().split('\n')[0]  # Take only the first line
            vote = ''.join(c for c in vote if c.isalnum() or c.isspace())  # Remove punctuation
            
            # Find the closest matching player name
            target_player = self._find_closest_player_match(vote)
            
            # Ensure the target is a living non-werewolf
            if target_player and target_player.alive and target_player.role != Role.WEREWOLF:
                werewolf_votes[target_player.name] = werewolf_votes.get(target_player.name, 0) + 1
                if self.config.verbose:
                    print(f"{werewolf.name} votes to kill {target_player.name}")
        
        # Determine the final target (most votes)
        if werewolf_votes:
            max_votes = max(werewolf_votes.values())
            candidates = [name for name, votes in werewolf_votes.items() if votes == max_votes]
            target = random.choice(candidates)
            self.state.night_actions["werewolf_kill"] = target
            
            if self.config.verbose:
                print(f"Werewolves choose to kill {target}")
    
    def _process_seer_action(self, seer: Player) -> None:
        """Process a seer's night action.
        
        Args:
            seer: The seer player
        """
        prompt = create_seer_night_prompt(seer, self.state)
        response = self._get_ai_response(prompt)
        
        # Clean up and validate the response
        response = response.strip().split('\n')[0]  # Take only the first line
        response = ''.join(c for c in response if c.isalnum() or c.isspace())  # Remove punctuation
        
        # Find the closest matching player name
        target_player = self._find_closest_player_match(response)
        
        if target_player and target_player.alive and target_player.name != seer.name:
            # Record the seer's action
            self.state.night_actions[f"seer_{seer.name}"] = target_player.name
            
            # Determine if the target is a werewolf and tell the seer
            is_werewolf = target_player.role == Role.WEREWOLF
            result = "a Werewolf" if is_werewolf else "not a Werewolf"
            
            # Add to seer's memory
            seer.add_memory(
                f"Night {self.state.turn + 1}: You investigated {target_player.name} " 
                f"and discovered they are {result}."
            )
            
            # Also add to seer's action channel
            action_channel = self.state.channels[f"action_{seer.name}"]
            action_channel.add_narrator_message(
                f"You focus your powers on {target_player.name} and receive a vision. " 
                f"You see that they are {result}."
            )
            
            if self.config.verbose:
                print(f"Seer {seer.name} investigates {target_player.name} and learns they are {result}")
    
    def _process_werewolf_kill(self) -> Optional[Player]:
        """Process the werewolf kill action.
        
        Returns:
            The player killed by werewolves, or None if no kill occurred
        """
        if "werewolf_kill" not in self.state.night_actions:
            return None
        
        victim_name = self.state.night_actions["werewolf_kill"]
        victim = self.state.get_player_by_name(victim_name)
        
        if victim and victim.alive:
            # Add to werewolf memory
            for werewolf in self.state.get_living_werewolves():
                werewolf.add_memory(
                    f"Night {self.state.turn + 1}: The werewolves killed {victim_name}."
                )
            
            return victim
        
        return None
    
    def _find_closest_player_match(self, name: str) -> Optional[Player]:
        """Find the player whose name most closely matches the given name.
        
        Args:
            name: The name to match
            
        Returns:
            The matching player, or None if no good match found
        """
        name = name.lower()
        
        # Try exact match first
        for player in self.state.players:
            if player.name.lower() == name:
                return player
        
        # Try contains match
        for player in self.state.players:
            if name in player.name.lower() or player.name.lower() in name:
                return player
        
        # If still no match, try to find the closest match
        for player in self.state.players:
            # Simple case-insensitive partial match
            if any(part in player.name.lower() for part in name.split()) or \
               any(part in name for part in player.name.lower().split()):
                return player
        
        return None
    
    def _get_ai_response(self, prompt: str) -> str:
        """Get a response from the AI.
        
        Args:
            prompt: The prompt to send to the AI
            
        Returns:
            The AI's response
        """
        # Add instruction to keep responses brief
        system_content = (
            "You are playing a role in a game of Werewolf. Respond in character as described in the prompt. "
            "Keep your responses brief (1-2 sentences maximum) and direct."
        )
        
        messages = [
            {"role": "system", "content": system_content},
            {"role": "user", "content": prompt}
        ]
        
        # Log the prompt if logging is enabled
        if self.config.log_to_file:
            self._log_to_file(f"PROMPT:\n{prompt}\n")
        
        # Get response from AI
        response = self.ai.complete(messages)
        
        if response:
            # Limit response length if needed (max ~100 words)
            words = response.split()
            if len(words) > 40:
                truncated_response = ' '.join(words[:40]) + "..."
            else:
                truncated_response = response
                
            # Log the full response if logging is enabled
            if self.config.log_to_file:
                self._log_to_file(f"RESPONSE:\n{response}\n")
                self._log_to_file("-" * 50 + "\n")
                
            return truncated_response
        else:
            # Fallback response if API call fails
            if self.config.log_to_file:
                self._log_to_file("API CALL FAILED\n")
                self._log_to_file("-" * 50 + "\n")
                
            return "I'm not sure what to say at this moment."
    
    def _print_game_summary(self) -> None:
        """Print a summary of the game results."""
        print("\n=== Game Over ===\n")
        
        # Narrator's game over message
        game_over_message = create_narrator_game_over_message(self.state)
        print(f"Narrator: {game_over_message}\n")
        self.state.channels["village"].add_narrator_message(game_over_message)
        
        # Print player roles
        print("Player Roles:")
        for player in self.state.players:
            status = "Alive" if player.alive else "Dead"
            print(f"{player.name}: {player.role} ({status})")
        
        print("\nThank you for playing Werewolf!")
        
    def _log_to_file(self, content: str) -> None:
        """Log content to the log file.
        
        Args:
            content: The content to log
        """
        try:
            with open("logs.txt", "a") as f:
                f.write(f"{time.strftime('%Y-%m-%d %H:%M:%S')} - {content}")
        except Exception as e:
            if self.config.verbose:
                print(f"Error writing to log file: {e}")
                
    def _initialize_log_file(self) -> None:
        """Initialize the log file with game configuration information."""
        if not self.config.log_to_file:
            return
            
        try:
            with open("logs.txt", "w") as f:
                f.write(f"=== Werewolf Game Log - {time.strftime('%Y-%m-%d %H:%M:%S')} ===\n")
                f.write(f"Players: {self.config.total_players}, ")
                f.write(f"Werewolves: {self.config.num_werewolves}, ")
                f.write(f"Seers: {self.config.num_seers}, ")
                f.write(f"Villagers: {self.config.num_villagers}\n")
                f.write(f"API Type: {self.config.api_type}\n")
                f.write(f"Model: {self.config.model_name}\n")
                f.write("=" * 50 + "\n\n")
        except Exception as e:
            if self.config.verbose:
                print(f"Error initializing log file: {e}")
