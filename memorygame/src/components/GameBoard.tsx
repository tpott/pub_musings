import { GameBoardProps, Color } from '../types';
import { ColorButton } from './ColorButton';
import './GameBoard.css';

export const GameBoard = ({
  sequence,
  currentPlaybackIndex,
  gamePhase,
  onColorClick
}: GameBoardProps) => {
  const colors: Color[] = ['red', 'green', 'yellow', 'blue'];

  const isButtonDisabled = gamePhase !== 'playback';

  const isButtonActive = (color: Color) => {
    if (gamePhase === 'recording' && currentPlaybackIndex >= 0) {
      return sequence[currentPlaybackIndex] === color;
    }
    return false;
  };

  return (
    <div className="game-board">
      {colors.map(color => (
        <ColorButton
          key={color}
          color={color}
          isActive={isButtonActive(color)}
          disabled={isButtonDisabled}
          onClick={onColorClick}
        />
      ))}
    </div>
  );
};
