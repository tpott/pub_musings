import './GameOver.css';

interface GameOverProps {
  round: number;
  onPlayAgain: () => void;
}

export const GameOver = ({ round, onPlayAgain }: GameOverProps) => {
  return (
    <div className="game-over">
      <div className="game-over-content">
        <h1 className="game-over-title">Game Over!</h1>
        <p className="game-over-score">You completed {round} round{round !== 1 ? 's' : ''}!</p>
        <button className="play-again-button" onClick={onPlayAgain}>
          Play Again
        </button>
      </div>
    </div>
  );
};
