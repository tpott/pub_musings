// Playback speed control utility

// Available speed options for language learning (slower speeds for comprehension)
export const PLAYBACK_SPEEDS = [0.8, 0.9, 1.0] as const;
export type PlaybackSpeed = (typeof PLAYBACK_SPEEDS)[number];

export const DEFAULT_SPEED: PlaybackSpeed = 1.0;

// localStorage key for persistence
const STORAGE_KEY = 'subtitler:playback-speed';

/**
 * Get the saved playback speed from localStorage, or default if not set/invalid
 */
export function getSavedSpeed(): PlaybackSpeed {
    if (typeof localStorage === 'undefined') return DEFAULT_SPEED;

    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
        const speed = parseFloat(saved);
        if (PLAYBACK_SPEEDS.includes(speed as PlaybackSpeed)) {
            return speed as PlaybackSpeed;
        }
    }
    return DEFAULT_SPEED;
}

/**
 * Save playback speed preference to localStorage
 */
export function saveSpeed(speed: PlaybackSpeed): void {
    if (typeof localStorage === 'undefined') return;
    localStorage.setItem(STORAGE_KEY, speed.toString());
}

/**
 * Apply playback speed to a video element and save preference
 */
export function setPlaybackSpeed(video: HTMLVideoElement, speed: PlaybackSpeed): void {
    video.playbackRate = speed;
    saveSpeed(speed);
}

/**
 * Get the next speed in the cycle (for keyboard shortcut)
 */
export function getNextSpeed(currentSpeed: number): PlaybackSpeed {
    const currentIndex = PLAYBACK_SPEEDS.indexOf(currentSpeed as PlaybackSpeed);
    if (currentIndex === -1) return DEFAULT_SPEED;
    const nextIndex = (currentIndex + 1) % PLAYBACK_SPEEDS.length;
    return PLAYBACK_SPEEDS[nextIndex];
}

/**
 * Get the previous speed in the cycle (for keyboard shortcut)
 */
export function getPreviousSpeed(currentSpeed: number): PlaybackSpeed {
    const currentIndex = PLAYBACK_SPEEDS.indexOf(currentSpeed as PlaybackSpeed);
    if (currentIndex === -1) return DEFAULT_SPEED;
    const prevIndex = (currentIndex - 1 + PLAYBACK_SPEEDS.length) % PLAYBACK_SPEEDS.length;
    return PLAYBACK_SPEEDS[prevIndex];
}

/**
 * Increase speed to next step (for ] key)
 */
export function increaseSpeed(currentSpeed: number): PlaybackSpeed {
    const currentIndex = PLAYBACK_SPEEDS.indexOf(currentSpeed as PlaybackSpeed);
    if (currentIndex === -1) return DEFAULT_SPEED;
    // Cap at max speed
    const nextIndex = Math.min(currentIndex + 1, PLAYBACK_SPEEDS.length - 1);
    return PLAYBACK_SPEEDS[nextIndex];
}

/**
 * Decrease speed to previous step (for [ key)
 */
export function decreaseSpeed(currentSpeed: number): PlaybackSpeed {
    const currentIndex = PLAYBACK_SPEEDS.indexOf(currentSpeed as PlaybackSpeed);
    if (currentIndex === -1) return DEFAULT_SPEED;
    // Cap at min speed
    const prevIndex = Math.max(currentIndex - 1, 0);
    return PLAYBACK_SPEEDS[prevIndex];
}

/**
 * Format speed for display (e.g., "1x", "0.75x", "1.5x")
 */
export function formatSpeed(speed: number): string {
    // Show whole numbers without decimal (1x, 2x)
    // Show fractional numbers with appropriate decimals (0.5x, 0.75x, 1.25x, 1.5x)
    if (speed === Math.floor(speed)) {
        return `${speed}x`;
    }
    // For 0.25 increments, show up to 2 decimal places but strip trailing zeros
    const formatted = speed.toFixed(2).replace(/\.?0+$/, '');
    return `${formatted}x`;
}
