/**
 * Fetch timeout utility
 *
 * Provides a wrapper around fetch with configurable timeouts using AbortController.
 * Helps prevent hanging requests on slow networks.
 */

/** Default timeout in milliseconds (30 seconds) */
export const DEFAULT_TIMEOUT_MS = 30000;

/** Error thrown when a fetch request times out */
export class FetchTimeoutError extends Error {
	constructor(
		message: string = 'Request timed out',
		public readonly timeoutMs: number = DEFAULT_TIMEOUT_MS
	) {
		super(message);
		this.name = 'FetchTimeoutError';
	}
}

export interface FetchWithTimeoutOptions extends RequestInit {
	/** Timeout in milliseconds. Default: 30000 (30 seconds) */
	timeoutMs?: number;
}

/**
 * Fetch with automatic timeout handling.
 *
 * @param url - The URL to fetch
 * @param options - Fetch options plus optional timeoutMs
 * @returns Promise with the fetch response
 * @throws FetchTimeoutError if the request times out
 *
 * @example
 * ```typescript
 * try {
 *   const response = await fetchWithTimeout('/api/data', { timeoutMs: 5000 });
 *   const data = await response.json();
 * } catch (err) {
 *   if (err instanceof FetchTimeoutError) {
 *     console.error('Request timed out after', err.timeoutMs, 'ms');
 *   }
 * }
 * ```
 */
export async function fetchWithTimeout(
	url: string,
	options: FetchWithTimeoutOptions = {}
): Promise<Response> {
	const { timeoutMs = DEFAULT_TIMEOUT_MS, signal: existingSignal, ...fetchOptions } = options;

	// Create AbortController for timeout
	const controller = new AbortController();

	// If an existing signal was provided, chain abort events
	if (existingSignal) {
		if (existingSignal.aborted) {
			// Already aborted
			controller.abort(existingSignal.reason);
		} else {
			existingSignal.addEventListener('abort', () => {
				controller.abort(existingSignal.reason);
			});
		}
	}

	// Set up timeout
	const timeoutId = setTimeout(() => {
		controller.abort(new FetchTimeoutError(`Request timed out after ${timeoutMs}ms`, timeoutMs));
	}, timeoutMs);

	try {
		const response = await fetch(url, {
			...fetchOptions,
			signal: controller.signal,
		});
		return response;
	} catch (err) {
		// Check if this was a timeout abort
		if (err instanceof Error && err.name === 'AbortError') {
			// Check if the abort reason is our timeout error
			const abortReason = controller.signal.reason;
			if (abortReason instanceof FetchTimeoutError) {
				throw abortReason;
			}
			// Re-throw as timeout error if we timed out
			throw new FetchTimeoutError(`Request timed out after ${timeoutMs}ms`, timeoutMs);
		}
		throw err;
	} finally {
		clearTimeout(timeoutId);
	}
}

/**
 * Check if an error is a timeout error.
 *
 * @param err - The error to check
 * @returns true if the error is a FetchTimeoutError or AbortError from timeout
 */
export function isTimeoutError(err: unknown): err is FetchTimeoutError {
	return err instanceof FetchTimeoutError;
}

/**
 * Create an AbortController with automatic timeout.
 * Useful when you need more control over the abort logic.
 *
 * @param timeoutMs - Timeout in milliseconds
 * @returns Object with controller and cleanup function
 *
 * @example
 * ```typescript
 * const { controller, cleanup } = createTimeoutController(5000);
 * try {
 *   const response = await fetch('/api/data', { signal: controller.signal });
 * } finally {
 *   cleanup();
 * }
 * ```
 */
export function createTimeoutController(timeoutMs: number = DEFAULT_TIMEOUT_MS): {
	controller: AbortController;
	cleanup: () => void;
} {
	const controller = new AbortController();
	const timeoutId = setTimeout(() => {
		controller.abort(new FetchTimeoutError(`Request timed out after ${timeoutMs}ms`, timeoutMs));
	}, timeoutMs);

	return {
		controller,
		cleanup: () => clearTimeout(timeoutId),
	};
}
