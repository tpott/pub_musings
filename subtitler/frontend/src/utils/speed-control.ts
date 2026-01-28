// Video player speed control UI utility
// Shared logic for speed dropdown and indicator across video player instances

import {
    PLAYBACK_SPEEDS,
    getSavedSpeed,
    setPlaybackSpeed,
    increaseSpeed,
    decreaseSpeed,
    formatSpeed,
    type PlaybackSpeed
} from './playback-speed';

export interface SpeedControlElements {
    video: HTMLVideoElement;
    speedBtn: HTMLButtonElement;
    speedOptions: HTMLElement;
    speedIndicator: HTMLElement;
}

export interface SpeedControlState {
    menuOpen: boolean;
    indicatorTimeout: number | null;
    focusedIndex: number;
}

/**
 * Create a new speed control instance for a video player
 */
export function createSpeedControl(elements: SpeedControlElements): {
    state: SpeedControlState;
    updateUI: (speed: number) => void;
    showIndicator: (speed: number) => void;
    toggleMenu: () => void;
    closeMenu: () => void;
    initialize: () => void;
    handleSpeedChange: (newSpeed: PlaybackSpeed) => void;
    handleKeyboardShortcut: (key: string) => boolean;
    cleanup: () => void;
} {
    const { video, speedBtn, speedOptions, speedIndicator } = elements;

    const state: SpeedControlState = {
        menuOpen: false,
        indicatorTimeout: null,
        focusedIndex: -1
    };

    // AbortController for cleanup
    const abortController = new AbortController();
    const signal = abortController.signal;

    /**
     * Get all speed option elements
     */
    function getOptions(): HTMLElement[] {
        return Array.from(speedOptions.querySelectorAll('.speed-option')) as HTMLElement[];
    }

    /**
     * Update focus styling on options
     */
    function updateFocus(index: number) {
        const options = getOptions();
        options.forEach((opt, i) => {
            opt.classList.toggle('focused', i === index);
            opt.setAttribute('tabindex', i === index ? '0' : '-1');
        });
        if (index >= 0 && index < options.length) {
            options[index].focus();
        }
    }

    /**
     * Update the speed button and options UI to reflect current speed
     */
    function updateUI(speed: number) {
        speedBtn.textContent = formatSpeed(speed);
        speedOptions.querySelectorAll('.speed-option').forEach((opt) => {
            const optEl = opt as HTMLElement;
            const optSpeed = parseFloat(optEl.dataset.speed || '1');
            optEl.classList.toggle('active', optSpeed === speed);
        });
    }

    /**
     * Show a temporary speed indicator overlay
     */
    function showIndicator(speed: number) {
        speedIndicator.textContent = formatSpeed(speed);
        speedIndicator.classList.add('visible');

        if (state.indicatorTimeout) {
            clearTimeout(state.indicatorTimeout);
        }

        state.indicatorTimeout = window.setTimeout(() => {
            speedIndicator.classList.remove('visible');
            state.indicatorTimeout = null;
        }, 800);
    }

    /**
     * Toggle the speed menu dropdown open/closed
     */
    function toggleMenu() {
        state.menuOpen = !state.menuOpen;
        speedOptions.style.display = state.menuOpen ? 'block' : 'none';
        speedBtn.setAttribute('aria-expanded', state.menuOpen.toString());

        if (state.menuOpen) {
            // Find the currently active option and focus it
            const options = getOptions();
            const activeIndex = options.findIndex(opt => opt.classList.contains('active'));
            state.focusedIndex = activeIndex >= 0 ? activeIndex : 0;
            updateFocus(state.focusedIndex);
        } else {
            state.focusedIndex = -1;
        }
    }

    /**
     * Close the speed menu dropdown
     */
    function closeMenu() {
        state.menuOpen = false;
        speedOptions.style.display = 'none';
        speedBtn.setAttribute('aria-expanded', 'false');
        state.focusedIndex = -1;
        speedBtn.focus(); // Return focus to button
    }

    /**
     * Handle speed change - update video, UI, and show indicator
     */
    function handleSpeedChange(newSpeed: PlaybackSpeed) {
        setPlaybackSpeed(video, newSpeed);
        updateUI(newSpeed);
        showIndicator(newSpeed);
        closeMenu();
    }

    /**
     * Handle keyboard shortcuts for speed control
     * Returns true if the key was handled
     */
    function handleKeyboardShortcut(key: string): boolean {
        if (key === '[') {
            const currentSpeed = video.playbackRate;
            const newSpeed = decreaseSpeed(currentSpeed);
            if (newSpeed !== currentSpeed) {
                handleSpeedChange(newSpeed);
            }
            return true;
        }

        if (key === ']') {
            const currentSpeed = video.playbackRate;
            const newSpeed = increaseSpeed(currentSpeed);
            if (newSpeed !== currentSpeed) {
                handleSpeedChange(newSpeed);
            }
            return true;
        }

        return false;
    }

    /**
     * Initialize speed control with saved preference and event handlers
     */
    function initialize() {
        const savedSpeed = getSavedSpeed();
        setPlaybackSpeed(video, savedSpeed);
        updateUI(savedSpeed);

        // Speed button click handler
        speedBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            toggleMenu();
        }, { signal });

        // Speed option selection handler
        speedOptions.addEventListener('click', (e) => {
            const target = e.target as HTMLElement;
            const option = target.closest('.speed-option') as HTMLElement | null;
            if (option) {
                const speed = parseFloat(option.dataset.speed || '1');
                if (PLAYBACK_SPEEDS.includes(speed as PlaybackSpeed)) {
                    handleSpeedChange(speed as PlaybackSpeed);
                }
            }
        }, { signal });

        // Keyboard navigation handler for the dropdown menu
        speedOptions.addEventListener('keydown', (e) => {
            const options = getOptions();
            if (options.length === 0) return;

            switch (e.key) {
                case 'ArrowDown':
                    e.preventDefault();
                    state.focusedIndex = (state.focusedIndex + 1) % options.length;
                    updateFocus(state.focusedIndex);
                    break;

                case 'ArrowUp':
                    e.preventDefault();
                    state.focusedIndex = (state.focusedIndex - 1 + options.length) % options.length;
                    updateFocus(state.focusedIndex);
                    break;

                case 'Enter':
                case ' ':
                    e.preventDefault();
                    if (state.focusedIndex >= 0 && state.focusedIndex < options.length) {
                        const option = options[state.focusedIndex];
                        const speed = parseFloat(option.dataset.speed || '1');
                        if (PLAYBACK_SPEEDS.includes(speed as PlaybackSpeed)) {
                            handleSpeedChange(speed as PlaybackSpeed);
                        }
                    }
                    break;

                case 'Escape':
                    e.preventDefault();
                    closeMenu();
                    break;

                case 'Tab':
                    // Close menu on Tab to allow natural focus flow
                    closeMenu();
                    break;
            }
        }, { signal });

        // Also allow keyboard navigation when speed button has focus
        speedBtn.addEventListener('keydown', (e) => {
            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                e.preventDefault();
                if (!state.menuOpen) {
                    toggleMenu();
                }
            }
        }, { signal });
    }

    /**
     * Cleanup event listeners and timeouts
     */
    function cleanup() {
        abortController.abort();
        if (state.indicatorTimeout) {
            clearTimeout(state.indicatorTimeout);
            state.indicatorTimeout = null;
        }
    }

    return {
        state,
        updateUI,
        showIndicator,
        toggleMenu,
        closeMenu,
        initialize,
        handleSpeedChange,
        handleKeyboardShortcut,
        cleanup
    };
}
