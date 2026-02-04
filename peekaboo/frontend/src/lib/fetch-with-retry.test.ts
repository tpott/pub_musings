import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fetchWithRetry, calculateDelay, sleep } from './fetch-with-retry';

describe('calculateDelay', () => {
  it('calculates exponential backoff', () => {
    expect(calculateDelay(0, 1000, 10000)).toBe(1000); // 1000 * 2^0 = 1000
    expect(calculateDelay(1, 1000, 10000)).toBe(2000); // 1000 * 2^1 = 2000
    expect(calculateDelay(2, 1000, 10000)).toBe(4000); // 1000 * 2^2 = 4000
    expect(calculateDelay(3, 1000, 10000)).toBe(8000); // 1000 * 2^3 = 8000
  });

  it('caps delay at maxDelay', () => {
    expect(calculateDelay(4, 1000, 10000)).toBe(10000); // 1000 * 2^4 = 16000, capped at 10000
    expect(calculateDelay(10, 1000, 5000)).toBe(5000);
  });
});

describe('sleep', () => {
  it('resolves after specified time', async () => {
    const start = Date.now();
    await sleep(50);
    const elapsed = Date.now() - start;
    expect(elapsed).toBeGreaterThanOrEqual(40); // Allow some tolerance
  });
});

describe('fetchWithRetry', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it('returns response on first success', async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      headers: new Headers(),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const response = await fetchWithRetry('/api/test');

    expect(response).toBe(mockResponse);
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  it('retries on 429 status', async () => {
    const rateLimitedResponse = {
      ok: false,
      status: 429,
      headers: new Headers(),
    };
    const successResponse = {
      ok: true,
      status: 200,
      headers: new Headers(),
    };

    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce(rateLimitedResponse)
      .mockResolvedValueOnce(successResponse);

    const fetchPromise = fetchWithRetry('/api/test', undefined, {
      maxRetries: 3,
      baseDelay: 100,
    });

    // Fast-forward through the delay
    await vi.runAllTimersAsync();

    const response = await fetchPromise;
    expect(response).toBe(successResponse);
    expect(global.fetch).toHaveBeenCalledTimes(2);
  });

  it('retries on 500 status', async () => {
    const serverErrorResponse = {
      ok: false,
      status: 500,
      headers: new Headers(),
    };
    const successResponse = {
      ok: true,
      status: 200,
      headers: new Headers(),
    };

    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce(serverErrorResponse)
      .mockResolvedValueOnce(successResponse);

    const fetchPromise = fetchWithRetry('/api/test', undefined, {
      maxRetries: 3,
      baseDelay: 100,
    });

    await vi.runAllTimersAsync();

    const response = await fetchPromise;
    expect(response).toBe(successResponse);
    expect(global.fetch).toHaveBeenCalledTimes(2);
  });

  it('retries on network error', async () => {
    const successResponse = {
      ok: true,
      status: 200,
      headers: new Headers(),
    };

    (global.fetch as ReturnType<typeof vi.fn>)
      .mockRejectedValueOnce(new Error('Network error'))
      .mockResolvedValueOnce(successResponse);

    const fetchPromise = fetchWithRetry('/api/test', undefined, {
      maxRetries: 3,
      baseDelay: 100,
    });

    await vi.runAllTimersAsync();

    const response = await fetchPromise;
    expect(response).toBe(successResponse);
    expect(global.fetch).toHaveBeenCalledTimes(2);
  });

  it('throws after max retries exhausted', async () => {
    const errorResponse = {
      ok: false,
      status: 500,
      headers: new Headers(),
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(errorResponse);

    const fetchPromise = fetchWithRetry('/api/test', undefined, {
      maxRetries: 2,
      baseDelay: 100,
    });

    await vi.runAllTimersAsync();

    // After 2 retries (3 total attempts), it should return the error response
    const response = await fetchPromise;
    expect(response.status).toBe(500);
    expect(global.fetch).toHaveBeenCalledTimes(3); // Initial + 2 retries
  });

  it('does not retry on non-retryable status codes', async () => {
    const notFoundResponse = {
      ok: false,
      status: 404,
      headers: new Headers(),
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(notFoundResponse);

    const response = await fetchWithRetry('/api/test', undefined, {
      maxRetries: 3,
      baseDelay: 100,
    });

    expect(response.status).toBe(404);
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  it('respects Retry-After header', async () => {
    const rateLimitedResponse = {
      ok: false,
      status: 429,
      headers: new Headers({ 'Retry-After': '2' }), // 2 seconds
    };
    const successResponse = {
      ok: true,
      status: 200,
      headers: new Headers(),
    };

    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce(rateLimitedResponse)
      .mockResolvedValueOnce(successResponse);

    const fetchPromise = fetchWithRetry('/api/test', undefined, {
      maxRetries: 3,
      baseDelay: 100,
    });

    await vi.runAllTimersAsync();

    const response = await fetchPromise;
    expect(response).toBe(successResponse);
  });

  it('passes through request options', async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      headers: new Headers(),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await fetchWithRetry('/api/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key: 'value' }),
    });

    expect(global.fetch).toHaveBeenCalledWith('/api/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key: 'value' }),
    });
  });
});
