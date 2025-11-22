# Memory Game - Implementation Plan

## Overview
A Simon Says-style memory game built with React where players must memorize and repeat increasingly long sequences of colored buttons. The game features audio feedback, background music, and a countdown timer.

## Technical Stack

### Core Technologies
- **React** - UI framework
- **TypeScript** - Type safety and better developer experience
- **CSS/CSS Modules** - Styling with component-scoped styles
- **Web Audio API** - Sound effects and background music

### Build Tools
- **Vite** - Development server and bundling
- **ESLint** - Code linting
- **Prettier** - Code formatting

## Component Architecture

### Component Hierarchy
```
App
├── AudioControls (Volume/Mute buttons)
├── GameContainer
│   ├── StartScreen
│   │   └── StartButton
│   ├── CountdownOverlay
│   └── GameBoard
│       ├── ColorButton (x4: Red, Green, Yellow, Blue)
│       └── GameStatus (optional: shows current phase)
```

### Component Descriptions

**App**
- Root component that manages global state
- Handles audio context initialization
- Renders audio controls and game container

**AudioControls**
- Volume slider (0-100%)
- Mute/unmute toggle button
- Positioned in top right corner

**StartScreen**
- Full-screen overlay with colored buttons in background
- Large centered "Start" button
- Triggers countdown when clicked

**CountdownOverlay**
- Displays "3", "2", "1" countdown
- Full-screen semi-transparent overlay
- Fades out after countdown completes

**GameBoard**
- Container for the 4 colored buttons
- 2x2 grid layout
- Handles button clicks during playback phase

**ColorButton**
- Individual colored button (Red/Green/Yellow/Blue)
- Visual feedback: blink/pulse animation when active
- Plays sound when pressed or during sequence playback
- Disabled during recording phase

## State Management

### Game State Structure
```typescript
interface GameState {
  // Game flow
  gamePhase: 'start' | 'countdown' | 'recording' | 'playback' | 'gameOver';

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

type Color = 'red' | 'green' | 'yellow' | 'blue';
```

### State Management Approach
- Use React **useState** and **useReducer** for game state
- Custom hooks for complex logic:
  - `useGameLogic()` - Game flow and phase transitions
  - `useAudio()` - Audio playback and volume control
  - `useSequence()` - Sequence generation and validation

## Audio System

### Sound Assets Needed
1. **Button Sounds** (4 unique tones for each color)
   - Red: Low tone (e.g., 261.63 Hz - C4)
   - Green: Medium-low tone (e.g., 329.63 Hz - E4)
   - Yellow: Medium-high tone (e.g., 392.00 Hz - G4)
   - Blue: High tone (e.g., 523.25 Hz - C5)

2. **Background Music**
   - Upbeat, looping track
   - Plays during recording and playback phases

3. **Game Event Sounds**
   - Countdown beep (for "3", "2", "1")
   - "Uh oh" / buzzer sound for wrong input
   - Victory sound (optional: for completing rounds)

### Audio Implementation Options

**Option 1: Web Audio API** (Recommended)
- Generate tones programmatically using OscillatorNode
- More control over volume, timing, and effects
- Smaller bundle size (no audio files)

**Option 2: Audio Files**
- Use pre-recorded sound files
- Simpler implementation
- Requires audio asset management

### Audio Player Hook
```typescript
const useAudio = () => {
  const [volume, setVolume] = useState(0.7);
  const [isMuted, setIsMuted] = useState(false);

  const playButtonSound = (color: Color) => { /* ... */ };
  const playBackgroundMusic = () => { /* ... */ };
  const stopBackgroundMusic = () => { /* ... */ };
  const playUhOhSound = () => { /* ... */ };
  const playCountdownBeep = () => { /* ... */ };

  return { /* ... */ };
};
```

## Game Flow and Logic

### Phase Transitions

1. **Start Phase**
   - Display start screen with background buttons visible
   - Wait for user to click "Start" button
   - Transition to: **Countdown**

2. **Countdown Phase**
   - Display "3", "2", "1" with 1 second intervals
   - Play beep sound for each number
   - Start background music
   - Initialize empty sequence
   - Transition to: **Recording**

3. **Recording Phase**
   - Add one random color to sequence
   - Play back entire sequence with visual and audio feedback
   - Each color blinks for ~600ms with ~200ms gap
   - Buttons are disabled (no user input)
   - Transition to: **Playback**

4. **Playback Phase**
   - Enable button clicks
   - Track player input
   - Validate each button press against sequence
   - On correct input: continue until sequence complete
   - On wrong input: transition to **Game Over**
   - On sequence complete: transition to **Recording** (next round)

