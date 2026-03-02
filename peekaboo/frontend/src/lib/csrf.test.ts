import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fetchCSRFToken, getCSRFToken, getCSRFHeaders, clearCSRFToken, resetCSRFState } from './csrf';

describe('csrf', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    resetCSRFState();
    fetchSpy = vi.spyOn(globalThis, 'fetch');
  });

  describe('fetchCSRFToken', () => {
    it('fetches and caches the CSRF token', async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ token: 'csrf-abc-123' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      );

      const token = await fetchCSRFToken();

      expect(token).toBe('csrf-abc-123');
      expect(getCSRFToken()).toBe('csrf-abc-123');
      expect(fetchSpy).toHaveBeenCalledWith('/api/auth/csrf', {
        credentials: 'same-origin',
      });
    });

    it('returns null when not authenticated (401)', async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'not authenticated' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
      );

      const token = await fetchCSRFToken();

      expect(token).toBeNull();
      expect(getCSRFToken()).toBeNull();
    });

    it('returns null on network error', async () => {
      fetchSpy.mockRejectedValueOnce(new Error('Network error'));

      const token = await fetchCSRFToken();

      expect(token).toBeNull();
      expect(getCSRFToken()).toBeNull();
    });

    it('returns null when response is not JSON', async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response('not json', { status: 200 }),
      );

      const token = await fetchCSRFToken();

      expect(token).toBeNull();
    });

    it('returns null when response has no token field', async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'something' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      );

      const token = await fetchCSRFToken();

      expect(token).toBeNull();
    });
  });

  describe('getCSRFToken', () => {
    it('returns null before fetch', () => {
      expect(getCSRFToken()).toBeNull();
    });

    it('returns cached token after fetch', async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ token: 'my-token' }), { status: 200 }),
      );

      await fetchCSRFToken();

      expect(getCSRFToken()).toBe('my-token');
    });
  });

  describe('getCSRFHeaders', () => {
    it('returns empty object when no token cached', () => {
      const headers = getCSRFHeaders();

      expect(headers).toEqual({});
    });

    it('returns X-CSRF-Token header when token is cached', async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ token: 'test-token' }), { status: 200 }),
      );
      await fetchCSRFToken();

      const headers = getCSRFHeaders();

      expect(headers).toEqual({ 'X-CSRF-Token': 'test-token' });
    });

    it('merges with existing plain object headers', async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ token: 'test-token' }), { status: 200 }),
      );
      await fetchCSRFToken();

      const headers = getCSRFHeaders({ 'Content-Type': 'application/json' });

      expect(headers).toEqual({
        'Content-Type': 'application/json',
        'X-CSRF-Token': 'test-token',
      });
    });

    it('merges with existing Headers instance', async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ token: 'test-token' }), { status: 200 }),
      );
      await fetchCSRFToken();

      const existing = new Headers({ 'Content-Type': 'text/plain' });
      const headers = getCSRFHeaders(existing);

      expect(headers).toEqual({
        'content-type': 'text/plain',
        'X-CSRF-Token': 'test-token',
      });
    });

    it('merges with existing array headers', async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ token: 'test-token' }), { status: 200 }),
      );
      await fetchCSRFToken();

      const headers = getCSRFHeaders([['Content-Type', 'application/json']]);

      expect(headers).toEqual({
        'Content-Type': 'application/json',
        'X-CSRF-Token': 'test-token',
      });
    });

    it('preserves existing headers when no token', () => {
      const headers = getCSRFHeaders({ 'Content-Type': 'application/json' });

      expect(headers).toEqual({ 'Content-Type': 'application/json' });
    });
  });

  describe('clearCSRFToken', () => {
    it('clears the cached token', async () => {
      fetchSpy.mockResolvedValueOnce(
        new Response(JSON.stringify({ token: 'to-clear' }), { status: 200 }),
      );
      await fetchCSRFToken();
      expect(getCSRFToken()).toBe('to-clear');

      clearCSRFToken();

      expect(getCSRFToken()).toBeNull();
    });
  });
});
