import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  getOrCreateSessionId,
  submitFeedback,
  type FeedbackData,
} from './feedback';

describe('feedback', () => {
  describe('getOrCreateSessionId', () => {
    beforeEach(() => {
      localStorage.clear();
    });

    afterEach(() => {
      localStorage.clear();
    });

    it('creates a new session ID if none exists', () => {
      const sessionId = getOrCreateSessionId();
      expect(sessionId).toBeTruthy();
      expect(typeof sessionId).toBe('string');
      expect(sessionId.length).toBeGreaterThan(0);
    });

    it('returns the same session ID on subsequent calls', () => {
      const sessionId1 = getOrCreateSessionId();
      const sessionId2 = getOrCreateSessionId();
      expect(sessionId1).toBe(sessionId2);
    });

    it('stores session ID in localStorage', () => {
      const sessionId = getOrCreateSessionId();
      expect(localStorage.getItem('peekaboo_session_id')).toBe(sessionId);
    });

    it('returns existing session ID from localStorage', () => {
      const existingId = 'existing-session-123';
      localStorage.setItem('peekaboo_session_id', existingId);

      const sessionId = getOrCreateSessionId();
      expect(sessionId).toBe(existingId);
    });

    it('generates UUID-like format', () => {
      const sessionId = getOrCreateSessionId();
      // UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
      expect(sessionId).toMatch(
        /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
      );
    });
  });

  describe('submitFeedback', () => {
    let fetchSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      fetchSpy = vi.spyOn(globalThis, 'fetch');
    });

    afterEach(() => {
      fetchSpy.mockRestore();
    });

    it('sends feedback to the API', async () => {
      const mockResponse = { id: 'feedback_123', status: 'ok' };
      fetchSpy.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      } as Response);

      const data: FeedbackData = {
        type: 'general',
        message: 'Great app!',
        context: {
          sessionId: 'session-123',
          pageUrl: '/',
        },
      };

      const result = await submitFeedback(data);

      expect(fetchSpy).toHaveBeenCalledWith('/api/feedback', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          type: 'general',
          message: 'Great app!',
          context: {
            session_id: 'session-123',
            page_url: '/',
          },
        }),
      });
      expect(result).toEqual(mockResponse);
    });

    it('sends feedback with all optional fields', async () => {
      const mockResponse = { id: 'feedback_456', status: 'ok' };
      fetchSpy.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      } as Response);

      const data: FeedbackData = {
        type: 'bug',
        message: 'Something broke',
        rating: 3,
        context: {
          sessionId: 'session-xyz',
          pageUrl: 'https://example.com/',
          conceptId: 'cat',
          transcript: 'show me a cat',
          userAgent: 'Mozilla/5.0',
        },
      };

      await submitFeedback(data);

      expect(fetchSpy).toHaveBeenCalledWith('/api/feedback', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          type: 'bug',
          message: 'Something broke',
          rating: 3,
          context: {
            session_id: 'session-xyz',
            page_url: 'https://example.com/',
            concept_id: 'cat',
            transcript: 'show me a cat',
            user_agent: 'Mozilla/5.0',
          },
        }),
      });
    });

    it('throws error on API error response', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        status: 400,
        json: () => Promise.resolve({ error: 'message is required' }),
      } as Response);

      const data: FeedbackData = {
        type: 'general',
        message: '',
        context: {
          sessionId: 'session-123',
          pageUrl: '/',
        },
      };

      await expect(submitFeedback(data)).rejects.toThrow('message is required');
    });

    it('throws user-friendly error on rate limit', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        status: 429,
        json: () =>
          Promise.resolve({ error: 'rate limit exceeded, try again later' }),
      } as Response);

      const data: FeedbackData = {
        type: 'general',
        message: 'Test',
        context: {
          sessionId: 'session-123',
          pageUrl: '/',
        },
      };

      await expect(submitFeedback(data)).rejects.toThrow(
        'Too many requests. Please try again later.'
      );
    });

    it('handles network errors', async () => {
      fetchSpy.mockRejectedValueOnce(new Error('Network error'));

      const data: FeedbackData = {
        type: 'general',
        message: 'Test',
        context: {
          sessionId: 'session-123',
          pageUrl: '/',
        },
      };

      await expect(submitFeedback(data)).rejects.toThrow('Network error');
    });

    it('throws generic error when error response is not JSON', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        status: 502,
        json: () => Promise.reject(new Error('Unexpected token')),
      } as unknown as Response);

      const data: FeedbackData = {
        type: 'general',
        message: 'Test',
        context: {
          sessionId: 'session-123',
          pageUrl: '/',
        },
      };

      await expect(submitFeedback(data)).rejects.toThrow('Failed to submit feedback');
    });

    it('returns empty object when success response is not JSON', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.reject(new Error('Unexpected token')),
      } as unknown as Response);

      const data: FeedbackData = {
        type: 'general',
        message: 'Test',
        context: {
          sessionId: 'session-123',
          pageUrl: '/',
        },
      };

      const result = await submitFeedback(data);
      expect(result).toEqual({});
    });

    it('excludes null/undefined optional fields from request', async () => {
      const mockResponse = { id: 'feedback_789', status: 'ok' };
      fetchSpy.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      } as Response);

      const data: FeedbackData = {
        type: 'feature',
        message: 'Add elephants!',
        rating: null,
        context: {
          sessionId: 'session-abc',
          pageUrl: '/',
          conceptId: null,
          transcript: null,
        },
      };

      await submitFeedback(data);

      const body = JSON.parse(fetchSpy.mock.calls[0][1]!.body as string);
      expect(body.rating).toBeUndefined();
      expect(body.context.concept_id).toBeUndefined();
      expect(body.context.transcript).toBeUndefined();
    });
  });
});
