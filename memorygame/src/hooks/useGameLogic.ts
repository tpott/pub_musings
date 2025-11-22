import { useState, useCallback } from 'react';
import { Color, GamePhase } from '../types';

const COLORS: Color[] = ['red', 'green', 'yellow', 'blue'];

// TODO update getRandomColor to accept a seed for random or mock
const getRandomColor = (): Color => {
  return COLORS[Math.floor(Math.random() * COLORS.length)];
};

export const useGameLogic = () => {
  const [gamePhase, setGamePhase] = useState<GamePhase>('start');
  const [sequence, setSequence] = useState<Color[]>([]);
  const [playerInput, setPlayerInput] = useState<Color[]>([]);
  const [round, setRound] = useState(0);
  const [currentPlaybackIndex, setCurrentPlaybackIndex] = useState(-1);

  const startGame = useCallback(() => {
    setGamePhase('countdown');
  }, []);

  const startCountdown = useCallback(() => {
    setGamePhase('countdown');
    setSequence([]);
    setPlayerInput([]);
    setRound(0);
  }, []);

  const startRecording = useCallback(() => {
    setGamePhase('recording');
    setPlayerInput([]);

    // Add a new random color to the sequence
    const newColor = getRandomColor();
    setSequence(prev => [...prev, newColor]);
    setRound(prev => prev + 1);
  }, []);

  const startPlayback = useCallback(() => {
    setGamePhase('playback');
  }, []);

  const handlePlayerInput = useCallback((color: Color) => {
    setPlayerInput(prev => {
      const newInput = [...prev, color];
      return newInput;
    });
  }, []);

  const validateInput = useCallback((color: Color, index: number): 'correct' | 'complete' | 'wrong' => {
    if (sequence[index] !== color) {
      return 'wrong';
    }
    if (index === sequence.length - 1) {
      return 'complete';
    }
    return 'correct';
  }, [sequence]);

  const endGame = useCallback(() => {
    setGamePhase('gameOver');
  }, []);

  const resetGame = useCallback(() => {
    setGamePhase('start');
    setSequence([]);
    setPlayerInput([]);
    setRound(0);
    setCurrentPlaybackIndex(-1);
  }, []);

  return {
    gamePhase,
    sequence,
    playerInput,
    round,
    currentPlaybackIndex,
    setCurrentPlaybackIndex,
    startGame,
    startCountdown,
    startRecording,
    startPlayback,
    handlePlayerInput,
    validateInput,
    endGame,
    resetGame,
    setGamePhase,
  };
};
