/**
 * Session ID management for anonymous user tracking.
 *
 * Anonymous users are assigned a session_id stored in localStorage.
 * This session_id is used to:
 * - Track which videos belong to the anonymous user
 * - Enforce upload limits (2 uploads max for anonymous)
 * - Filter videos on the My Videos page
 *
 * Session ID creation requires cookie consent.
 */

const SESSION_ID_KEY = 'subtitler:session_id';
const CONSENT_KEY = 'subtitler:cookie_consent';

/**
 * Checks if user has accepted cookie consent.
 */
export function hasCookieConsent(): boolean {
	return localStorage.getItem(CONSENT_KEY) === 'accepted';
}

/**
 * Checks if user has declined cookie consent.
 */
export function hasCookieDeclined(): boolean {
	return localStorage.getItem(CONSENT_KEY) === 'declined';
}

/**
 * Checks if user has made a cookie consent choice (either accepted or declined).
 */
export function hasCookieChoice(): boolean {
	const consent = localStorage.getItem(CONSENT_KEY);
	return consent === 'accepted' || consent === 'declined';
}

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
 * Returns the session_id for anonymous users.
 * Returns empty string if cookie consent has not been given.
 */
export function getOrCreateSessionId(): string {
	// Check for cookie consent before creating/returning session ID
	if (!hasCookieConsent()) {
		return '';
	}

	let sessionId = localStorage.getItem(SESSION_ID_KEY);
	if (!sessionId) {
		sessionId = generateSessionId();
		localStorage.setItem(SESSION_ID_KEY, sessionId);
	}
	return sessionId;
}

/**
 * Gets the current session ID without creating one.
 * Returns null if no session exists or if consent not given.
 */
export function getSessionId(): string | null {
	if (!hasCookieConsent()) {
		return null;
	}
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
	if (!hasCookieConsent()) {
		return false;
	}
	return localStorage.getItem(SESSION_ID_KEY) !== null;
}
