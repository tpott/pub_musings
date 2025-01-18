import { render, screen } from '@testing-library/react';
import App from './App';

test('renders party list', () => {
  render(<App />);
  const linkElement = screen.getByText(/party list/i);
  expect(linkElement).toBeInTheDocument();
});
