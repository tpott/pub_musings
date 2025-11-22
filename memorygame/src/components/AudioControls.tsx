import { AudioControlsProps } from '../types';
import './AudioControls.css';

export const AudioControls = ({
  volume,
  isMuted,
  onVolumeChange,
  onMuteToggle
}: AudioControlsProps) => {
  return (
    <div className="audio-controls">
      <button
        className="mute-button"
        onClick={onMuteToggle}
        aria-label={isMuted ? 'Unmute' : 'Mute'}
      >
        {isMuted ? '🔇' : '🔊'}
      </button>
      <input
        type="range"
        min="0"
        max="100"
        value={volume * 100}
        onChange={(e) => onVolumeChange(Number(e.target.value) / 100)}
        className="volume-slider"
        aria-label="Volume control"
      />
    </div>
  );
};
