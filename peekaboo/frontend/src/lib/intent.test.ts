import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { extractIntent } from './intent';

describe('extractIntent', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('sends text to /api/intent', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({ subject: 'cat' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await extractIntent('show me a cat');

    expect(global.fetch).toHaveBeenCalledWith('/api/intent', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ text: 'show me a cat' }),
    });
  });

  it('returns extracted subject', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({ subject: 'dog' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const result = await extractIntent('show me a dog');

    expect(result.subject).toBe('dog');
  });

  it('throws error on failed response', async () => {
    // Use 400 which is not retried by fetchWithRetry
    const mockResponse = {
      ok: false,
      status: 400,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({ error: 'Bad request' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await expect(extractIntent('show me a cat')).rejects.toThrow('Bad request');
  });

  it('throws error when response contains error field', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({ error: 'no show_media tool call in response' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await expect(extractIntent('hello there')).rejects.toThrow('no show_media tool call in response');
  });

  it('throws error when no subject extracted', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({}),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await expect(extractIntent('show me something')).rejects.toThrow('No subject extracted');
  });
});
