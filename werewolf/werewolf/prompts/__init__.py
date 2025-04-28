# Re-export all prompt functions
from werewolf.prompts.player import create_player_context

# Day phase prompts
from werewolf.prompts.day import (
    create_day_discussion_prompt,
    create_day_voting_prompt,
    create_day_reaction_prompt,
    create_neighbor_whisper_prompt,
)

# Night phase prompts
from werewolf.prompts.night import (
    create_werewolf_night_prompt,
    create_werewolf_kill_prompt,
    create_seer_night_prompt,
)

# Narrator prompts
from werewolf.prompts.narrator import (
    create_narrator_day_start_message,
    create_narrator_day_voting_message,
    create_narrator_day_end_message,
    create_narrator_night_start_message,
    create_narrator_game_over_message,
)
