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
            expect(PLAYBACK_SPEEDS).toEqual([0.8, 0.9, 1.0]);
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
            localStorageMock._setStore({ 'subtitler:playback-speed': '0.8' });
            expect(getSavedSpeed()).toBe(0.8);
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
            saveSpeed(0.8);
            expect(localStorageMock.setItem).toHaveBeenCalledWith('subtitler:playback-speed', '0.8');
        });

        it('should overwrite previous value', () => {
            saveSpeed(0.8);
            saveSpeed(0.9);
            expect(localStorageMock.setItem).toHaveBeenLastCalledWith('subtitler:playback-speed', '0.9');
        });
    });

    describe('setPlaybackSpeed', () => {
        it('should set video playbackRate and save to localStorage', () => {
            const mockVideo = { playbackRate: 1.0 } as HTMLVideoElement;
            setPlaybackSpeed(mockVideo, 0.9);
            expect(mockVideo.playbackRate).toBe(0.9);
            expect(localStorageMock.setItem).toHaveBeenCalledWith('subtitler:playback-speed', '0.9');
        });
    });

    describe('getNextSpeed', () => {
        it('should return next speed in sequence', () => {
            expect(getNextSpeed(0.8)).toBe(0.9);
            expect(getNextSpeed(0.9)).toBe(1.0);
        });

        it('should wrap around from max to min', () => {
            expect(getNextSpeed(1.0)).toBe(0.8);
        });

        it('should return default for invalid speed', () => {
            expect(getNextSpeed(3.0)).toBe(DEFAULT_SPEED);
        });
    });

    describe('getPreviousSpeed', () => {
        it('should return previous speed in sequence', () => {
            expect(getPreviousSpeed(1.0)).toBe(0.9);
            expect(getPreviousSpeed(0.9)).toBe(0.8);
        });

        it('should wrap around from min to max', () => {
            expect(getPreviousSpeed(0.8)).toBe(1.0);
        });

        it('should return default for invalid speed', () => {
            expect(getPreviousSpeed(3.0)).toBe(DEFAULT_SPEED);
        });
    });

    describe('increaseSpeed', () => {
        it('should increase speed by one step', () => {
            expect(increaseSpeed(0.8)).toBe(0.9);
            expect(increaseSpeed(0.9)).toBe(1.0);
        });

        it('should stay at max speed when already at max', () => {
            expect(increaseSpeed(1.0)).toBe(1.0);
        });

        it('should return default for invalid speed', () => {
            expect(increaseSpeed(3.0)).toBe(DEFAULT_SPEED);
        });
    });

    describe('decreaseSpeed', () => {
        it('should decrease speed by one step', () => {
            expect(decreaseSpeed(1.0)).toBe(0.9);
            expect(decreaseSpeed(0.9)).toBe(0.8);
        });

        it('should stay at min speed when already at min', () => {
            expect(decreaseSpeed(0.8)).toBe(0.8);
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
            expect(formatSpeed(0.8)).toBe('0.8x');
            expect(formatSpeed(0.9)).toBe('0.9x');
        });

        it('should handle 1.0 as whole number', () => {
            expect(formatSpeed(1.0)).toBe('1x');
        });
    });
});
