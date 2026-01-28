# Playback Speed Control

## Overview

Variable playback speed control for language learners who want to slow down video to hear pronunciation more clearly.

## Status: Implemented

Created: 2026-01-27
Updated: 2026-01-28

## User Requirements

Language learners benefit from:
1. Slowing down speech to catch difficult words or pronunciation
2. Speeding up content for review of familiar material
3. Quick access to speed controls without interrupting the learning flow

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
- At very slow speeds (< 0.5x), audio quality may degrade

**Note:** `HTMLMediaElement.preservesPitch` is supported in all modern browsers and is `true` by default. No AudioContext is needed - the browser handles pitch preservation automatically.

Testing showed:
- 0.75x - Good quality, clear speech, slight time stretch
- 0.5x - Acceptable for close listening, some artifacts
- Below 0.5x - Quality degrades noticeably

## Implementation

### Speed Options

Provide these speed options for language learning use case:

| Label | Value | Use Case |
|-------|-------|----------|
| 0.5x  | 0.5   | Detailed listening, difficult pronunciations |
| 0.75x | 0.75  | Learning mode, catching new vocabulary |
| 1x    | 1.0   | Normal playback (default) |
| 1.25x | 1.25  | Light review, familiar content |
| 1.5x  | 1.5   | Fast review |
| 2x    | 2.0   | Quick scan/skip through |

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
// Speed options array
const PLAYBACK_SPEEDS = [0.5, 0.75, 1.0, 1.25, 1.5, 2.0];
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
1. Set speed to 0.75x, verify audio pitch is preserved
2. Reload page, verify speed preference is restored
3. Test keyboard shortcuts cycle through speeds
4. Test at extreme speeds (0.5x, 2x) for audio quality
5. Verify subtitles stay synced at all speeds

### E2E Tests
- Test speed selector appears in UI
- Test clicking changes displayed speed
- Test keyboard shortcuts work
- Test persistence across page reload

## Future Enhancements

1. **Fine-grained control**: Slider for any speed from 0.25x to 3x
2. **Per-video speed**: Remember speed per video ID
3. **Speed presets**: Custom speed presets for users
4. **Auto-slow on difficult segments**: AI-detected difficult passages automatically slow down
