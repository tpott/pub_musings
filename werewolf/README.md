# Werewolf Game with AI Players

A Python implementation of the social deduction game Werewolf (also known as Mafia) where all players are AI agents powered by either the Anthropic or OpenAI API.

## Features

- Full implementation of the Werewolf game with AI players
- Support for different roles: Villagers, Werewolves, and Seers
- Day/night phase progression with voting and elimination
- AI players with memory, role-specific behavior, and decision-making
- Narrator storytelling for a immersive experience
- Command-line interface for configuring and running games
- Support for both Anthropic Claude and OpenAI GPT models

## Requirements

- Python 3.8+
- Requests library (for API communication)
- Black (for code formatting, development only)

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

1. Create a file containing your Anthropic API key with secure permissions:
```bash
echo "your-anthropic-api-key-here" > anthropic_key.txt
chmod 600 anthropic_key.txt
```

2. Run the game with the default configuration using Anthropic's Claude:
```bash
python main.py --anthropic-key-file anthropic_key.txt
```

3. Customize the game with additional options:
```bash
python main.py --anthropic-key-file anthropic_key.txt --players 8 --werewolves 2 --seers 1 --verbose
```

4. You can also specify a particular Claude model:
```bash
python main.py --anthropic-key-file anthropic_key.txt --model claude-3-opus-20240229
```

## Command Line Options

- `--players`: Total number of players (default: 6)
- `--werewolves`: Number of werewolf players (default: 1)
- `--seers`: Number of seer players (default: 1)
- `--anthropic-key-file`: Path to file containing your Anthropic API key
- `--openai-key-file`: Path to file containing your OpenAI API key (alternative to using Anthropic)
- `--model`: Model to use (defaults to claude-3-7-sonnet-20250219 for Anthropic or gpt-4o for OpenAI)
- `--max-words`: Maximum number of words per AI response (default: 100)
- `--verbose`: Enable verbose output to see more details about the game
- `--log-to-file`: Log detailed AI prompts and responses to logs.txt (useful for debugging or studying AI behavior)

**Note**: You must provide either `--anthropic-key-file` OR `--openai-key-file`, but not both.

## Testing

To test the game with a smaller configuration (to save API costs):

```bash
python main.py --anthropic-key-file anthropic_key.txt --players 4 --werewolves 1 --seers 1 --verbose
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
- `werewolf/ai.py`: Common AI client interface
- `werewolf/anthropic.py`: Anthropic API integration
- `werewolf/openai.py`: OpenAI API integration
- `werewolf/prompts.py`: AI prompt templates for different roles and game phases

## Model Options

### Anthropic Models
You can specify different Claude models using the `--model` parameter:

- `claude-3-7-sonnet-20250219` (default): A balanced model with strong reasoning at an affordable price
- `claude-3-5-haiku-20241022`: Fast, efficient AI model balancing speed and intelligence perfectly
- `claude-3-opus-20240229`: Most capable model for complex tasks, highest quality

Example:
```bash
python main.py --anthropic-key-file anthropic_key.txt --model claude-3-opus-20240229
```

### OpenAI Models
For OpenAI, you can specify different GPT models:

- `gpt-4o` (default): Latest balanced model with strong reasoning abilities
- `gpt-4o-mini`: More efficient, smaller model
- `gpt-4-turbo`: Previous generation capable model
- `gpt-3.5-turbo`: Cost-effective model for simpler tasks

Example:
```bash
python main.py --openai-key-file openai_key.txt --model gpt-3.5-turbo
```

## Notes

- The game uses either the Anthropic or OpenAI API, which incurs costs based on the number of tokens processed.
- The default model for Anthropic is Claude 3.7 Sonnet, while the default for OpenAI is GPT-4o.
- Due to the conversational nature of the game, it can generate a significant number of API requests.
- More economical models will run faster and cost less but may have lower quality gameplay.

## API Costs

Be aware that running the game will make multiple calls to the chosen API, which will incur costs based on your usage. 
The total cost depends on:

1. Number of players
2. Number of days/nights the game runs for
3. The specific model used

To minimize costs during testing, consider:
- Using a smaller number of players
- Using a less expensive model

## License

[MIT License](LICENSE)
