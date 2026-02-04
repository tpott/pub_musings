import { describe, it, expect, vi } from 'vitest';
import {
  ApiError,
  getUserFriendlyMessage,
  createApiErrorFromResponse,
  createNetworkError,
  ErrorType,
} from './errors';

describe('ApiError', () => {
  it('creates error with type and status', () => {
    const error = new ApiError('Test error', 'server', 500);
    expect(error.message).toBe('Test error');
    expect(error.type).toBe('server');
    expect(error.status).toBe(500);
    expect(error.name).toBe('ApiError');
  });

  it('creates error with retryAfter for rate limits', () => {
    const error = new ApiError('Rate limited', 'rate_limit', 429, 60);
    expect(error.retryAfter).toBe(60);
  });
});

describe('getUserFriendlyMessage', () => {
  it('returns network message for network errors', () => {
    const error = new ApiError('Network failed', 'network');
    expect(getUserFriendlyMessage(error)).toBe(
      'Network issue - please check your connection and try again'
    );
  });

  it('returns rate limit message with retry time', () => {
    const error = new ApiError('Too many requests', 'rate_limit', 429, 30);
    expect(getUserFriendlyMessage(error)).toBe(
      'Too many requests - please wait 30 seconds'
    );
  });

  it('returns rate limit message without retry time', () => {
    const error = new ApiError('Too many requests', 'rate_limit', 429);
    expect(getUserFriendlyMessage(error)).toBe(
      'Too many requests - please wait a moment and try again'
    );
  });

  it('returns server error message for server errors', () => {
    const error = new ApiError('Server error', 'server', 500);
    expect(getUserFriendlyMessage(error)).toBe(
      'Service temporarily unavailable - please try again'
    );
  });

  it('returns original message for client errors', () => {
    const error = new ApiError('Invalid request', 'client', 400);
    expect(getUserFriendlyMessage(error)).toBe(
      'Invalid request'
    );
  });

  it('returns generic message for unknown errors', () => {
    const error = new ApiError('Something', 'unknown');
    expect(getUserFriendlyMessage(error)).toBe(
      'Something went wrong - please try again'
    );
  });

  it('handles TypeError Failed to fetch as network error', () => {
    const error = new TypeError('Failed to fetch');
    expect(getUserFriendlyMessage(error)).toBe(
      'Network issue - please check your connection and try again'
    );
  });

  it('returns message for generic Error', () => {
    const error = new Error('Some error');
    expect(getUserFriendlyMessage(error)).toBe('Some error');
  });
});

describe('createApiErrorFromResponse', () => {
  function createMockResponse(status: number, body?: object, headers?: Record<string, string>): Response {
    const headersObj = new Headers(headers);
    return {
      status,
      ok: status >= 200 && status < 300,
      headers: headersObj,
      json: vi.fn().mockResolvedValue(body || {}),
    } as unknown as Response;
  }

  it('creates rate_limit error for 429 status', async () => {
    const response = createMockResponse(429, { error: 'Rate limited' }, { 'Retry-After': '60' });
    const error = await createApiErrorFromResponse(response, 'Default message');

    expect(error.type).toBe('rate_limit');
    expect(error.status).toBe(429);
    expect(error.message).toBe('Rate limited');
    expect(error.retryAfter).toBe(60);
  });

  it('creates server error for 500 status', async () => {
    const response = createMockResponse(500, { error: 'Internal error' });
    const error = await createApiErrorFromResponse(response, 'Default message');

    expect(error.type).toBe('server');
    expect(error.status).toBe(500);
    expect(error.message).toBe('Internal error');
  });

  it('creates server error for 502 status', async () => {
    const response = createMockResponse(502);
    const error = await createApiErrorFromResponse(response, 'Bad gateway');

    expect(error.type).toBe('server');
    expect(error.status).toBe(502);
  });

  it('creates client error for 400 status', async () => {
    const response = createMockResponse(400, { error: 'Invalid input' });
    const error = await createApiErrorFromResponse(response, 'Default message');

    expect(error.type).toBe('client');
    expect(error.status).toBe(400);
    expect(error.message).toBe('Invalid input');
  });

  it('creates client error for 404 status', async () => {
    const response = createMockResponse(404, { error: 'Not found' });
    const error = await createApiErrorFromResponse(response, 'Default message');

    expect(error.type).toBe('client');
    expect(error.status).toBe(404);
    expect(error.message).toBe('Not found');
  });

  it('uses default message when JSON parse fails', async () => {
    const response = {
      status: 500,
      ok: false,
      headers: new Headers(),
      json: vi.fn().mockRejectedValue(new Error('Parse error')),
    } as unknown as Response;

    const error = await createApiErrorFromResponse(response, 'Default message');

    expect(error.message).toBe('Default message');
  });
});

describe('createNetworkError', () => {
  it('creates network error from original error', () => {
    const original = new Error('Network request failed');
    const error = createNetworkError(original);

    expect(error.type).toBe('network');
    expect(error.message).toBe('Network request failed');
  });
});
