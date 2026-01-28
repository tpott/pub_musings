# Video Scroll Fix Specification

**Status: Resolved** (Task 156)

This issue was fixed in Task 156. The spec is retained as a reference for the bug pattern and solution.

---

## Problem

When watching a video, the page scrolls down causing the video to go out of view. This is the **THIRD** time this issue has been reported.

## Root Cause Analysis

The issue is caused by using `scrollIntoView()` on subtitle segment elements during video playback:

```javascript
// PROBLEMATIC CODE (upload.astro)
el.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
```

While `scrollIntoView` with `block: 'nearest'` sounds like it should only scroll if necessary, **it can still scroll the entire page** if the element's container doesn't fully contain the element in the viewport.

### How scrollIntoView Works

`scrollIntoView()` scrolls **all ancestor scrollable containers**, not just the immediate parent. So when:

1. The video is at the top of the page
2. The segments list is below the video
3. A segment at the bottom of the list becomes active

The browser can scroll the **entire page** to bring that segment into view, causing the video to scroll up and out of the viewport.

## Solution

Replace `scrollIntoView()` with manual container scrolling that ONLY scrolls within the segments container:

```javascript
// CORRECT APPROACH (from videos.astro)
const container = segments;
const segmentEl = el as HTMLElement;
const containerRect = container.getBoundingClientRect();
const segmentRect = segmentEl.getBoundingClientRect();

// Only scroll if segment is outside the visible area of the container
if (segmentRect.top < containerRect.top) {
    container.scrollTop -= (containerRect.top - segmentRect.top);
} else if (segmentRect.bottom > containerRect.bottom) {
    container.scrollTop += (segmentRect.bottom - containerRect.bottom);
}
```

### Why This Works

1. We compare the segment's position to the **container's** visible area (not the viewport)
2. We only adjust `container.scrollTop` - this scrolls **within** the container
3. The page itself never scrolls, so the video stays visible

## Files to Update

### upload.astro

Two locations need fixing:

1. **`navigateToSegment()` function** (around line 2544):
   - Called when user presses Tab/Shift+Tab to navigate segments
   - Currently uses `scrollIntoView({ behavior: 'smooth', block: 'nearest' })`

2. **`updateCurrentSubtitle()` function** (around line 2717):
   - Called on every video `timeupdate` event during playback
   - Currently uses `scrollIntoView({ behavior: 'smooth', block: 'nearest' })`

### videos.astro

Already fixed correctly (lines 1088-1099). No changes needed.

## Helper Function

Create a reusable helper to avoid code duplication:

```javascript
// Scroll element into view within a container only (don't scroll the page)
function scrollIntoContainerView(container: HTMLElement, element: HTMLElement) {
    const containerRect = container.getBoundingClientRect();
    const elementRect = element.getBoundingClientRect();

    if (elementRect.top < containerRect.top) {
        container.scrollTop -= (containerRect.top - elementRect.top);
    } else if (elementRect.bottom > containerRect.bottom) {
        container.scrollTop += (elementRect.bottom - containerRect.bottom);
    }
}
```

## Testing Strategy

### Manual Testing

1. Upload a video with many subtitle segments (10+)
2. Play the video from the beginning
3. Watch through multiple segments
4. **Verify**: Video should remain visible at all times
5. Use Tab/Shift+Tab to navigate segments
6. **Verify**: Video should remain visible at all times

### Automated Testing (E2E)

Add Playwright tests that:
1. Upload a video
2. Wait for transcription
3. Start playback
4. Assert video element remains in viewport throughout playback
5. Use keyboard navigation
6. Assert video element still visible

## Prevention

### Why This Keeps Regressing

1. `scrollIntoView` is the "obvious" solution when you want to scroll to an element
2. The `block: 'nearest'` option sounds like it should prevent page scrolling
3. The bug only manifests when:
   - The segments list is long enough to have internal scrolling
   - The active segment is far from the current scroll position
   - The page layout has the video above the segments

### Prevention Measures

1. **Never use `scrollIntoView()` for elements inside scrollable containers**
2. Always use manual container scrolling (adjust `container.scrollTop`)
3. Add a comment explaining WHY we don't use `scrollIntoView`
4. Add E2E tests that verify video visibility during playback
5. Consider creating a shared utility function to enforce this pattern
