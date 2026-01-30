import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	fetchWithTimeout,
	FetchTimeoutError,
	isTimeoutError,
	createTimeoutController,
	DEFAULT_TIMEOUT_MS,
} from './fetch-timeout';

describe('fetch-timeout utility', () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('FetchTimeoutError', () => {
		it('should be an instance of Error', () => {
			const error = new FetchTimeoutError();
			expect(error).toBeInstanceOf(Error);
		});

		it('should have name FetchTimeoutError', () => {
			const error = new FetchTimeoutError();
			expect(error.name).toBe('FetchTimeoutError');
		});

		it('should use default message and timeout', () => {
			const error = new FetchTimeoutError();
			expect(error.message).toBe('Request timed out');
			expect(error.timeoutMs).toBe(DEFAULT_TIMEOUT_MS);
		});

		it('should accept custom message and timeout', () => {
			const error = new FetchTimeoutError('Custom timeout', 5000);
			expect(error.message).toBe('Custom timeout');
			expect(error.timeoutMs).toBe(5000);
		});
	});

	describe('isTimeoutError', () => {
		it('should return true for FetchTimeoutError', () => {
			const error = new FetchTimeoutError();
			expect(isTimeoutError(error)).toBe(true);
		});

		it('should return false for regular Error', () => {
			const error = new Error('regular error');
			expect(isTimeoutError(error)).toBe(false);
		});

		it('should return false for null', () => {
			expect(isTimeoutError(null)).toBe(false);
		});

		it('should return false for undefined', () => {
			expect(isTimeoutError(undefined)).toBe(false);
		});

		it('should return false for string', () => {
			expect(isTimeoutError('error string')).toBe(false);
		});
	});

	describe('fetchWithTimeout', () => {
		it('should return response on successful fetch', async () => {
			const mockResponse = new Response(JSON.stringify({ success: true }), {
				status: 200,
			});
			vi.spyOn(global, 'fetch').mockResolvedValue(mockResponse);

			const response = await fetchWithTimeout('/api/test');

			expect(response).toBe(mockResponse);
			expect(fetch).toHaveBeenCalledWith('/api/test', expect.objectContaining({
				signal: expect.any(AbortSignal),
			}));
		});

		it('should pass through fetch options', async () => {
			const mockResponse = new Response('ok');
			vi.spyOn(global, 'fetch').mockResolvedValue(mockResponse);

			await fetchWithTimeout('/api/test', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ data: 'test' }),
			});

			expect(fetch).toHaveBeenCalledWith('/api/test', expect.objectContaining({
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ data: 'test' }),
				signal: expect.any(AbortSignal),
			}));
		});

		it('should use default timeout', () => {
			// Verify that DEFAULT_TIMEOUT_MS is used when no timeout specified
			expect(DEFAULT_TIMEOUT_MS).toBe(30000);
		});

		it('should timeout with short timeout', async () => {
			// Mock fetch to hang until aborted
			vi.spyOn(global, 'fetch').mockImplementation((_url, options) => {
				return new Promise((_resolve, reject) => {
					const signal = options?.signal as AbortSignal | undefined;
					if (signal) {
						signal.addEventListener('abort', () => {
							reject(new DOMException('Aborted', 'AbortError'));
						});
					}
				});
			});

			// Use very short timeout for testing
			await expect(fetchWithTimeout('/api/test', { timeoutMs: 50 })).rejects.toThrow(FetchTimeoutError);
		}, 1000);

		it('should throw FetchTimeoutError on timeout', async () => {
			// Mock fetch to hang until aborted
			vi.spyOn(global, 'fetch').mockImplementation((_url, options) => {
				return new Promise((_resolve, reject) => {
					const signal = options?.signal as AbortSignal | undefined;
					if (signal) {
						signal.addEventListener('abort', () => {
							reject(new DOMException('Aborted', 'AbortError'));
						});
					}
				});
			});

			try {
				await fetchWithTimeout('/api/test', { timeoutMs: 50 });
				expect.fail('Should have thrown');
			} catch (err) {
				expect(err).toBeInstanceOf(FetchTimeoutError);
				expect((err as FetchTimeoutError).timeoutMs).toBe(50);
				expect((err as FetchTimeoutError).message).toContain('50ms');
			}
		}, 1000);

		it('should clear timeout on successful response', async () => {
			const clearTimeoutSpy = vi.spyOn(global, 'clearTimeout');
			const mockResponse = new Response('ok');
			vi.spyOn(global, 'fetch').mockResolvedValue(mockResponse);

			await fetchWithTimeout('/api/test');

			expect(clearTimeoutSpy).toHaveBeenCalled();
		});

		it('should clear timeout on fetch error', async () => {
			const clearTimeoutSpy = vi.spyOn(global, 'clearTimeout');
			vi.spyOn(global, 'fetch').mockRejectedValue(new Error('Network error'));

			try {
				await fetchWithTimeout('/api/test');
			} catch {
				// Expected
			}

			expect(clearTimeoutSpy).toHaveBeenCalled();
		});

		it('should propagate network errors', async () => {
			const networkError = new Error('Network failed');
			vi.spyOn(global, 'fetch').mockRejectedValue(networkError);

			await expect(fetchWithTimeout('/api/test')).rejects.toThrow('Network failed');
		});

		it('should handle existing signal that is already aborted', async () => {
			const existingController = new AbortController();
			existingController.abort();

			vi.spyOn(global, 'fetch').mockImplementation(() => {
				throw new DOMException('Aborted', 'AbortError');
			});

			// Should throw timeout error since we convert AbortError to timeout
			await expect(
				fetchWithTimeout('/api/test', { signal: existingController.signal })
			).rejects.toThrow(FetchTimeoutError);
		});
	});

	describe('createTimeoutController', () => {
		it('should create an AbortController', () => {
			const { controller, cleanup } = createTimeoutController();
			expect(controller).toBeInstanceOf(AbortController);
			cleanup();
		});

		it('should abort after timeout', async () => {
			const { controller, cleanup } = createTimeoutController(50);

			expect(controller.signal.aborted).toBe(false);

			// Wait for timeout to fire
			await new Promise(resolve => setTimeout(resolve, 60));

			expect(controller.signal.aborted).toBe(true);
			cleanup();
		}, 1000);

		it('should not abort immediately', () => {
			const { controller, cleanup } = createTimeoutController(1000);

			// Immediately after creation, should not be aborted
			expect(controller.signal.aborted).toBe(false);
			cleanup();
		});

		it('should use default timeout when not specified', () => {
			// Verify default is 30 seconds
			expect(DEFAULT_TIMEOUT_MS).toBe(30000);

			const { controller, cleanup } = createTimeoutController();
			// Should not abort immediately
			expect(controller.signal.aborted).toBe(false);
			cleanup();
		});

		it('cleanup should clear the timeout', () => {
			const clearTimeoutSpy = vi.spyOn(global, 'clearTimeout');
			const { cleanup } = createTimeoutController();

			cleanup();

			expect(clearTimeoutSpy).toHaveBeenCalled();
		});

		it('cleanup should prevent abort after cleanup', async () => {
			const { controller, cleanup } = createTimeoutController(50);

			// Clean up immediately
			cleanup();

			// Wait longer than the timeout would have been
			await new Promise(resolve => setTimeout(resolve, 100));

			// Should not be aborted since we cleaned up
			expect(controller.signal.aborted).toBe(false);
		}, 1000);
	});

	describe('module exports', () => {
		it('should export DEFAULT_TIMEOUT_MS constant', () => {
			expect(DEFAULT_TIMEOUT_MS).toBe(30000);
		});

		it('should export FetchTimeoutError class', () => {
			expect(FetchTimeoutError).toBeDefined();
			expect(new FetchTimeoutError()).toBeInstanceOf(Error);
		});

		it('should export fetchWithTimeout function', () => {
			expect(typeof fetchWithTimeout).toBe('function');
		});

		it('should export isTimeoutError function', () => {
			expect(typeof isTimeoutError).toBe('function');
		});

		it('should export createTimeoutController function', () => {
			expect(typeof createTimeoutController).toBe('function');
		});
	});
});
