/**
 * Session ID management for anonymous user tracking.
 *
 * Anonymous users are assigned a session_id stored in localStorage.
 * This session_id is used to:
 * - Track which videos belong to the anonymous user
 * - Enforce upload limits (2 uploads max for anonymous)
 * - Filter videos on the My Videos page
 */

const SESSION_ID_KEY = 'subtitler:session_id';

/**
 * Generates a random session ID (32 hex characters)
 */
function generateSessionId(): string {
	const array = new Uint8Array(16);
	crypto.getRandomValues(array);
	return Array.from(array, (byte) => byte.toString(16).padStart(2, '0')).join('');
}

/**
 * Gets the current session ID, creating one if it doesn't exist.
 * Returns the session_id for anonymous users, or empty string if authenticated.
 */
export function getOrCreateSessionId(): string {
	let sessionId = localStorage.getItem(SESSION_ID_KEY);
	if (!sessionId) {
		sessionId = generateSessionId();
		localStorage.setItem(SESSION_ID_KEY, sessionId);
	}
	return sessionId;
}

/**
 * Gets the current session ID without creating one.
 * Returns null if no session exists.
 */
export function getSessionId(): string | null {
	return localStorage.getItem(SESSION_ID_KEY);
}

/**
 * Clears the session ID. Call this when user logs in
 * (registered user videos are tracked by user_id, not session_id).
 */
export function clearSessionId(): void {
	localStorage.removeItem(SESSION_ID_KEY);
}

/**
 * Checks if a session ID exists in localStorage.
 */
export function hasSessionId(): boolean {
	return localStorage.getItem(SESSION_ID_KEY) !== null;
}
