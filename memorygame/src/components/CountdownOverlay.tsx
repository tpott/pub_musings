import { CountdownOverlayProps } from '../types';
import './CountdownOverlay.css';

export const CountdownOverlay = ({ count }: CountdownOverlayProps) => {
  if (count === null) return null;

  return (
    <div className="countdown-overlay">
      <div className="countdown-number">{count}</div>
    </div>
  );
};
