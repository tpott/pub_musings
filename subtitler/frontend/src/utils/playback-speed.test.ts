import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
    PLAYBACK_SPEEDS,
    DEFAULT_SPEED,
    getSavedSpeed,
    saveSpeed,
    setPlaybackSpeed,
    getNextSpeed,
    getPreviousSpeed,
    increaseSpeed,
    decreaseSpeed,
    formatSpeed,
} from './playback-speed';

// Mock localStorage
const localStorageMock = (() => {
    let store: Record<string, string> = {};
    return {
        getItem: vi.fn((key: string) => store[key] || null),
        setItem: vi.fn((key: string, value: string) => {
            store[key] = value;
        }),
        removeItem: vi.fn((key: string) => {
            delete store[key];
        }),
        clear: vi.fn(() => {
            store = {};
        }),
        _setStore: (newStore: Record<string, string>) => {
            store = { ...newStore };
        },
    };
})();

describe('playback-speed utility', () => {
    beforeEach(() => {
        // Setup localStorage mock
        vi.stubGlobal('localStorage', localStorageMock);
        localStorageMock.clear();
        vi.clearAllMocks();
    });

    describe('PLAYBACK_SPEEDS constant', () => {
        it('should contain expected speed values', () => {
            expect(PLAYBACK_SPEEDS).toEqual([0.5, 0.75, 1.0, 1.25, 1.5, 2.0]);
        });

        it('should have DEFAULT_SPEED as 1.0', () => {
            expect(DEFAULT_SPEED).toBe(1.0);
        });
    });

    describe('getSavedSpeed', () => {
        it('should return default when localStorage is empty', () => {
            expect(getSavedSpeed()).toBe(DEFAULT_SPEED);
        });

        it('should return saved value when valid', () => {
            localStorageMock._setStore({ 'subtitler:playback-speed': '0.75' });
            expect(getSavedSpeed()).toBe(0.75);
        });

        it('should return saved value for all valid speeds', () => {
            for (const speed of PLAYBACK_SPEEDS) {
                localStorageMock._setStore({ 'subtitler:playback-speed': speed.toString() });
                expect(getSavedSpeed()).toBe(speed);
            }
        });

        it('should return default for invalid saved value', () => {
            localStorageMock._setStore({ 'subtitler:playback-speed': '3.0' });
            expect(getSavedSpeed()).toBe(DEFAULT_SPEED);
        });

        it('should return default for non-numeric value', () => {
            localStorageMock._setStore({ 'subtitler:playback-speed': 'fast' });
            expect(getSavedSpeed()).toBe(DEFAULT_SPEED);
        });

        it('should return default for negative value', () => {
            localStorageMock._setStore({ 'subtitler:playback-speed': '-1.0' });
            expect(getSavedSpeed()).toBe(DEFAULT_SPEED);
        });
    });

    describe('saveSpeed', () => {
        it('should save speed to localStorage', () => {
            saveSpeed(0.75);
            expect(localStorageMock.setItem).toHaveBeenCalledWith('subtitler:playback-speed', '0.75');
        });

        it('should overwrite previous value', () => {
            saveSpeed(0.75);
            saveSpeed(1.25);
            expect(localStorageMock.setItem).toHaveBeenLastCalledWith('subtitler:playback-speed', '1.25');
        });
    });

    describe('setPlaybackSpeed', () => {
        it('should set video playbackRate and save to localStorage', () => {
            const mockVideo = { playbackRate: 1.0 } as HTMLVideoElement;
            setPlaybackSpeed(mockVideo, 1.25);
            expect(mockVideo.playbackRate).toBe(1.25);
            expect(localStorageMock.setItem).toHaveBeenCalledWith('subtitler:playback-speed', '1.25');
        });
    });

    describe('getNextSpeed', () => {
        it('should return next speed in sequence', () => {
            expect(getNextSpeed(0.5)).toBe(0.75);
            expect(getNextSpeed(0.75)).toBe(1.0);
            expect(getNextSpeed(1.0)).toBe(1.25);
            expect(getNextSpeed(1.25)).toBe(1.5);
            expect(getNextSpeed(1.5)).toBe(2.0);
        });

        it('should wrap around from max to min', () => {
            expect(getNextSpeed(2.0)).toBe(0.5);
        });

        it('should return default for invalid speed', () => {
            expect(getNextSpeed(3.0)).toBe(DEFAULT_SPEED);
        });
    });

    describe('getPreviousSpeed', () => {
        it('should return previous speed in sequence', () => {
            expect(getPreviousSpeed(2.0)).toBe(1.5);
            expect(getPreviousSpeed(1.5)).toBe(1.25);
            expect(getPreviousSpeed(1.25)).toBe(1.0);
            expect(getPreviousSpeed(1.0)).toBe(0.75);
            expect(getPreviousSpeed(0.75)).toBe(0.5);
        });

        it('should wrap around from min to max', () => {
            expect(getPreviousSpeed(0.5)).toBe(2.0);
        });

        it('should return default for invalid speed', () => {
            expect(getPreviousSpeed(3.0)).toBe(DEFAULT_SPEED);
        });
    });

    describe('increaseSpeed', () => {
        it('should increase speed by one step', () => {
            expect(increaseSpeed(0.5)).toBe(0.75);
            expect(increaseSpeed(0.75)).toBe(1.0);
            expect(increaseSpeed(1.0)).toBe(1.25);
            expect(increaseSpeed(1.5)).toBe(2.0);
        });

        it('should stay at max speed when already at max', () => {
            expect(increaseSpeed(2.0)).toBe(2.0);
        });

        it('should return default for invalid speed', () => {
            expect(increaseSpeed(3.0)).toBe(DEFAULT_SPEED);
        });
    });

    describe('decreaseSpeed', () => {
        it('should decrease speed by one step', () => {
            expect(decreaseSpeed(2.0)).toBe(1.5);
            expect(decreaseSpeed(1.0)).toBe(0.75);
            expect(decreaseSpeed(0.75)).toBe(0.5);
        });

        it('should stay at min speed when already at min', () => {
            expect(decreaseSpeed(0.5)).toBe(0.5);
        });

        it('should return default for invalid speed', () => {
            expect(decreaseSpeed(3.0)).toBe(DEFAULT_SPEED);
        });
    });

    describe('formatSpeed', () => {
        it('should format whole numbers without decimal', () => {
            expect(formatSpeed(1)).toBe('1x');
            expect(formatSpeed(2)).toBe('2x');
        });

        it('should format fractional speeds correctly', () => {
            expect(formatSpeed(0.5)).toBe('0.5x');
            expect(formatSpeed(0.75)).toBe('0.75x');
            expect(formatSpeed(1.25)).toBe('1.25x');
            expect(formatSpeed(1.5)).toBe('1.5x');
        });

        it('should handle 1.0 as whole number', () => {
            expect(formatSpeed(1.0)).toBe('1x');
        });
    });
});
