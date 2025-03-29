# Werewolf Game with AI Players

A Python implementation of the social deduction game Werewolf (also known as Mafia) where all players are AI agents powered by the OpenAI API.

## Features

- Full implementation of the Werewolf game with AI players
- Support for different roles: Villagers, Werewolves, and Seers
- Day/night phase progression with voting and elimination
- AI players with memory, role-specific behavior, and decision-making
- Narrator storytelling for a immersive experience
- Command-line interface for configuring and running games

## Requirements

- Python 3.8+
- Requests library (for OpenAI API communication)

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd werewolf
```

2. Create and activate a virtual environment (optional but recommended):
```bash
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
```

3. Install the required dependencies:
```bash
pip install -r requirements.txt
```

4. Install the package in development mode:
```bash
pip install -e .
```

## Usage

1. Create a file containing your OpenAI API key with secure permissions:
```bash
echo "your-api-key-here" > api_key.txt
chmod 600 api_key.txt
```

2. Run the game with the default configuration:
```bash
python main.py --api-key-file api_key.txt
```

3. Customize the game with additional options:
```bash
python main.py --api-key-file api_key.txt --players 8 --werewolves 2 --seers 1 --verbose
```

## Command Line Options

- `--players`: Total number of players (default: 6)
- `--werewolves`: Number of werewolf players (default: 1)
- `--seers`: Number of seer players (default: 1)
- `--api-key-file`: Path to file containing your OpenAI API key
- `--model`: OpenAI model to use (default: gpt-4o)
- `--verbose`: Enable verbose output to see more details about the game

## Testing

To test the game with a smaller configuration (to save API costs):

```bash
python main.py --api-key-file api_key.txt --players 4 --werewolves 1 --seers 1 --verbose
```

The verbose flag will show you player roles at the start of the game and additional information during gameplay, which is helpful for verifying that the game mechanics work correctly.

## Game Mechanics

1. **Setup Phase**: The game starts with assigning roles to players.

2. **Day Phase**:
   - Players discuss who might be werewolves
   - Players vote on who to eliminate
   - The player with the most votes is eliminated
   - If the eliminated player is a werewolf, the villagers are one step closer to winning

3. **Night Phase**:
   - Werewolves choose a villager to eliminate
   - Seers can check one player to learn if they are a werewolf
   - The chosen victim is eliminated

4. **Win Conditions**:
   - Villagers win if all werewolves are eliminated
   - Werewolves win if they equal or outnumber the remaining villagers

## Project Structure

- `main.py`: Entry point for the game
- `werewolf/config.py`: Game configuration
- `werewolf/game.py`: Main game logic
- `werewolf/state.py`: Game state management
- `werewolf/roles.py`: Player and role definitions
- `werewolf/channel.py`: Communication channels between players
- `werewolf/ai.py`: OpenAI API integration
- `werewolf/prompts.py`: AI prompt templates for different roles and game phases

## Notes

- The game uses the OpenAI API, which incurs costs based on the number of tokens processed.
- The model default is set to gpt-4o but can be changed to a less expensive model with the `--model` option.
- Due to the conversational nature of the game, it can generate a significant number of API requests.

## API Costs

Be aware that running the game will make multiple calls to the OpenAI API, which will incur costs based on your usage. 
The total cost depends on:

1. Number of players
2. Number of days/nights the game runs for
3. The specific OpenAI model used

To minimize costs during testing, consider:
- Using a smaller number of players
- Using a less expensive model

## License

[MIT License](LICENSE)