import { useRef, useCallback, useEffect } from 'react';
import { Color } from '../types';

// Frequency mapping for each color button
const COLOR_FREQUENCIES: Record<Color, number> = {
  red: 261.63,    // C4
  green: 329.63,  // E4
  yellow: 392.00, // G4
  blue: 523.25,   // C5
};

export const useAudio = (volume: number, isMuted: boolean) => {
  const audioContextRef = useRef<AudioContext | null>(null);
  const backgroundOscillatorRef = useRef<OscillatorNode | null>(null);
  const backgroundGainRef = useRef<GainNode | null>(null);

  // Initialize audio context
  useEffect(() => {
    audioContextRef.current = new (window.AudioContext || (window as any).webkitAudioContext)();
    return () => {
      if (audioContextRef.current !== null) {
        audioContextRef.current.close();
      }
    };
  }, []);

  // Play button sound
  const playButtonSound = useCallback((color: Color) => {
    if (!audioContextRef.current || isMuted) return;

    const context = audioContextRef.current;
    const oscillator = context.createOscillator();
    const gainNode = context.createGain();

    oscillator.connect(gainNode);
    gainNode.connect(context.destination);

    oscillator.frequency.value = COLOR_FREQUENCIES[color];
    oscillator.type = 'sine';

    gainNode.gain.setValueAtTime(volume * 0.3, context.currentTime);
    gainNode.gain.exponentialRampToValueAtTime(0.01, context.currentTime + 0.3);

    oscillator.start(context.currentTime);
    oscillator.stop(context.currentTime + 0.3);
  }, [volume, isMuted]);

  // Play background music (simple looping melody)
  const playBackgroundMusic = useCallback(() => {
    if (!audioContextRef.current || isMuted || backgroundOscillatorRef.current) return;

    const context = audioContextRef.current;
    const oscillator = context.createOscillator();
    const gainNode = context.createGain();

    oscillator.connect(gainNode);
    gainNode.connect(context.destination);

    // Create a simple melody pattern
    oscillator.type = 'sine';
    oscillator.frequency.value = 440; // A4

    gainNode.gain.value = volume * 0.1; // Background music quieter

    oscillator.start();

    backgroundOscillatorRef.current = oscillator;
    backgroundGainRef.current = gainNode;

    // Simple melody loop using frequency modulation
    const melodyLoop = () => {
      if (!backgroundOscillatorRef.current) return;

      const frequencies = [440, 494, 523, 587, 523, 494]; // A4, B4, C5, D5, C5, B4
      let index = 0;

      const interval = setInterval(() => {
        if (!backgroundOscillatorRef.current) {
          clearInterval(interval);
          return;
        }
        backgroundOscillatorRef.current.frequency.value = frequencies[index];
        index = (index + 1) % frequencies.length;
      }, 500);

      return interval;
    };

    const intervalId = melodyLoop();

    // Store interval ID for cleanup
    (oscillator as any).intervalId = intervalId;
  }, [volume, isMuted]);

  // Stop background music
  const stopBackgroundMusic = useCallback(() => {
    if (backgroundOscillatorRef.current) {
      const oscillator = backgroundOscillatorRef.current;
      if ((oscillator as any).intervalId) {
        clearInterval((oscillator as any).intervalId);
      }
      oscillator.stop();
      backgroundOscillatorRef.current = null;
      backgroundGainRef.current = null;
    }
  }, []);

  // Play countdown beep
  const playCountdownBeep = useCallback(() => {
    if (!audioContextRef.current || isMuted) return;

    const context = audioContextRef.current;
    const oscillator = context.createOscillator();
    const gainNode = context.createGain();

    oscillator.connect(gainNode);
    gainNode.connect(context.destination);

    oscillator.frequency.value = 800; // High beep
    oscillator.type = 'square';

    gainNode.gain.setValueAtTime(volume * 0.2, context.currentTime);
    gainNode.gain.exponentialRampToValueAtTime(0.01, context.currentTime + 0.1);

    oscillator.start(context.currentTime);
    oscillator.stop(context.currentTime + 0.1);
  }, [volume, isMuted]);

  // Play "uh oh" sound
  const playUhOhSound = useCallback(() => {
    if (!audioContextRef.current || isMuted) return;

    const context = audioContextRef.current;
    const oscillator = context.createOscillator();
    const gainNode = context.createGain();

    oscillator.connect(gainNode);
    gainNode.connect(context.destination);

    // Descending tone for "uh oh" effect
    oscillator.frequency.setValueAtTime(400, context.currentTime);
    oscillator.frequency.exponentialRampToValueAtTime(200, context.currentTime + 0.5);
    oscillator.type = 'sawtooth';

    gainNode.gain.setValueAtTime(volume * 0.3, context.currentTime);
    gainNode.gain.exponentialRampToValueAtTime(0.01, context.currentTime + 0.5);

    oscillator.start(context.currentTime);
    oscillator.stop(context.currentTime + 0.5);
  }, [volume, isMuted]);

  // Update background music volume when volume changes
  useEffect(() => {
    if (backgroundGainRef.current) {
      backgroundGainRef.current.gain.value = isMuted ? 0 : volume * 0.1;
    }
  }, [volume, isMuted]);

  return {
    playButtonSound,
    playBackgroundMusic,
    stopBackgroundMusic,
    playCountdownBeep,
    playUhOhSound,
  };
};
