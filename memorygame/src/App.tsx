import { useState, useEffect, useCallback } from 'react';
import { useGameLogic } from './hooks/useGameLogic';
import { useAudio } from './hooks/useAudio';
import { GameBoard } from './components/GameBoard';
import { StartScreen } from './components/StartScreen';
import { CountdownOverlay } from './components/CountdownOverlay';
import { AudioControls } from './components/AudioControls';
import { GameOver } from './components/GameOver';
import { Color } from './types';
import './App.css';

function App() {
  const [volume, setVolume] = useState(0.7);
  const [isMuted, setIsMuted] = useState(false);
  const [countdownValue, setCountdownValue] = useState<number | null>(null);

  const {
    gamePhase,
    sequence,
    playerInput,
    round,
    currentPlaybackIndex,
    setCurrentPlaybackIndex,
    startGame,
    startRecording,
    startPlayback,
    handlePlayerInput,
    validateInput,
    endGame,
    resetGame,
  } = useGameLogic();

  const {
    playButtonSound,
    playBackgroundMusic,
    stopBackgroundMusic,
    playCountdownBeep,
    playUhOhSound,
  } = useAudio(volume, isMuted);

  const handleMuteToggle = useCallback(() => {
    setIsMuted(prev => !prev);
  }, []);

  // Handle countdown phase
  useEffect(() => {
    if (gamePhase !== 'countdown') {
      return;
    }

    let count = 3;
    setCountdownValue(count);
    playCountdownBeep();

    const interval = setInterval(() => {
      count -= 1;
      if (count > 0) {
        setCountdownValue(count);
        playCountdownBeep();
      } else {
        setCountdownValue(null);
        clearInterval(interval);
        playBackgroundMusic();
        startRecording();
      }
    }, 1000);

    return () => clearInterval(interval);
  }, [gamePhase, playCountdownBeep, playBackgroundMusic, startRecording]);

  // Handle recording phase - play sequence
  useEffect(() => {
    if (gamePhase !== 'recording' || sequence.length === 0) {
      return;
    }

    let index = 0;
    setCurrentPlaybackIndex(index);

    const playSequence = () => {
      if (index >= sequence.length) {
        return;
      }

      playButtonSound(sequence[index]);
      setCurrentPlaybackIndex(index);

      setTimeout(() => {
        setCurrentPlaybackIndex(-1);

        setTimeout(() => {
          index++;
          if (index < sequence.length) {
            playSequence();
          } else {
            startPlayback();
          }
        }, 200);
      }, 600);
    };

    playSequence();
  }, [gamePhase, sequence, playButtonSound, startPlayback, setCurrentPlaybackIndex]);

  // Handle player color click
  const handleColorClick = useCallback((color: Color) => {
    if (gamePhase !== 'playback') { return; }

    playButtonSound(color);
    handlePlayerInput(color);

    const currentIndex = playerInput.length;
    const result = validateInput(color, currentIndex);

    if (result === 'wrong') {
      playUhOhSound();
      stopBackgroundMusic();
      endGame();
    } else if (result === 'complete') {
      // Round complete, start next round
      setTimeout(() => {
        startRecording();
      }, 1000);
    }
  }, [
    gamePhase,
    playerInput.length,
    playButtonSound,
    handlePlayerInput,
    validateInput,
    playUhOhSound,
    stopBackgroundMusic,
    endGame,
    startRecording,
  ]);

  // Handle game over
  const handlePlayAgain = useCallback(() => {
    resetGame();
  }, [resetGame]);

  return (
    <div className="app">
      <AudioControls
        volume={volume}
        isMuted={isMuted}
        onVolumeChange={setVolume}
        onMuteToggle={handleMuteToggle}
      />

      {gamePhase === 'start' && <StartScreen onStart={startGame} />}

      {countdownValue !== null && <CountdownOverlay count={countdownValue} />}

      {gamePhase === 'gameOver' && (
        <GameOver round={round} onPlayAgain={handlePlayAgain} />
      )}

      <GameBoard
        sequence={sequence}
        currentPlaybackIndex={currentPlaybackIndex}
        gamePhase={gamePhase}
        onColorClick={handleColorClick}
      />
    </div>
  );
}

export default App;
