/**
 * CSRF token management for authenticated requests.
 *
 * The backend requires an X-CSRF-Token header on all state-changing requests
 * (POST, PUT, DELETE, PATCH) from authenticated users. The token is derived
 * from the session cookie via HMAC-SHA256, so it stays constant for a session.
 *
 * Usage: call fetchCSRFToken() after login, then getCSRFToken() returns the
 * cached value synchronously. Include the header in requests via getCSRFHeaders().
 */

import { logger } from './logger';

let cachedToken: string | null = null;

/**
 * Fetch the CSRF token from the backend and cache it.
 * Call this after successful login. Returns the token string,
 * or null if the user is not authenticated.
 */
export async function fetchCSRFToken(): Promise<string | null> {
  try {
    const response = await fetch('/api/auth/csrf', {
      credentials: 'same-origin',
    });

    if (!response.ok) {
      // Not authenticated or server error — no CSRF token needed
      cachedToken = null;
      return null;
    }

    let data: { token?: string };
    try {
      data = await response.json();
    } catch {
      logger.debug('Failed to parse CSRF response as JSON');
      cachedToken = null;
      return null;
    }

    if (data.token) {
      cachedToken = data.token;
      return cachedToken;
    }

    cachedToken = null;
    return null;
  } catch {
    // Network error — silently fail, CSRF will be retried on next call
    logger.debug('Failed to fetch CSRF token');
    cachedToken = null;
    return null;
  }
}

/**
 * Get the cached CSRF token. Returns null if not yet fetched
 * or if the user is not authenticated.
 */
export function getCSRFToken(): string | null {
  return cachedToken;
}

/**
 * Returns headers object with X-CSRF-Token if a token is cached.
 * Merges with any existing headers. Use this to add CSRF protection
 * to fetch requests.
 */
export function getCSRFHeaders(existingHeaders?: HeadersInit): Record<string, string> {
  const headers: Record<string, string> = {};

  // Copy existing headers
  if (existingHeaders) {
    if (existingHeaders instanceof Headers) {
      existingHeaders.forEach((value, key) => {
        headers[key] = value;
      });
    } else if (Array.isArray(existingHeaders)) {
      for (const [key, value] of existingHeaders) {
        headers[key] = value;
      }
    } else {
      Object.assign(headers, existingHeaders);
    }
  }

  // Add CSRF token if available
  if (cachedToken) {
    headers['X-CSRF-Token'] = cachedToken;
  }

  return headers;
}

/**
 * Clear the cached CSRF token. Call on logout.
 */
export function clearCSRFToken(): void {
  cachedToken = null;
}

/**
 * Reset CSRF state (for testing).
 */
export function resetCSRFState(): void {
  cachedToken = null;
}
