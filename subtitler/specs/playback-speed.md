# Playback Speed Control

## Overview

Variable playback speed control for language learners who want to slow down video to hear pronunciation more clearly.

## Status: Implemented

Created: 2026-01-27
Updated: 2026-01-29

## IMPORTANT: Speed Options Are Restricted

**DO NOT expand the speed options without explicit user/owner approval.**

The allowed speeds are **0.8x, 0.9x, and 1.0x only**. This restriction has been
violated multiple times and reverted each time. The owner has explicitly stated:
only these 3 speeds are acceptable. More speeds may be added in the future, but
only with approval.

## User Requirements

Language learners benefit from:
1. Slowing down speech to catch difficult words or pronunciation
2. Quick access to speed controls without interrupting the learning flow

## Technical Approach

### HTML5 Video playbackRate

The HTML5 `<video>` element supports the `playbackRate` property:
- `playbackRate = 1.0` is normal speed
- `playbackRate = 0.75` is 75% speed (slower)
- `playbackRate = 1.5` is 150% speed (faster)

Modern browsers (Chrome, Firefox, Safari, Edge) all support playback rates from 0.25x to 4x+.

### Audio Quality Considerations

When changing playback rate:
- Browser automatically adjusts audio pitch preservation
- Modern browsers use `preservesPitch` (true by default) to prevent "chipmunk" effect at high speeds

**Note:** `HTMLMediaElement.preservesPitch` is supported in all modern browsers and is `true` by default. No AudioContext is needed - the browser handles pitch preservation automatically.

Testing showed:
- 0.9x - Excellent quality, nearly indistinguishable from normal
- 0.8x - Good quality, clear speech, slight time stretch

## Implementation

### Speed Options

**Only these 3 speeds are allowed (DO NOT add more without owner approval):**

| Label | Value | Use Case |
|-------|-------|----------|
| 0.8x  | 0.8   | Slower for catching difficult pronunciations |
| 0.9x  | 0.9   | Slightly slower for new vocabulary |
| 1x    | 1.0   | Normal playback (default) |

### UI Design

Speed selector button next to video controls:
- Shows current speed (e.g., "1x")
- Click cycles through speeds OR opens dropdown menu
- Keyboard shortcut: `[` to slow down, `]` to speed up

### Persistence

Store user's preferred speed in localStorage:
- Key: `subtitler:playback-speed`
- Value: numeric rate (e.g., `1.0`, `0.75`)
- Applied automatically when video loads

### Implementation Locations

1. **upload.astro**: Add speed control to video player section
2. **videos.astro**: Add speed control to modal video player

### Code Structure

```typescript
// Speed options array - DO NOT expand without owner approval
const PLAYBACK_SPEEDS = [0.8, 0.9, 1.0];
const DEFAULT_SPEED = 1.0;

// Storage key
const STORAGE_KEY = 'subtitler:playback-speed';

// Get saved speed
function getSavedSpeed(): number {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
        const speed = parseFloat(saved);
        if (PLAYBACK_SPEEDS.includes(speed)) {
            return speed;
        }
    }
    return DEFAULT_SPEED;
}

// Save speed preference
function saveSpeed(speed: number): void {
    localStorage.setItem(STORAGE_KEY, speed.toString());
}

// Apply speed to video
function setPlaybackSpeed(video: HTMLVideoElement, speed: number): void {
    video.playbackRate = speed;
    saveSpeed(speed);
}
```

### Keyboard Shortcuts

Add to existing keyboard handler:
- `[` (bracket left): Decrease speed to previous step
- `]` (bracket right): Increase speed to next step

Document in keyboard shortcuts modal (triggered by `?`).

## Testing Plan

### Unit Tests
- Test getSavedSpeed() returns default when localStorage empty
- Test getSavedSpeed() returns saved value when valid
- Test getSavedSpeed() returns default for invalid saved values
- Test saveSpeed() persists to localStorage

### Manual Testing
1. Set speed to 0.8x, verify audio pitch is preserved
2. Reload page, verify speed preference is restored
3. Test keyboard shortcuts cycle through 0.8x, 0.9x, 1x
4. Verify subtitles stay synced at all speeds

### E2E Tests
- Test speed selector appears in UI
- Test clicking changes displayed speed
- Test keyboard shortcuts work
- Test persistence across page reload

## Future Enhancements (require owner approval)

1. **More speed options**: Additional speeds like 0.7x, 0.6x (requires owner approval)
2. **Per-video speed**: Remember speed per video ID
3. **Auto-slow on difficult segments**: AI-detected difficult passages automatically slow down
