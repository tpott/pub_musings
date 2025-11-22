import { ColorButtonProps } from '../types';
import './ColorButton.css';

export const ColorButton = ({ color, isActive, disabled, onClick }: ColorButtonProps) => {
  const handleClick = () => {
    if (!disabled) {
      onClick(color);
    }
  };

  return (
    <button
      className={`color-button color-button--${color} ${isActive ? 'color-button--active' : ''}`}
      onClick={handleClick}
      disabled={disabled}
      aria-label={`${color} button`}
    />
  );
};
