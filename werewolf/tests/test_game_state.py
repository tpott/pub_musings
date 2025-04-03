#!/usr/bin/env python3
"""
Tests for the Werewolf game state transitions
"""

import unittest
from unittest.mock import patch, MagicMock, call
from werewolf.config import GameConfig
from werewolf.game import Game
from werewolf.state import GamePhase, GameState
from werewolf.roles import Role, Player


class MockAIClient:
    """Mock AI client for testing"""
    def complete(self, messages):
        """Return a mock response"""
        return "mock response"


class TestNightPhaseTransitions(unittest.TestCase):
    """Test case for night phase transitions"""

    def setUp(self):
        """Set up a game for testing"""
        # Create test configuration with minimal players
        self.config = GameConfig(
            total_players=4,
            num_werewolves=1,
            num_seers=1,
            # Use mock API values
            openai_api_key="test_key",
            api_type="openai",
            model_name="test_model",
            verbose=True
        )
        
        # Create game but don't run it
        self.game = Game(self.config)
        
        # Replace AI client with mock
        self.game.ai = MockAIClient()
        
        # Initialize the game to night phase
        self.game.state.phase = GamePhase.NIGHT

    def test_werewolf_actions_after_seer(self):
        """
        Test that werewolf actions still occur after seer actions.
        This tests the bug where a seer action might cause
        werewolf voting to be skipped during night phase.
        """
        # Get the seer and werewolf player
        seer = next(p for p in self.game.state.players if p.role == Role.SEER)
        werewolf = next(p for p in self.game.state.players if p.role == Role.WEREWOLF)
        villager = next(p for p in self.game.state.players if p.role == Role.VILLAGER)
        
        # Mock the night action methods to avoid any side effects
        with patch.object(self.game, '_process_seer_action') as mock_seer_action, \
             patch.object(self.game, '_process_werewolf_actions') as mock_werewolf_actions, \
             patch.object(self.game, '_process_werewolf_kill') as mock_werewolf_kill:
            
            # Run the night phase
            self.game._run_night_phase()
            
            # Verify all methods were called
            mock_werewolf_actions.assert_called_once()
            mock_seer_action.assert_called_once()
            mock_werewolf_kill.assert_called_once()

    def test_method_sequence_in_night_phase(self):
        """
        Test that the night phase methods are called in correct sequence.
        This specifically addresses the bug where werewolf actions might be skipped.
        """
        # Keep track of calls in order
        call_sequence = []
        
        def track_werewolf_actions(werewolves):
            call_sequence.append('werewolf_actions')
            # Add werewolf_kill to night_actions to simulate successful voting
            self.game.state.night_actions['werewolf_kill'] = 'test_victim'
            
        def track_seer_action(seer):
            call_sequence.append('seer_action')
            # Add seer action to night_actions to simulate successful investigation
            self.game.state.night_actions[f'seer_{seer.name}'] = 'test_target'
            
        def track_werewolf_kill():
            call_sequence.append('werewolf_kill')
            return None  # No actual kill in test
        
        # Mock advance_phase to prevent night_actions from being reset
        original_advance_phase = self.game.state.advance_phase
        def mock_advance_phase():
            # Don't actually advance phase or reset night_actions in test
            # Just track that it was called
            call_sequence.append('advance_phase')
        
        # Patch the methods with our tracking functions
        with patch.object(self.game, '_process_werewolf_actions', side_effect=track_werewolf_actions), \
             patch.object(self.game, '_process_seer_action', side_effect=track_seer_action), \
             patch.object(self.game, '_process_werewolf_kill', side_effect=track_werewolf_kill), \
             patch.object(self.game.state, 'advance_phase', side_effect=mock_advance_phase):
            
            # Clear night actions before test
            self.game.state.night_actions = {}
            
            # Run the night phase
            self.game._run_night_phase()
            
            # Verify correct sequence - seer_action should be in the sequence
            expected_sequence = ['werewolf_actions', 'seer_action', 'werewolf_kill', 'advance_phase']
            self.assertEqual(call_sequence, expected_sequence, 
                             f"Expected sequence {expected_sequence} but got {call_sequence}")
            
            # Verify both actions were recorded in the night_actions dictionary
            self.assertIn('werewolf_kill', self.game.state.night_actions, 
                          "Werewolf kill action was not recorded in night_actions")
            
            seer = next(p for p in self.game.state.players if p.role == Role.SEER)
            seer_action_key = f'seer_{seer.name}'
            self.assertIn(seer_action_key, self.game.state.night_actions,
                          f"Seer action {seer_action_key} was not recorded in night_actions")


class TestStateMachine(unittest.TestCase):
    """Test the state machine transitions"""
    
    def setUp(self):
        self.config = GameConfig(
            total_players=4,
            num_werewolves=1,
            num_seers=1,
            openai_api_key="test_key",
            api_type="openai"
        )
        self.state = GameState(self.config)
    
    def test_phase_transitions(self):
        """Test that phases transition correctly"""
        # Start in SETUP phase
        self.assertEqual(self.state.phase, GamePhase.SETUP)
        
        # Advance to DAY_DISCUSSION
        self.state.advance_phase()
        self.assertEqual(self.state.phase, GamePhase.DAY_DISCUSSION)
        
        # Advance to DAY_VOTING
        self.state.advance_phase()
        self.assertEqual(self.state.phase, GamePhase.DAY_VOTING)
        
        # Advance to NIGHT
        self.state.advance_phase()
        self.assertEqual(self.state.phase, GamePhase.NIGHT)
        
        # Advance to DAY_DISCUSSION (next day)
        self.state.advance_phase()
        self.assertEqual(self.state.phase, GamePhase.DAY_DISCUSSION)
        self.assertEqual(self.state.turn, 1)  # Turn should be incremented
        
    def test_night_actions_persistence(self):
        """Test that night_actions are maintained throughout the night phase"""
        # Start in NIGHT phase
        self.state.phase = GamePhase.NIGHT
        
        # Add some night actions
        self.state.night_actions["werewolf_kill"] = "Victim"
        self.state.night_actions["seer_Seer"] = "Target"
        
        # Verify actions are present
        self.assertEqual(self.state.night_actions["werewolf_kill"], "Victim")
        self.assertEqual(self.state.night_actions["seer_Seer"], "Target")
        
        # Advance to DAY_DISCUSSION - night actions should be cleared
        self.state.advance_phase()
        self.assertEqual(self.state.phase, GamePhase.DAY_DISCUSSION)
        self.assertEqual(self.state.night_actions, {}, 
                         "Night actions should be cleared when advancing to DAY_DISCUSSION")
        
        # Go back to NIGHT phase
        self.state.phase = GamePhase.NIGHT
        
        # Add night action for werewolf
        self.state.night_actions["werewolf_kill"] = "Victim2"
        
        # Add seer action later - werewolf action should still be there
        self.state.night_actions["seer_Seer"] = "Target2"
        
        # Both actions should be present
        self.assertEqual(self.state.night_actions["werewolf_kill"], "Victim2")
        self.assertEqual(self.state.night_actions["seer_Seer"], "Target2")


if __name__ == "__main__":
    unittest.main()