/**
 * Fetch utility with exponential backoff retry logic
 */

export interface RetryOptions {
  /** Maximum number of retry attempts (default: 3) */
  maxRetries?: number;
  /** Base delay in milliseconds (default: 1000) */
  baseDelay?: number;
  /** Maximum delay in milliseconds (default: 10000) */
  maxDelay?: number;
  /** HTTP status codes that should trigger a retry (default: [429, 500, 502, 503, 504]) */
  retryStatusCodes?: number[];
}

const DEFAULT_OPTIONS: Required<RetryOptions> = {
  maxRetries: 3,
  baseDelay: 1000,
  maxDelay: 10000,
  retryStatusCodes: [429, 500, 502, 503, 504],
};

/**
 * Calculate exponential backoff delay
 * @param attempt Current attempt number (0-indexed)
 * @param baseDelay Base delay in milliseconds
 * @param maxDelay Maximum delay in milliseconds
 * @returns Delay in milliseconds
 */
export function calculateDelay(attempt: number, baseDelay: number, maxDelay: number): number {
  // Exponential backoff: baseDelay * 2^attempt
  const delay = baseDelay * Math.pow(2, attempt);
  return Math.min(delay, maxDelay);
}

/**
 * Sleep for a given number of milliseconds
 * @param ms Milliseconds to sleep
 */
export function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * Fetch with automatic retry on failure with exponential backoff
 *
 * Retries on:
 * - Network errors
 * - Rate limiting (429)
 * - Server errors (500, 502, 503, 504)
 *
 * @param input Fetch input (URL or Request)
 * @param init Fetch options
 * @param options Retry options
 * @returns Response if successful
 * @throws Error after all retries exhausted
 */
export async function fetchWithRetry(
  input: RequestInfo | URL,
  init?: RequestInit,
  options?: RetryOptions
): Promise<Response> {
  const opts = { ...DEFAULT_OPTIONS, ...options };
  let lastError: Error | null = null;

  for (let attempt = 0; attempt <= opts.maxRetries; attempt++) {
    try {
      const response = await fetch(input, init);

      // Check if we should retry based on status code
      if (opts.retryStatusCodes.includes(response.status)) {
        if (attempt < opts.maxRetries) {
          // Discard response body to free the underlying connection
          response.body?.cancel();

          // Use Retry-After header if present, otherwise calculate backoff
          let delay: number;
          const retryAfter = response.headers.get('Retry-After');
          if (retryAfter) {
            // Retry-After can be seconds or a date
            const retryAfterSeconds = parseInt(retryAfter, 10);
            delay = isNaN(retryAfterSeconds) ? opts.baseDelay : retryAfterSeconds * 1000;
          } else {
            delay = calculateDelay(attempt, opts.baseDelay, opts.maxDelay);
          }

          await sleep(delay);
          continue;
        }
      }

      // Return response (successful or non-retryable error)
      return response;
    } catch (error) {
      // Network error or fetch failed
      lastError = error instanceof Error ? error : new Error(String(error));

      if (attempt < opts.maxRetries) {
        const delay = calculateDelay(attempt, opts.baseDelay, opts.maxDelay);
        await sleep(delay);
        continue;
      }
    }
  }

  // All retries exhausted
  throw lastError || new Error('Fetch failed after retries');
}
