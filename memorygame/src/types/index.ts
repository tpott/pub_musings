export type Color = 'red' | 'green' | 'yellow' | 'blue';

export type GamePhase = 'start' | 'countdown' | 'recording' | 'playback' | 'gameOver';

export interface GameState {
  // Game flow
  gamePhase: GamePhase;

  // Sequence tracking
  sequence: Color[];
  currentPlaybackIndex: number;
  playerInput: Color[];

  // Round tracking
  round: number;

  // Audio
  volume: number;
  isMuted: boolean;
  backgroundMusicPlaying: boolean;
}

export interface ColorButtonProps {
  color: Color;
  isActive: boolean;
  disabled: boolean;
  onClick: (color: Color) => void;
}

export interface AudioControlsProps {
  volume: number;
  isMuted: boolean;
  onVolumeChange: (volume: number) => void;
  onMuteToggle: () => void;
}

export interface CountdownOverlayProps {
  count: number | null;
}

export interface GameBoardProps {
  sequence: Color[];
  currentPlaybackIndex: number;
  gamePhase: GamePhase;
  onColorClick: (color: Color) => void;
}
