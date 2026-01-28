import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	CSRF_HEADER,
	fetchCsrfToken,
	getCsrfToken,
	clearCsrfToken,
	setCsrfToken,
	createCsrfHeaders,
	csrfFetch,
	getMinRefreshInterval,
	setMinRefreshInterval,
	resetRateLimiter,
} from './csrf';

// Mock fetch
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

describe('CSRF utility', () => {
	let originalInterval: number;

	beforeEach(() => {
		clearCsrfToken();
		resetRateLimiter();
		mockFetch.mockReset();
		// Save original interval
		originalInterval = getMinRefreshInterval();
	});

	afterEach(() => {
		// Restore original interval
		setMinRefreshInterval(originalInterval);
		vi.restoreAllMocks();
	});

	describe('fetchCsrfToken', () => {
		it('should fetch and cache CSRF token', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'test-token-123' }),
			});

			const token = await fetchCsrfToken();
			expect(token).toBe('test-token-123');
			expect(mockFetch).toHaveBeenCalledWith('/api/auth/csrf');
		});

		it('should return null when not authenticated', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 401,
			});

			const token = await fetchCsrfToken();
			expect(token).toBeNull();
		});

		it('should return null on network error', async () => {
			mockFetch.mockRejectedValueOnce(new Error('Network error'));

			const token = await fetchCsrfToken();
			expect(token).toBeNull();
		});

		it('should return null on JSON parse error', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => {
					throw new SyntaxError('Unexpected token');
				},
			});

			const token = await fetchCsrfToken();
			expect(token).toBeNull();
		});

		it('should handle response with missing csrf_token field', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({}),
			});

			const token = await fetchCsrfToken();
			expect(token).toBeNull();
		});

		it('should rate limit rapid refresh attempts', async () => {
			// Set a short interval for testing
			setMinRefreshInterval(100);

			// First fetch should go through
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'token1' }),
			});

			const token1 = await fetchCsrfToken();
			expect(token1).toBe('token1');
			expect(mockFetch).toHaveBeenCalledTimes(1);

			// Second fetch immediately after should be rate limited
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'token2' }),
			});

			const token2 = await fetchCsrfToken();
			// Should return cached token without making new request
			expect(token2).toBe('token1');
			expect(mockFetch).toHaveBeenCalledTimes(1);
		});

		it('should allow refresh after rate limit interval passes', async () => {
			// Set a very short interval for testing
			setMinRefreshInterval(10);

			// First fetch
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'token1' }),
			});
			await fetchCsrfToken();
			expect(mockFetch).toHaveBeenCalledTimes(1);

			// Wait for interval to pass
			await new Promise((resolve) => setTimeout(resolve, 20));

			// Second fetch should go through
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'token2' }),
			});
			const token = await fetchCsrfToken();
			expect(token).toBe('token2');
			expect(mockFetch).toHaveBeenCalledTimes(2);
		});

		it('should return null when rate limited and no cached token', async () => {
			// Set a short interval for testing
			setMinRefreshInterval(100);

			// First fetch fails (no token cached)
			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 401,
			});

			const token1 = await fetchCsrfToken();
			expect(token1).toBeNull();

			// Second fetch immediately - should be rate limited and return null
			const token2 = await fetchCsrfToken();
			expect(token2).toBeNull();
			// Only one fetch should have been made
			expect(mockFetch).toHaveBeenCalledTimes(1);
		});

		it('should reset rate limiter when clearing token', async () => {
			// Set a long interval
			setMinRefreshInterval(10000);

			// First fetch
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'token1' }),
			});
			await fetchCsrfToken();

			// Clear token - should reset rate limiter
			clearCsrfToken();

			// Second fetch should go through immediately
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'token2' }),
			});
			const token = await fetchCsrfToken();
			expect(token).toBe('token2');
			expect(mockFetch).toHaveBeenCalledTimes(2);
		});
	});

	describe('getCsrfToken', () => {
		it('should return cached token without fetching', async () => {
			setCsrfToken('cached-token');

			const token = await getCsrfToken();
			expect(token).toBe('cached-token');
			expect(mockFetch).not.toHaveBeenCalled();
		});

		it('should fetch token if not cached', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'fetched-token' }),
			});

			const token = await getCsrfToken();
			expect(token).toBe('fetched-token');
			expect(mockFetch).toHaveBeenCalled();
		});
	});

	describe('clearCsrfToken', () => {
		it('should clear the cached token', async () => {
			setCsrfToken('token-to-clear');

			clearCsrfToken();

			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'new-token' }),
			});

			// Should fetch again after clearing
			const token = await getCsrfToken();
			expect(mockFetch).toHaveBeenCalled();
			expect(token).toBe('new-token');
		});
	});

	describe('createCsrfHeaders', () => {
		it('should include CSRF token in headers', async () => {
			setCsrfToken('header-token');

			const headers = await createCsrfHeaders();
			expect(headers[CSRF_HEADER]).toBe('header-token');
		});

		it('should merge with additional headers', async () => {
			setCsrfToken('merge-token');

			const headers = await createCsrfHeaders({
				'Content-Type': 'application/json',
			});
			expect(headers[CSRF_HEADER]).toBe('merge-token');
			expect(headers['Content-Type']).toBe('application/json');
		});

		it('should return empty object when no token', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 401,
			});

			const headers = await createCsrfHeaders();
			expect(headers[CSRF_HEADER]).toBeUndefined();
		});
	});

	describe('csrfFetch', () => {
		it('should add CSRF token to POST requests', async () => {
			setCsrfToken('post-token');
			mockFetch.mockResolvedValueOnce({
				ok: true,
				status: 200,
			});

			await csrfFetch('/api/test', { method: 'POST' });

			expect(mockFetch).toHaveBeenCalled();
			const [url, options] = mockFetch.mock.calls[0];
			expect(url).toBe('/api/test');
			expect(options.headers.get(CSRF_HEADER)).toBe('post-token');
		});

		it('should add CSRF token to DELETE requests', async () => {
			setCsrfToken('delete-token');
			mockFetch.mockResolvedValueOnce({
				ok: true,
				status: 200,
			});

			await csrfFetch('/api/test/123', { method: 'DELETE' });

			const [, options] = mockFetch.mock.calls[0];
			expect(options.headers.get(CSRF_HEADER)).toBe('delete-token');
		});

		it('should add CSRF token to PUT requests', async () => {
			setCsrfToken('put-token');
			mockFetch.mockResolvedValueOnce({
				ok: true,
				status: 200,
			});

			await csrfFetch('/api/test', { method: 'PUT' });

			const [, options] = mockFetch.mock.calls[0];
			expect(options.headers.get(CSRF_HEADER)).toBe('put-token');
		});

		it('should NOT add CSRF token to GET requests', async () => {
			setCsrfToken('get-token');
			mockFetch.mockResolvedValueOnce({
				ok: true,
				status: 200,
			});

			await csrfFetch('/api/test', { method: 'GET' });

			const [, options] = mockFetch.mock.calls[0];
			// GET should not modify headers
			expect(options.headers).toBeUndefined();
		});

		it('should retry with fresh token on CSRF 403 error', async () => {
			setCsrfToken('old-token');

			// First call returns CSRF error
			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 403,
				clone: () => ({
					json: async () => ({ error: 'Invalid or missing CSRF token' }),
				}),
			});

			// Token refresh
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'new-token' }),
			});

			// Retry succeeds
			mockFetch.mockResolvedValueOnce({
				ok: true,
				status: 200,
			});

			const response = await csrfFetch('/api/test', { method: 'POST' });
			expect(response.ok).toBe(true);
			expect(mockFetch).toHaveBeenCalledTimes(3);
		});

		it('should preserve existing headers', async () => {
			setCsrfToken('preserve-token');
			mockFetch.mockResolvedValueOnce({
				ok: true,
				status: 200,
			});

			await csrfFetch('/api/test', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
			});

			const [, options] = mockFetch.mock.calls[0];
			expect(options.headers.get(CSRF_HEADER)).toBe('preserve-token');
			expect(options.headers.get('Content-Type')).toBe('application/json');
		});

		it('should stop retrying after max consecutive failures (circuit breaker)', async () => {
			// Disable rate limiting for this test
			setMinRefreshInterval(0);

			// First, get a valid token so we can make CSRF requests
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'initial-token' }),
			});
			await fetchCsrfToken();

			// First request: CSRF error -> triggers refresh -> refresh fails
			mockFetch
				.mockResolvedValueOnce({
					ok: false,
					status: 403,
					clone: () => ({
						json: async () => ({ error: 'Invalid CSRF token' }),
					}),
				})
				.mockResolvedValueOnce({ ok: false, status: 401 }); // refresh fails (count=1)
			await csrfFetch('/api/test1', { method: 'POST' });

			// Second request needs getCsrfToken() which will fetch since cache is null
			mockFetch
				.mockResolvedValueOnce({ ok: false, status: 401 }) // getCsrfToken() fails (count=2, trips breaker)
				.mockResolvedValueOnce({
					ok: false,
					status: 403,
					clone: () => ({
						json: async () => ({ error: 'Invalid CSRF token' }),
					}),
				}); // CSRF error, won't retry due to breaker
			await csrfFetch('/api/test2', { method: 'POST' });

			// Third request - getCsrfToken will fail, and circuit breaker should prevent CSRF retry
			mockFetch
				.mockResolvedValueOnce({ ok: false, status: 401 }) // getCsrfToken() fails but breaker already tripped
				.mockResolvedValueOnce({
					ok: false,
					status: 403,
					clone: () => ({
						json: async () => ({ error: 'Invalid CSRF token' }),
					}),
				});

			const response = await csrfFetch('/api/test3', { method: 'POST' });

			// Verify the response came back as 403 without infinite retry
			expect(response.status).toBe(403);
		});

		it('should reset circuit breaker after successful token fetch', async () => {
			// Disable rate limiting for this test
			setMinRefreshInterval(0);

			// Fail twice to trip circuit breaker
			mockFetch.mockResolvedValueOnce({ ok: false, status: 401 }); // fail 1
			await fetchCsrfToken();
			mockFetch.mockResolvedValueOnce({ ok: false, status: 401 }); // fail 2
			await fetchCsrfToken();

			// Succeed - should reset breaker
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'new-token' }),
			});
			await fetchCsrfToken();

			// Now CSRF error should trigger retry again
			setCsrfToken('test-token');
			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 403,
				clone: () => ({
					json: async () => ({ error: 'Invalid CSRF token' }),
				}),
			});
			mockFetch.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ csrf_token: 'refreshed-token' }),
			});
			mockFetch.mockResolvedValueOnce({
				ok: true,
				status: 200,
			});

			const response = await csrfFetch('/api/test', { method: 'POST' });
			expect(response.ok).toBe(true);
		});
	});
});
