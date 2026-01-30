# Kid Mode (Screen Lock)

## Context

User feedback (2026-01-30): When watching videos on mobile, kids tapping the screen
accidentally clicks buttons. A "lock" mode is needed to prevent accidental button
presses during video playback.

## Requirements

1. **Lock button** appears in the video modal's `.modal-actions` bar, on the right side
   (the speed dropdown is on the left)
2. **When locked:**
   - All interactive elements in the modal are disabled (buttons, links, segments)
   - The video `<video>` element's native controls remain functional (play/pause, seek, volume)
   - Keyboard shortcuts for speed and modal close are disabled
   - Clicking the overlay background does NOT close the modal
   - A visible "locked" indicator is shown so the user knows the state
3. **Unlocking** requires a slightly harder gesture than a single tap, to prevent
   kids from accidentally unlocking:
   - **Long-press** the lock button (hold for 1 second) to unlock
   - The lock button shows a progress indicator during the hold
   - A single tap on the lock button while locked does nothing (prevents accidental unlock)
4. **Lock state is per-session** (not persisted). Each time the modal opens, it starts unlocked.

## Design

### UI

- Lock button: a simple padlock icon button, positioned at the far right of `.modal-actions`
- When unlocked: open padlock icon, subtle styling
- When locked: closed padlock icon, highlighted (accent color background)
- During unlock hold: button shows a fill animation (CSS transition on background width)

### HTML (added to videos.astro modal-actions)

```html
<button class="kid-mode-btn" id="modalKidModeBtn" aria-label="Lock screen for kid mode" aria-pressed="false">
  <span class="kid-mode-icon" aria-hidden="true"></span>
</button>
```

### State (added to ModalState)

```ts
kidModeLocked: boolean;  // default false
```

### Behavior

**Locking (single tap when unlocked):**
1. Set `kidModeLocked = true`
2. Add class `kid-mode-locked` to `modal-content`
3. Update button: aria-pressed="true", aria-label="Unlock screen (hold for 1 second)"
4. All buttons/links/segments get `pointer-events: none` via CSS class on parent
5. The lock button itself retains pointer-events

**Unlocking (long-press when locked):**
1. On `pointerdown` on lock button while locked, start a 1-second timer
2. Add class `kid-mode-unlocking` to the button (triggers CSS fill animation)
3. On `pointerup`/`pointerleave`/`pointercancel` before 1s, cancel timer, remove class
4. After 1s: set `kidModeLocked = false`, remove classes, update aria

**Keyboard blocking while locked:**
- The `handleModalKeydown` function checks `kidModeLocked` and returns early
  for all keys except Tab (for accessibility, though most elements are inert)

**Overlay click blocking while locked:**
- The overlay click handler checks `kidModeLocked` and skips `closeVideoModal()`

### CSS

```css
/* Kid mode lock button */
.kid-mode-btn { ... }

/* When locked, disable all modal interactive elements except the lock button */
.kid-mode-locked .modal-actions > *:not(.kid-mode-btn),
.kid-mode-locked .modal-segments,
.kid-mode-locked .modal-close,
.kid-mode-locked .speed-dropdown {
  pointer-events: none;
  opacity: 0.4;
}

/* Unlock progress animation */
.kid-mode-btn.kid-mode-unlocking::after {
  /* animated fill from left to right over 1s */
}
```

## Non-requirements

- No persistence of lock state across sessions or page refreshes
- No backend changes needed
- No new dependencies

## Testing

- Unit test: toggling kid mode state
- Unit test: keyboard handler respects kid mode
- Unit test: overlay click respects kid mode
- Unit test: unlock requires long-press (pointerdown -> timer -> pointerup)
