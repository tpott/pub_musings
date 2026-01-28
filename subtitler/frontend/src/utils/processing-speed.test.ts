import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import {
  recordProcessingTime,
  getAverageSpeedRatio,
  estimateTotalTime,
  estimateTimeRemaining,
  formatTimeRemaining,
  hasHistoricalData,
  clearHistory
} from './processing-speed';

describe('processing-speed utility', () => {
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
      clear: () => {
        store = {};
      },
      _setStore: (newStore: Record<string, string>) => {
        store = newStore;
      },
      _getStore: () => store
    };
  })();

  beforeEach(() => {
    localStorageMock.clear();
    vi.stubGlobal('localStorage', localStorageMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe('recordProcessingTime', () => {
    it('should record valid processing times', () => {
      recordProcessingTime(60, 30); // 60 second video, 30 seconds to process
      expect(hasHistoricalData()).toBe(true);
    });

    it('should not record invalid data (zero duration)', () => {
      recordProcessingTime(0, 30);
      expect(hasHistoricalData()).toBe(false);
    });

    it('should not record invalid data (zero processing time)', () => {
      recordProcessingTime(60, 0);
      expect(hasHistoricalData()).toBe(false);
    });

    it('should not record invalid data (negative values)', () => {
      recordProcessingTime(-60, 30);
      recordProcessingTime(60, -30);
      expect(hasHistoricalData()).toBe(false);
    });

    it('should keep only the most recent records', () => {
      // Record more than MAX_HISTORY_SIZE (10) entries
      for (let i = 0; i < 15; i++) {
        recordProcessingTime(60, 30 + i);
      }

      const stored = JSON.parse(localStorageMock._getStore()['subtitler:processing_speed_history']);
      expect(stored.records.length).toBe(10);
    });
  });

  describe('getAverageSpeedRatio', () => {
    it('should return null with no history', () => {
      expect(getAverageSpeedRatio()).toBeNull();
    });

    it('should calculate weighted average ratio', () => {
      // Video duration: 100s, processing time: 50s -> ratio 0.5
      recordProcessingTime(100, 50);
      // Video duration: 100s, processing time: 100s -> ratio 1.0
      recordProcessingTime(100, 100);

      // Weighted average: (0.5 * 1 + 1.0 * 2) / 3 = 2.5 / 3 = 0.833...
      const ratio = getAverageSpeedRatio();
      expect(ratio).toBeCloseTo(0.833, 2);
    });
  });

  describe('estimateTotalTime', () => {
    it('should return null with no history', () => {
      expect(estimateTotalTime(120)).toBeNull();
    });

    it('should estimate based on historical ratio', () => {
      // Record a 1:2 ratio (processing takes half of video duration)
      recordProcessingTime(100, 50);

      // For 120 second video, expect ~60 seconds
      const estimate = estimateTotalTime(120);
      expect(estimate).toBeCloseTo(60, 1);
    });
  });

  describe('estimateTimeRemaining', () => {
    it('should return historical estimate at 0% progress', () => {
      recordProcessingTime(100, 50); // 0.5 ratio
      const remaining = estimateTimeRemaining(120, 0, 0);
      expect(remaining).toBeCloseTo(60, 1); // 120 * 0.5 = 60
    });

    it('should return 0 at 100% progress', () => {
      recordProcessingTime(100, 50);
      const remaining = estimateTimeRemaining(120, 100, 60);
      expect(remaining).toBe(0);
    });

    it('should estimate based on current rate when progress > 0', () => {
      // No history, 50% progress after 30 seconds
      // Estimated total: 60 seconds, remaining: 30 seconds
      const remaining = estimateTimeRemaining(120, 50, 30);
      expect(remaining).toBeCloseTo(30, 1);
    });

    it('should blend historical and current rate for early progress', () => {
      recordProcessingTime(100, 50); // 0.5 ratio
      // At 25% progress with 20 seconds elapsed
      // Current rate: 20 / 0.25 = 80 total
      // Historical: 120 * 0.5 = 60 total
      // Blend: mix historical (75%) and current (25%)
      const remaining = estimateTimeRemaining(120, 25, 20);
      expect(remaining).toBeDefined();
      expect(remaining!).toBeGreaterThan(0);
    });

    it('should return null if no progress and no history', () => {
      const remaining = estimateTimeRemaining(120, 0, 0);
      expect(remaining).toBeNull();
    });
  });

  describe('formatTimeRemaining', () => {
    it('should return "Estimating..." for null', () => {
      expect(formatTimeRemaining(null)).toBe('Estimating...');
    });

    it('should return "Almost done..." for negative', () => {
      expect(formatTimeRemaining(-5)).toBe('Almost done...');
    });

    it('should format seconds', () => {
      expect(formatTimeRemaining(1)).toBe('~1 second remaining');
      expect(formatTimeRemaining(30)).toBe('~30 seconds remaining');
      expect(formatTimeRemaining(59)).toBe('~59 seconds remaining');
    });

    it('should format minutes', () => {
      expect(formatTimeRemaining(60)).toBe('~1 minute remaining');
      expect(formatTimeRemaining(120)).toBe('~2 minutes remaining');
      expect(formatTimeRemaining(90)).toBe('~1m 30s remaining');
    });

    it('should format hours', () => {
      expect(formatTimeRemaining(3600)).toBe('~1h 0m remaining');
      expect(formatTimeRemaining(3660)).toBe('~1h 1m remaining');
      expect(formatTimeRemaining(7320)).toBe('~2h 2m remaining');
    });
  });

  describe('hasHistoricalData', () => {
    it('should return false with no data', () => {
      expect(hasHistoricalData()).toBe(false);
    });

    it('should return true after recording', () => {
      recordProcessingTime(60, 30);
      expect(hasHistoricalData()).toBe(true);
    });
  });

  describe('clearHistory', () => {
    it('should clear all history', () => {
      recordProcessingTime(60, 30);
      expect(hasHistoricalData()).toBe(true);

      clearHistory();
      expect(hasHistoricalData()).toBe(false);
    });
  });

  describe('edge cases', () => {
    it('should handle corrupted localStorage gracefully and clear it', () => {
      localStorageMock._setStore({ 'subtitler:processing_speed_history': 'not valid json' });
      expect(getAverageSpeedRatio()).toBeNull();
      expect(hasHistoricalData()).toBe(false);
      // Verify corrupted data was cleared
      expect(localStorageMock.getItem('subtitler:processing_speed_history')).toBeNull();
    });

    it('should handle malformed history object and clear it', () => {
      localStorageMock._setStore({
        'subtitler:processing_speed_history': JSON.stringify({ notRecords: true })
      });
      expect(getAverageSpeedRatio()).toBeNull();
      // Verify malformed data was cleared
      expect(localStorageMock.getItem('subtitler:processing_speed_history')).toBeNull();
    });

    it('should handle very small progress percentages', () => {
      recordProcessingTime(100, 50);
      const remaining = estimateTimeRemaining(120, 0.1, 0.05);
      expect(remaining).toBeGreaterThan(0);
    });
  });

  describe('localStorage quota exceeded handling', () => {
    it('should handle QuotaExceededError gracefully when recording', () => {
      const quotaError = new DOMException('Quota exceeded', 'QuotaExceededError');
      localStorageMock.setItem = vi.fn(() => {
        throw quotaError;
      });

      // Should not throw, just log the error
      expect(() => recordProcessingTime(60, 30)).not.toThrow();
    });

    it('should handle QuotaExceededError gracefully when clearing history', () => {
      const quotaError = new DOMException('Quota exceeded', 'QuotaExceededError');
      localStorageMock.removeItem = vi.fn(() => {
        throw quotaError;
      });

      // Should not throw, just log the error
      expect(() => clearHistory()).not.toThrow();
    });

    it('should handle localStorage unavailable for getItem', () => {
      localStorageMock.getItem = vi.fn(() => {
        throw new Error('localStorage unavailable');
      });

      // Should return null gracefully
      expect(getAverageSpeedRatio()).toBeNull();
      expect(hasHistoricalData()).toBe(false);
    });

    it('should not crash when localStorage.removeItem fails during corrupted data cleanup', () => {
      // Set up corrupted data
      localStorageMock._setStore({ 'subtitler:processing_speed_history': 'invalid json' });

      // Make removeItem throw when trying to clean up
      const originalRemoveItem = localStorageMock.removeItem;
      localStorageMock.removeItem = vi.fn(() => {
        throw new Error('Cannot remove');
      });

      // Should still return gracefully without crashing
      expect(getAverageSpeedRatio()).toBeNull();

      // Restore
      localStorageMock.removeItem = originalRemoveItem;
    });
  });
});
