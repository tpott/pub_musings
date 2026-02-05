import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  checkTTSAvailability,
  generatePhrase,
  synthesizeSpeech,
  speakSubject,
  resetTTSAvailability,
} from './text-to-speech';
import { ApiError } from './errors';

describe('text-to-speech', () => {
  beforeEach(() => {
    resetTTSAvailability();
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('generatePhrase', () => {
    it('generates phrase for cat', () => {
      expect(generatePhrase('cat')).toBe('Here is a cat!');
    });

    it('generates phrase for dog', () => {
      expect(generatePhrase('dog')).toBe('Here is a dog!');
    });

    it('handles empty string', () => {
      expect(generatePhrase('')).toBe('Here is a !');
    });
  });

  describe('checkTTSAvailability', () => {
    it('returns true when /api/speak returns 200', async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 200,
        ok: true,
      });

      const result = await checkTTSAvailability();
      expect(result).toBe(true);
    });

    it('returns false when /api/speak returns 404', async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 404,
        ok: false,
      });

      const result = await checkTTSAvailability();
      expect(result).toBe(false);
    });

    it('returns false on network error', async () => {
      global.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

      const result = await checkTTSAvailability();
      expect(result).toBe(false);
    });

    it('caches the result', async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 200,
        ok: true,
      });

      await checkTTSAvailability();
      await checkTTSAvailability();

      expect(global.fetch).toHaveBeenCalledTimes(1);
    });
  });

  describe('synthesizeSpeech', () => {
    it('throws error for empty text', async () => {
      await expect(synthesizeSpeech('')).rejects.toThrow('Text is required');
    });

    it('throws error for text too long', async () => {
      const longText = 'a'.repeat(257);
      await expect(synthesizeSpeech(longText)).rejects.toThrow('max 256 characters');
    });

    it('returns null when TTS not configured (404)', async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 404,
        ok: false,
      });

      const result = await synthesizeSpeech('Hello');
      expect(result).toBeNull();
    });

    it('throws ApiError on server error', async () => {
      // Return 400 (not in retry list) to avoid retry delays
      global.fetch = vi.fn().mockResolvedValue({
        status: 400,
        ok: false,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: () => Promise.resolve({ error: 'TTS server error' }),
      });

      await expect(synthesizeSpeech('Hello')).rejects.toThrow(ApiError);
    });

    it('returns Audio element on success', async () => {
      const mockBlob = new Blob(['fake-audio'], { type: 'audio/wav' });

      global.fetch = vi.fn().mockResolvedValue({
        status: 200,
        ok: true,
        blob: () => Promise.resolve(mockBlob),
      });

      // Mock URL.createObjectURL and URL.revokeObjectURL
      const mockUrl = 'blob:test-url';
      URL.createObjectURL = vi.fn().mockReturnValue(mockUrl);
      URL.revokeObjectURL = vi.fn();

      // Mock Audio constructor
      const mockAudioInstance = {
        addEventListener: vi.fn(),
        play: vi.fn().mockResolvedValue(undefined),
      };
      global.Audio = vi.fn().mockImplementation(() => mockAudioInstance) as unknown as typeof Audio;

      const result = await synthesizeSpeech('Hello');

      expect(result).toBe(mockAudioInstance);
      expect(URL.createObjectURL).toHaveBeenCalledWith(mockBlob);
      expect(global.Audio).toHaveBeenCalledWith(mockUrl);
      expect(mockAudioInstance.addEventListener).toHaveBeenCalledWith(
        'ended',
        expect.any(Function),
        { once: true }
      );
      expect(mockAudioInstance.addEventListener).toHaveBeenCalledWith(
        'error',
        expect.any(Function),
        { once: true }
      );
    });

    it('cleans up blob URL on audio ended', async () => {
      const mockBlob = new Blob(['fake-audio'], { type: 'audio/wav' });

      global.fetch = vi.fn().mockResolvedValue({
        status: 200,
        ok: true,
        blob: () => Promise.resolve(mockBlob),
      });

      const mockUrl = 'blob:test-url';
      URL.createObjectURL = vi.fn().mockReturnValue(mockUrl);
      URL.revokeObjectURL = vi.fn();

      let endedCallback: () => void = () => {};
      const mockAudioInstance = {
        addEventListener: vi.fn((event: string, callback: () => void) => {
          if (event === 'ended') {
            endedCallback = callback;
          }
        }),
        play: vi.fn().mockResolvedValue(undefined),
      };
      global.Audio = vi.fn().mockImplementation(() => mockAudioInstance) as unknown as typeof Audio;

      await synthesizeSpeech('Hello');

      // Trigger the ended callback
      endedCallback();

      expect(URL.revokeObjectURL).toHaveBeenCalledWith(mockUrl);
    });

    it('throws ApiError on network error', async () => {
      // Mock fetch to always fail - fetchWithRetry will exhaust retries
      // Use shorter retry options via direct import
      global.fetch = vi.fn().mockRejectedValue(new Error('Network failed'));

      // Reset TTS availability to ensure fresh state
      resetTTSAvailability();

      try {
        await synthesizeSpeech('Hello');
        expect.fail('Should have thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        expect((error as ApiError).type).toBe('network');
      }
    }, 15000); // Increase timeout to handle retries
  });

  describe('speakSubject', () => {
    it('generates phrase and synthesizes speech', async () => {
      const mockBlob = new Blob(['fake-audio'], { type: 'audio/wav' });

      global.fetch = vi.fn().mockResolvedValue({
        status: 200,
        ok: true,
        blob: () => Promise.resolve(mockBlob),
      });

      URL.createObjectURL = vi.fn().mockReturnValue('blob:test');
      URL.revokeObjectURL = vi.fn();

      const mockAudioInstance = {
        addEventListener: vi.fn(),
        play: vi.fn().mockResolvedValue(undefined),
      };
      global.Audio = vi.fn().mockImplementation(() => mockAudioInstance) as unknown as typeof Audio;

      const result = await speakSubject('cat');

      expect(result).toBe(mockAudioInstance);
      expect(mockAudioInstance.play).toHaveBeenCalled();

      // Verify the correct phrase was sent
      expect(global.fetch).toHaveBeenCalledWith(
        '/api/speak',
        expect.objectContaining({
          body: JSON.stringify({ text: 'Here is a cat!' }),
        })
      );
    });

    it('returns null when TTS unavailable', async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 404,
        ok: false,
      });

      const result = await speakSubject('cat');
      expect(result).toBeNull();
    });

    it('handles autoplay blocked gracefully', async () => {
      const mockBlob = new Blob(['fake-audio'], { type: 'audio/wav' });
      const consoleWarn = vi.spyOn(console, 'warn').mockImplementation(() => {});

      global.fetch = vi.fn().mockResolvedValue({
        status: 200,
        ok: true,
        blob: () => Promise.resolve(mockBlob),
      });

      URL.createObjectURL = vi.fn().mockReturnValue('blob:test');
      URL.revokeObjectURL = vi.fn();

      const mockAudioInstance = {
        addEventListener: vi.fn(),
        play: vi.fn().mockRejectedValue(new Error('Autoplay blocked')),
      };
      global.Audio = vi.fn().mockImplementation(() => mockAudioInstance) as unknown as typeof Audio;

      // Should not throw despite autoplay being blocked
      const result = await speakSubject('cat');

      expect(result).toBe(mockAudioInstance);
      // Wait for the async play promise to settle
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(consoleWarn).toHaveBeenCalledWith('[WARN] TTS autoplay blocked:', expect.any(Error));

      consoleWarn.mockRestore();
    });
  });

  describe('resetTTSAvailability', () => {
    it('resets cached availability', async () => {
      // First call - cache result
      global.fetch = vi.fn().mockResolvedValue({ status: 404, ok: false });
      await checkTTSAvailability();
      expect(await checkTTSAvailability()).toBe(false);

      // Reset
      resetTTSAvailability();

      // Second call - should check again
      global.fetch = vi.fn().mockResolvedValue({ status: 200, ok: true });
      expect(await checkTTSAvailability()).toBe(true);
      expect(global.fetch).toHaveBeenCalledTimes(1);
    });
  });
});
