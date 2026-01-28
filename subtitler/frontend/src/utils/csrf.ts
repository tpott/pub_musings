// CSRF token management for API requests
// Fetches and caches the CSRF token from the backend

let cachedToken: string | null = null;

// Circuit breaker: track consecutive refresh failures
let consecutiveRefreshFailures = 0;
const MAX_REFRESH_FAILURES = 2;

// Header name for CSRF token
export const CSRF_HEADER = 'X-CSRF-Token';

/**
 * Fetch the CSRF token from the backend.
 * The token is cached for subsequent requests.
 * Resets the circuit breaker on successful fetch.
 */
export async function fetchCsrfToken(): Promise<string | null> {
	try {
		const response = await fetch('/api/auth/csrf');
		if (!response.ok) {
			// Not authenticated or other error
			cachedToken = null;
			consecutiveRefreshFailures++;
			return null;
		}
		const data = await response.json();
		cachedToken = data.csrf_token || null;
		// Reset circuit breaker on success
		consecutiveRefreshFailures = 0;
		return cachedToken;
	} catch (error) {
		console.error('Failed to fetch CSRF token:', error);
		cachedToken = null;
		consecutiveRefreshFailures++;
		return null;
	}
}

/**
 * Get the cached CSRF token, or fetch it if not cached.
 */
export async function getCsrfToken(): Promise<string | null> {
	if (cachedToken) {
		return cachedToken;
	}
	return fetchCsrfToken();
}

/**
 * Clear the cached CSRF token.
 * Call this on logout or when the session changes.
 * Also resets the circuit breaker.
 */
export function clearCsrfToken(): void {
	cachedToken = null;
	consecutiveRefreshFailures = 0;
}

/**
 * Set the CSRF token directly (e.g., when received from login response).
 * Also resets the circuit breaker.
 */
export function setCsrfToken(token: string): void {
	cachedToken = token;
	consecutiveRefreshFailures = 0;
}

/**
 * Create headers object with CSRF token for fetch requests.
 * Use this for state-changing requests (POST, PUT, DELETE, PATCH).
 *
 * @param additionalHeaders - Additional headers to merge
 * @returns Headers object with CSRF token if available
 */
export async function createCsrfHeaders(
	additionalHeaders?: Record<string, string>
): Promise<Record<string, string>> {
	const token = await getCsrfToken();
	const headers: Record<string, string> = {
		...additionalHeaders,
	};
	if (token) {
		headers[CSRF_HEADER] = token;
	}
	return headers;
}

/**
 * Enhanced fetch wrapper that automatically includes CSRF token for state-changing requests.
 * Use this instead of regular fetch for API calls.
 *
 * @param url - The URL to fetch
 * @param options - Fetch options
 * @returns Promise with the fetch response
 */
export async function csrfFetch(
	url: string,
	options: RequestInit = {}
): Promise<Response> {
	const method = (options.method || 'GET').toUpperCase();

	// Only add CSRF token for state-changing methods
	if (['POST', 'PUT', 'DELETE', 'PATCH'].includes(method)) {
		const token = await getCsrfToken();
		if (token) {
			const headers = new Headers(options.headers);
			headers.set(CSRF_HEADER, token);
			options = { ...options, headers };
		}
	}

	const response = await fetch(url, options);

	// If we get a 403 with CSRF error, try refreshing the token and retry once
	// Circuit breaker: skip retry if we've had too many consecutive failures
	if (response.status === 403 && consecutiveRefreshFailures < MAX_REFRESH_FAILURES) {
		const data = await response.clone().json().catch(() => ({}));
		if (data.error && data.error.toLowerCase().includes('csrf')) {
			// Refresh token and retry
			await fetchCsrfToken();
			const token = cachedToken;
			if (token) {
				const headers = new Headers(options.headers);
				headers.set(CSRF_HEADER, token);
				return fetch(url, { ...options, headers });
			}
		}
	}

	return response;
}