5. **Game Over Phase**
   - Play "uh oh" sound
   - Stop background music
   - Display game over message with score (rounds completed)
   - Show "Play Again" button
   - Transition to: **Start** (reset game)

### Sequence Validation Logic
```typescript
const validateInput = (color: Color, index: number) => {
  if (sequence[index] !== color) {
    return 'wrong';
  }
  if (index === sequence.length - 1) {
    // Sequence complete - next round
    return 'complete';
  }
  // Continue playback
  return 'correct';
};
```

### Random Color Generation
```typescript
const getRandomColor = (): Color => {
  const colors: Color[] = ['red', 'green', 'yellow', 'blue'];
  return colors[Math.floor(Math.random() * colors.length)];
};
```

## UI/UX Design

### Layout
- **Full viewport** design (100vw x 100vh)
- **2x2 Grid** for colored buttons
- Each button takes up equal space
- Small gaps between buttons

### Color Palette
- **Red**: #FF4444 (active: #FF6666)
- **Green**: #44FF44 (active: #66FF66)
- **Yellow**: #FFFF44 (active: #FFFF66)
- **Blue**: #4444FF (active: #6666FF)
- **Background**: #1a1a1a or similar dark color

### Animations
- **Button Blink**: Increase brightness/scale for 600ms during recording
- **Button Press**: Scale down slightly on click
- **Countdown**: Fade in/out for each number
- **Start Button**: Subtle pulse animation
- **Game Over**: Shake animation or fade out

### Responsive Design
- Mobile-first approach
- Touch-friendly button sizes (minimum 44x44px)
- Scale button grid based on viewport size
- Maintain aspect ratio

## Implementation Phases

### Phase 1: Project Setup and Basic UI
- [ ] Initialize React project with TypeScript
- [ ] Set up project structure and dependencies
- [ ] Create basic component structure
- [ ] Implement static UI for start screen
- [ ] Implement 2x2 colored button grid
- [ ] Add basic CSS styling

### Phase 2: Game State Management
- [ ] Set up game state with useReducer or context
- [ ] Implement phase transitions (start → countdown → recording → playback)
- [ ] Create custom hooks for game logic
- [ ] Implement sequence generation
- [ ] Implement sequence validation logic

### Phase 3: Audio System
- [ ] Set up audio context and hooks
- [ ] Implement button sounds (Web Audio API)
- [ ] Add background music playback
- [ ] Implement volume and mute controls
- [ ] Add countdown beeps and "uh oh" sound

### Phase 4: Game Logic Integration
- [ ] Connect button clicks to game logic
- [ ] Implement recording phase animation and playback
- [ ] Implement playback phase input validation
- [ ] Handle game over state
- [ ] Add "Play Again" functionality

### Phase 5: Polish and Enhancement
- [ ] Add animations and transitions
- [ ] Improve visual feedback (button blinks, pulses)
- [ ] Add round/score display
- [ ] Optimize performance
- [ ] Add keyboard support (optional)
- [ ] Test on mobile devices

### Phase 6: Testing and Deployment
- [ ] Write unit tests for game logic
- [ ] Test audio on different browsers
- [ ] Cross-browser testing
- [ ] Mobile responsive testing
- [ ] Build for production
- [ ] Deploy to hosting platform

## Technical Considerations

### Performance
- Debounce button clicks to prevent double inputs
- Use CSS animations over JavaScript when possible
- Optimize re-renders with React.memo and useCallback
- Preload audio assets

### Browser Compatibility
- Test Web Audio API support across browsers
- Provide fallback for older browsers
- Handle autoplay restrictions (background music)
- Test touch events on mobile devices

### Accessibility
- Add ARIA labels for screen readers
- Ensure keyboard navigation support
- Provide visual alternatives for audio cues
- Use semantic HTML elements

### State Persistence (Optional Enhancement)
- Save high score to localStorage
- Resume game on page refresh
- Track statistics (rounds played, success rate)

## Future Enhancements

- **Speech-to-text Controls**: Speak the color button presses
- **Difficulty Levels**: Adjust playback speed
- **Multiplayer**: Two-player mode
- **Sound Themes**: Different sound packs
- **Authentication and Billing**: Authenticated users can pay for premium features

## Testing Strategy

### Unit Tests
- Game state reducer logic
- Sequence validation functions
- Random color generation

### Integration Tests
- Phase transitions
- Button click handling
- Audio playback timing

### Manual Testing
- Cross-browser compatibility
- Mobile touch interactions
- Audio playback on different devices
- Performance under extended play sessions

## Next Steps

1. Set up the React project
2. Create the basic component structure
3. Implement game state management
4. Build out each phase incrementally
5. Add audio integration
6. Polish UI/UX
7. Test and deploy
