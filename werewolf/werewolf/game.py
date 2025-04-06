import time
import random
import json
import heapq
from typing import Dict, List, Optional, Set, Tuple, Any

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
            model = (
                config.model_name
                if config.model_name != "gpt-4o"
                else config.default_model_name
            )
            self.ai = AnthropicClient(config.anthropic_api_key, model)

        # Initialize log file if logging is enabled
        if config.log_to_file:
            self._initialize_log_file()

    def run(self) -> None:
        """Run the game until completion."""
        if not self.ai:
            print(
                f"Error: No valid API key provided for {self.config.api_type} API. Cannot start game."
            )
            return

        print("\n=== Werewolf Game with AI Players ===\n")
        print(
            f"Players: {self.config.total_players}, "
            f"Werewolves: {self.config.num_werewolves}, "
            f"Seers: {self.config.num_seers}, "
            f"Villagers: {self.config.num_villagers}\n"
        )

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
        """Run the day discussion with dynamic actions and real-time simulation."""
        day_num = self.state.turn + 1
        print(f"\n=== Day {day_num}: Discussion Phase ===\n")

        # Get the last eliminated player in night phase (if any)
        eliminated_player = None
        night_actions = self.state.night_actions
        if night_actions and "werewolf_kill" in night_actions:
            victim_name = night_actions["werewolf_kill"]
            eliminated_player = self.state.get_player_by_name(victim_name)

        # Narrator starts the day
        day_start_message = create_narrator_day_start_message(
            self.state, eliminated_player
        )
        print(f"Narrator: {day_start_message}\n")
        self.state.channels["village"].add_narrator_message(day_start_message)

        # Get living players
        living_players = self.state.get_living_players()

        # Initialize simulation time variables
        start_time = time.time()  # Real start time
        current_sim_time = 0.0  # Simulated time in seconds
        day_duration_seconds = self.config.day_phase_duration_minutes * 60

        # Initialize action queue
        # Queue format: (execution_time, player, action)
        action_queue = []

        # Get initial actions from all players
        for player in living_players:
            # Generate action
            prompt = create_day_discussion_prompt(player, self.state)
            response = self._get_ai_response(prompt)

            # Parse the JSON action
            action_data = self._parse_json_action(response)

            # If parsing failed or action is invalid, default to OBSERVE
            if not action_data or not self._validate_action(
                action_data, player, GamePhase.DAY_DISCUSSION
            ):
                action_data = {"action_type": "OBSERVE"}

            # Schedule the action with a random delay between 5-20 seconds
            execution_time = random.uniform(5, 20)
            heapq.heappush(action_queue, (execution_time, player, action_data))

        # Process actions in time order until time expires
        while current_sim_time < day_duration_seconds and action_queue:
            # Get the next action
            execution_time, player, action = heapq.heappop(action_queue)

            # Update the simulation time
            current_sim_time = execution_time

            # Check if we've exceeded the day duration
            if current_sim_time >= day_duration_seconds:
                break

            # Execute the action and update simulation time
            current_sim_time = self._execute_player_action(
                player, action, current_sim_time
            )

            # Small real-time delay to avoid API rate limits and make the simulation feel more natural
            time.sleep(0.5)

            # Schedule next action for this player if they're still alive
            if player.alive:
                # Determine the next prompt based on what just happened
                prompt = create_day_reaction_prompt(player, self.state)
                response = self._get_ai_response(prompt)

                # Parse the JSON action
                action_data = self._parse_json_action(response)

                # If parsing failed or action is invalid, default to OBSERVE
                if not action_data or not self._validate_action(
                    action_data, player, GamePhase.DAY_DISCUSSION
                ):
                    action_data = {"action_type": "OBSERVE"}

                # Schedule the next action with a random delay between 20-40 seconds
                next_execution_time = current_sim_time + random.uniform(20, 40)
                heapq.heappush(action_queue, (next_execution_time, player, action_data))

        # Announce the end of the discussion phase
        end_message = "The sun begins to set. It's time for the village to vote on who to eliminate."
        print(f"\nNarrator: {end_message}\n")
        self.state.channels["village"].add_narrator_message(end_message)

        # Advance to voting phase
        self.state.advance_phase()

    def _run_day_voting_phase(self) -> None:
        """Run the day voting phase with time-based simulation."""
        day_num = self.state.turn + 1
        print(f"\n=== Day {day_num}: Voting Phase ===\n")

        # Narrator announces voting
        voting_message = create_narrator_day_voting_message(self.state)
        print(f"Narrator: {voting_message}\n")
        self.state.channels["village"].add_narrator_message(voting_message)

        # Initialize simulation time variables
        current_sim_time = 0.0  # Simulated time in seconds
        voting_duration_seconds = self.config.voting_phase_duration_seconds

        # Initialize action queue for voting
        action_queue = []

        # Each living player submits a vote with a random delay
        living_players = self.state.get_living_players()
        for player in living_players:
            prompt = create_day_voting_prompt(player, self.state)
            response = self._get_ai_response(prompt)

            # Parse the JSON action
            action_data = self._parse_json_action(response)

            # If parsing failed or action is invalid, create a random valid vote
            if not action_data or not self._validate_action(
                action_data, player, GamePhase.DAY_VOTING
            ):
                # Create a random vote for a living player other than self
                potential_targets = [p for p in living_players if p.name != player.name]
                if potential_targets:
                    random_target = random.choice(potential_targets)
                    action_data = {"action_type": "VOTE", "target": random_target.name}
                else:
                    # Edge case: if no valid targets (shouldn't happen), skip this player
                    continue

            # Schedule the vote with a random delay between 5-55 seconds
            execution_time = random.uniform(5, min(55, voting_duration_seconds - 5))
            heapq.heappush(action_queue, (execution_time, player, action_data))

        # Process votes in time order
        while current_sim_time < voting_duration_seconds and action_queue:
            # Get the next vote
            execution_time, player, action = heapq.heappop(action_queue)

            # Update the simulation time
            current_sim_time = execution_time

            # Check if we've exceeded the voting duration
            if current_sim_time >= voting_duration_seconds:
                break

            # Execute the vote and update simulation time
            current_sim_time = self._execute_player_action(
                player, action, current_sim_time
            )

            # Small real-time delay to avoid API rate limits
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
        """Run the night phase with time-based simulation."""
        day_num = self.state.turn + 1
        print(f"\n=== Day {day_num}: Night Phase ===\n")

        # Narrator starts the night
        night_start_message = create_narrator_night_start_message(self.state)
        print(f"Narrator: {night_start_message}\n")
        self.state.channels["village"].add_narrator_message(night_start_message)

        # Initialize simulation time variables
        current_sim_time = 0.0  # Simulated time in seconds
        night_duration_seconds = self.config.night_phase_duration_seconds

        # Initialize action queue
        action_queue = []

        # Get actions from all werewolves
        living_werewolves = self.state.get_living_werewolves()
        for werewolf in living_werewolves:
            prompt = create_werewolf_night_prompt(werewolf, self.state)
            response = self._get_ai_response(prompt)

            # Parse the JSON action
            action_data = self._parse_json_action(response)

            # If parsing failed or action is invalid, default to a reasonable action
            if not action_data or not self._validate_action(
                action_data, werewolf, GamePhase.NIGHT
            ):
                if len(living_werewolves) > 1:
                    # Default to werewolf chat
                    action_data = {
                        "action_type": "WEREWOLF_CHAT",
                        "message": "We need to decide who to eliminate tonight.",
                    }
                else:
                    # Default to a random kill target for lone werewolf
                    potential_victims = self.state.get_living_villagers()
                    if potential_victims:
                        random_victim = random.choice(potential_victims)
                        action_data = {
                            "action_type": "KILL",
                            "target": random_victim.name,
                        }
                    else:
                        # Edge case: no valid targets
                        action_data = {
                            "action_type": "WEREWOLF_CHAT",
                            "message": "No targets available.",
                        }

            # Schedule the action
            # Werewolf chat happens early, kill votes happen later
            if action_data.get("action_type", "").upper() == "WEREWOLF_CHAT":
                execution_time = random.uniform(5, 30)
            else:
                execution_time = random.uniform(60, night_duration_seconds - 30)

            heapq.heappush(action_queue, (execution_time, werewolf, action_data))

        # Get actions from all seers
        living_seers = [
            p for p in self.state.get_living_players() if p.role == Role.SEER
        ]
        for seer in living_seers:
            prompt = create_seer_night_prompt(seer, self.state)
            response = self._get_ai_response(prompt)

            # Parse the JSON action
            action_data = self._parse_json_action(response)

            # If parsing failed or action is invalid, create a random valid investigation
            if not action_data or not self._validate_action(
                action_data, seer, GamePhase.NIGHT
            ):
                # Choose random player to investigate
                potential_targets = [
                    p for p in self.state.get_living_players() if p != seer
                ]
                if potential_targets:
                    random_target = random.choice(potential_targets)
                    action_data = {
                        "action_type": "INVESTIGATE",
                        "target": random_target.name,
                    }
                else:
                    # Edge case: no valid targets
                    continue

            # Schedule the seer action in the middle of the night
            execution_time = random.uniform(30, 60)
            heapq.heappush(action_queue, (execution_time, seer, action_data))

        # Process actions in time order
        while (
            current_sim_time is None
            or current_sim_time < night_duration_seconds
            and action_queue
        ):
            # Get the next action
            execution_time, player, action = heapq.heappop(action_queue)

            # Update the simulation time
            current_sim_time = execution_time

            # Check if we've exceeded the night duration
            if current_sim_time >= night_duration_seconds:
                break

            # Execute the action and update simulation time
            current_sim_time = self._execute_player_action(
                player, action, current_sim_time
            )

            # For werewolf chat, potentially schedule another chat message
            if (
                player.role == Role.WEREWOLF
                and action.get("action_type", "").upper() == "WEREWOLF_CHAT"
            ):
                # Only schedule another chat if we're still in the first half of the night
                if (
                    current_sim_time is None
                    or current_sim_time < night_duration_seconds / 2
                ):
                    prompt = create_werewolf_night_prompt(player, self.state)
                    response = self._get_ai_response(prompt)

                    # Parse the JSON action
                    action_data = self._parse_json_action(response)

                    # Only schedule if it's another chat message or a kill action
                    if action_data and self._validate_action(
                        action_data, player, GamePhase.NIGHT
                    ):
                        # Schedule the next action with a reasonable delay
                        next_execution_time = current_sim_time + random.uniform(15, 30)
                        if next_execution_time < night_duration_seconds - 10:
                            heapq.heappush(
                                action_queue, (next_execution_time, player, action_data)
                            )

            # Small real-time delay to avoid API rate limits
            time.sleep(0.5)

        # Process the werewolf kill
        killed_player = self._process_werewolf_kill()
        if killed_player:
            # Mark the player as eliminated, but don't announce it until morning
            self.state.eliminate_player(killed_player)

        # Narrator ends the night
        end_message = "The long night comes to an end, and dawn approaches. The village begins to stir."
        print(f"\nNarrator: {end_message}\n")
        self.state.channels["village"].add_narrator_message(end_message)

        # Check if the game is over
        if self.state.game_over:
            self.state.phase = GamePhase.GAME_OVER
        else:
            # Advance to next day
            self.state.advance_phase()

    # These methods are maintained for backwards compatibility with tests
    # In the actual game flow, we now use _execute_player_action instead

    def _process_werewolf_actions(self, werewolves: List[Player]) -> None:
        """Process the werewolf discussion and killing.

        This method is kept for backward compatibility with tests.
        In actual gameplay, we use the new action system.

        Args:
            werewolves: List of living werewolf players
        """
        # Create a simple implementation for test compatibility
        sim_time = 0.0
        for werewolf in werewolves:
            action_data = {
                "action_type": "WEREWOLF_CHAT",
                "message": "Test werewolf chat",
            }
            sim_time = self._execute_player_action(werewolf, action_data, sim_time)

        # Add a random kill action
        if werewolves:
            potential_victims = self.state.get_living_villagers()
            if potential_victims:
                target = random.choice(potential_victims)
                self.state.night_actions["werewolf_kill"] = target.name

    def _process_seer_action(self, seer: Player) -> None:
        """Process a seer's night action.

        This method is kept for backward compatibility with tests.
        In actual gameplay, we use the new action system.

        Args:
            seer: The seer player
        """
        # Create a simple implementation for test compatibility
        potential_targets = [
            p for p in self.state.get_living_players() if p.name != seer.name
        ]
        if potential_targets:
            target = random.choice(potential_targets)
            self.state.night_actions[f"seer_{seer.name}"] = target.name

            # Add seer memory
            is_werewolf = target.role == Role.WEREWOLF
            result = "a Werewolf" if is_werewolf else "not a Werewolf"
            seer.add_memory(
                f"Night {self.state.turn + 1}: You investigated {target.name} and discovered they are {result}."
            )

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
        # If input is a JSON string, try to extract the target
        if name.strip().startswith("{") and name.strip().endswith("}"):
            try:
                data = json.loads(name)
                if "target" in data:
                    name = data["target"]
                elif "name" in data:
                    name = data["name"]
            except:
                pass  # Continue with original name if JSON parsing fails

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
            if any(part in player.name.lower() for part in name.split()) or any(
                part in name for part in player.name.lower().split()
            ):
                return player

        return None

    def _get_ai_response(self, prompt: str) -> str:
        """Get a response from the AI.

        Args:
            prompt: The prompt to send to the AI

        Returns:
            The AI's response
        """
        # Add instruction to keep responses varied in length
        system_content = (
            "You are playing a role in a game of Werewolf. Respond in character as described in the prompt. "
            f"Keep your responses naturally varied in length - sometimes brief (10-30 words), sometimes moderate (30-60 words), "
            f"and occasionally longer (60-{self.config.max_words} words) depending on the situation. "
            f"Never exceed {self.config.max_words} words. Use a length that feels most natural and appropriate "
            "for what your character would say in this specific moment."
        )

        messages = [
            {"role": "system", "content": system_content},
            {"role": "user", "content": prompt},
        ]

        # Log the prompt if logging is enabled
        if self.config.log_to_file:
            self._log_to_file(f"PROMPT:\n{prompt}\n")

        # Get response from AI
        response = self.ai.complete(messages)

        if response is not None:
            # Limit response length based on config
            words = response.split()
            if len(words) > self.config.max_words * 1.2:
                truncated_response = (
                    " ".join(words[: int(self.config.max_words * 1.2)]) + "..."
                )
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
                f.write(
                    f"=== Werewolf Game Log - {time.strftime('%Y-%m-%d %H:%M:%S')} ===\n"
                )
                f.write(f"Players: {self.config.total_players}, ")
                f.write(f"Werewolves: {self.config.num_werewolves}, ")
                f.write(f"Seers: {self.config.num_seers}, ")
                f.write(f"Villagers: {self.config.num_villagers}\n")
                f.write(f"API Type: {self.config.api_type}\n")
                f.write(f"Model: {self.config.model_name}\n")
                f.write(f"Max Words Per Response: {self.config.max_words}\n")
                f.write("=" * 50 + "\n\n")
        except Exception as e:
            if self.config.verbose:
                print(f"Error initializing log file: {e}")

    def get_player_to_left(self, player: Player) -> Player:
        """Return the player to the left of the given player in the circle.

        Args:
            player: The reference player

        Returns:
            The player to the left in the circle
        """
        living_players = self.state.get_living_players()
        if len(living_players) <= 1:
            return player

        # Find player's index in the list
        try:
            idx = living_players.index(player)
            # Return the player to the left (next in the list, wrapping around)
            return living_players[(idx + 1) % len(living_players)]
        except ValueError:
            # If player not found (shouldn't happen), return the first living player
            return living_players[0]

    def get_player_to_right(self, player: Player) -> Player:
        """Return the player to the right of the given player in the circle.

        Args:
            player: The reference player

        Returns:
            The player to the right in the circle
        """
        living_players = self.state.get_living_players()
        if len(living_players) <= 1:
            return player

        # Find player's index in the list
        try:
            idx = living_players.index(player)
            # Return the player to the right (previous in the list, wrapping around)
            return living_players[(idx - 1) % len(living_players)]
        except ValueError:
            # If player not found (shouldn't happen), return the first living player
            return living_players[0]

    def _parse_json_action(self, response_text: str) -> Dict[str, Any]:
        """Parse a JSON action from a text response.

        Args:
            response_text: The text response from the AI

        Returns:
            A dictionary containing the parsed action, or an empty dict if parsing failed
        """
        # Try to find JSON content inside the response
        json_start = response_text.find("{")
        json_end = response_text.rfind("}")

        if json_start >= 0 and json_end > json_start:
            # Extract the JSON content
            json_content = response_text[json_start : json_end + 1]
            try:
                action_data = json.loads(json_content)
                return action_data
            except json.JSONDecodeError:
                if self.config.verbose:
                    print(f"Failed to parse JSON action: {json_content}")
                return {}

        # If we couldn't find valid JSON, return an empty dict
        return {}

    def _validate_action(
        self, action_data: Dict[str, Any], player: Player, phase: GamePhase
    ) -> bool:
        """Validate a player action based on action type and game phase.

        Args:
            action_data: The action data to validate
            player: The player performing the action
            phase: The current game phase

        Returns:
            True if the action is valid, False otherwise
        """
        # Check for required fields
        if "action_type" not in action_data:
            return False

        action_type = action_data.get("action_type", "").upper()

        # Validate action types by phase
        if phase == GamePhase.DAY_DISCUSSION:
            valid_actions = ["SPEAK", "WHISPER", "OBSERVE"]
            if action_type not in valid_actions:
                return False

            # Validate whisper targets (must be neighbors)
            if action_type == "WHISPER":
                if "target" not in action_data:
                    return False

                target_name = action_data["target"]
                target_player = self.state.get_player_by_name(target_name)

                if not target_player or not target_player.alive:
                    return False

                # Check if target is a neighbor
                left_player = self.get_player_to_left(player)
                right_player = self.get_player_to_right(player)
                if target_player != left_player and target_player != right_player:
                    return False

        elif phase == GamePhase.DAY_VOTING:
            if action_type != "VOTE":
                return False

            # Validate vote target
            if "target" not in action_data:
                return False

            target_name = action_data["target"]
            target_player = self.state.get_player_by_name(target_name)

            if not target_player or not target_player.alive or target_player == player:
                return False

        elif phase == GamePhase.NIGHT:
            # Different valid actions depending on role
            if player.role == Role.WEREWOLF:
                valid_actions = ["WEREWOLF_CHAT", "KILL"]
                if action_type not in valid_actions:
                    return False

                # Validate kill target
                if action_type == "KILL" and "target" not in action_data:
                    return False

                if action_type == "KILL":
                    target_name = action_data["target"]
                    target_player = self.state.get_player_by_name(target_name)

                    if (
                        not target_player
                        or not target_player.alive
                        or target_player.role == Role.WEREWOLF
                    ):
                        return False

            elif player.role == Role.SEER:
                if action_type != "INVESTIGATE":
                    return False

                # Validate investigate target
                if "target" not in action_data:
                    return False

                target_name = action_data["target"]
                target_player = self.state.get_player_by_name(target_name)

                if (
                    not target_player
                    or not target_player.alive
                    or target_player == player
                ):
                    return False

            else:  # Villager
                # Villagers can't do anything at night
                return False

        # If we got here, the action is valid
        return True

    def _execute_player_action(
        self, player: Player, action: Dict[str, Any], current_time: float
    ) -> float:
        """Execute a player's action based on type (SPEAK, WHISPER, OBSERVE).

        Args:
            player: The player performing the action
            action: The action data
            current_time: The current simulated time

        Returns:
            The updated simulation time after the action is executed
        """
        action_type = action.get("action_type", "").upper()
        message = action.get("message", "")

        # Format time for display
        time_str = time.strftime("%H:%M:%S", time.gmtime(current_time))

        if action_type == "SPEAK":
            # Add message to village channel
            self.state.channels["village"].add_message(player, message)

            # Print message
            print(f"[{time_str}] {player.name}: {message}\n")

            # Add to player's memory
            player.add_memory(
                f'Day {self.state.turn + 1}: You said to the village: "{message}"'
            )

            # Add to other players' memories (just the fact that they spoke)
            for other_player in self.state.get_living_players():
                if other_player != player:
                    other_player.add_memory(
                        f"Day {self.state.turn + 1}: {player.name} spoke to the village."
                    )

            # Increment simulation time based on speech length (2 seconds per word)
            word_count = len(message.split())
            return current_time + (word_count * 2.0)

        elif action_type == "WHISPER":
            target_name = action.get("target", "")
            target_player = self.state.get_player_by_name(target_name)

            if not target_player or not target_player.alive:
                return current_time

            # Create a whisper channel key
            whisper_key = f"whisper_{min(player.name, target_player.name)}_{max(player.name, target_player.name)}"

            # Create the channel if it doesn't exist
            if whisper_key not in self.state.channels:
                whisper_channel = Channel(
                    f"Whispers between {player.name} and {target_player.name}",
                    f"Private whispers between {player.name} and {target_player.name}",
                    [player, target_player],
                )
                self.state.channels[whisper_key] = whisper_channel

            # Add message to whisper channel
            self.state.channels[whisper_key].add_message(player, message)

            # Print message (if verbose)
            if self.config.verbose:
                print(
                    f"[{time_str}] {player.name} whispers to {target_player.name}: {message}\n"
                )

            # Add to memories
            player.add_memory(
                f'Day {self.state.turn + 1}: You whispered to {target_player.name}: "{message}"'
            )
            target_player.add_memory(
                f'Day {self.state.turn + 1}: {player.name} whispered to you: "{message}"'
            )

            # Whispers take less time than speaking (1 second per word)
            word_count = len(message.split())
            return current_time + (word_count * 1.0)

        elif action_type == "OBSERVE":
            # This is a passive action just to observe without speaking
            if self.config.verbose:
                print(f"[{time_str}] {player.name} observes the village silently\n")

            # Add to player's memory
            player.add_memory(
                f"Day {self.state.turn + 1}: You observed the village silently."
            )

            # Observation takes a fixed amount of time (5 seconds)
            return current_time + 5.0

        elif action_type == "VOTE":
            target_name = action.get("target", "")
            target_player = self.state.get_player_by_name(target_name)

            if not target_player or not target_player.alive:
                return current_time

            # Record the vote
            self.state.votes[player.name] = target_player.name

            # Print the vote
            print(f"[{time_str}] {player.name} votes for {target_player.name}")

            # Add to memories
            player.add_memory(
                f"Day {self.state.turn + 1}: You voted to eliminate {target_player.name}."
            )

            # Voting is a brief action (10 seconds)
            return current_time + 10.0

        elif action_type == "WEREWOLF_CHAT":
            # Add message to werewolf channel
            werewolf_channel = self.state.channels["werewolf"]
            werewolf_channel.add_message(player, message)

            # Print message (if verbose)
            if self.config.verbose:
                print(f"[{time_str}] [Werewolf Chat] {player.name}: {message}")

            # Add to werewolf memories
            for werewolf in self.state.get_living_werewolves():
                if werewolf != player:
                    werewolf.add_memory(
                        f'Night {self.state.turn + 1}: {player.name} said to the werewolves: "{message}"'
                    )

            player.add_memory(
                f'Night {self.state.turn + 1}: You said to the werewolves: "{message}"'
            )

            # Werewolf chat uses same timing as speech (2 seconds per word)
            word_count = len(message.split())
            return current_time + (word_count * 2.0)

        elif action_type == "KILL":
            target_name = action.get("target", "")
            target_player = self.state.get_player_by_name(target_name)

            if not target_player or not target_player.alive:
                return current_time

            # Record the werewolf kill vote
            werewolf_votes = self.state.night_actions.get("werewolf_votes", {})
            werewolf_votes[target_player.name] = (
                werewolf_votes.get(target_player.name, 0) + 1
            )
            self.state.night_actions["werewolf_votes"] = werewolf_votes

            # Calculate the current target based on votes
            max_votes = 0
            max_voted_players = []

            for name, count in werewolf_votes.items():
                if count > max_votes:
                    max_votes = count
                    max_voted_players = [name]
                elif count == max_votes:
                    max_voted_players.append(name)

            # If we have a target, set it
            if max_voted_players:
                # Break ties randomly
                chosen_target = random.choice(max_voted_players)
                self.state.night_actions["werewolf_kill"] = chosen_target

            # Print the vote (if verbose)
            if self.config.verbose:
                print(f"[{time_str}] {player.name} votes to kill {target_player.name}")
                print(
                    f"Current werewolf target: {self.state.night_actions.get('werewolf_kill', 'undecided')}"
                )

            # Add to werewolf memories
            player.add_memory(
                f"Night {self.state.turn + 1}: You voted to kill {target_player.name}."
            )

            # Kill vote is a quick action (15 seconds)
            return current_time + 15.0

        elif action_type == "INVESTIGATE":
            target_name = action.get("target", "")
            target_player = self.state.get_player_by_name(target_name)

            if not target_player or not target_player.alive:
                return current_time

            # Record the seer's action
            self.state.night_actions[f"seer_{player.name}"] = target_player.name

            # Determine if the target is a werewolf and tell the seer
            is_werewolf = target_player.role == Role.WEREWOLF
            result = "a Werewolf" if is_werewolf else "not a Werewolf"

            # Add to seer's memory
            player.add_memory(
                f"Night {self.state.turn + 1}: You investigated {target_player.name} "
                f"and discovered they are {result}."
            )

            # Also add to seer's action channel
            action_channel = self.state.channels[f"action_{player.name}"]
            action_channel.add_narrator_message(
                f"You focus your powers on {target_player.name} and receive a vision. "
                f"You see that they are {result}."
            )

            # Print the investigation (if verbose)
            if self.config.verbose:
                print(
                    f"[{time_str}] Seer {player.name} investigates {target_player.name} and learns they are {result}"
                )

            # Investigation is a lengthy action (30 seconds)
            return current_time + 30.0

        # If we reach here, it's an unknown action type, so don't modify time
        return current_time
