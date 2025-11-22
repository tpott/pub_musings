import './StartScreen.css';

interface StartScreenProps {
  onStart: () => void;
}

export const StartScreen = ({ onStart }: StartScreenProps) => {
  return (
    <div className="start-screen">
      <button className="start-button" onClick={onStart}>
        Start
      </button>
    </div>
  );
};
