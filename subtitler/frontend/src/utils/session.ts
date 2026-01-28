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

/**
 * Upload session tracking constants
 */
const UPLOAD_SESSION_PREFIX = 'subtitler:upload_session:';
const UPLOAD_SESSION_TIMESTAMPS_KEY = 'subtitler:upload_session_timestamps';
const STALE_SESSION_THRESHOLD_MS = 48 * 60 * 60 * 1000; // 48 hours
const MAX_UPLOAD_SESSIONS = 50; // Prevent localStorage pollution from many tabs

/**
 * Returns the maximum number of concurrent upload sessions allowed.
 * Exported for testing purposes.
 */
export function getMaxUploadSessions(): number {
	return MAX_UPLOAD_SESSIONS;
}

/**
 * Records the creation time for an upload session.
 * Call this when storing a new upload session ID.
 * Enforces a maximum of MAX_UPLOAD_SESSIONS to prevent localStorage pollution.
 * When the limit is reached, the oldest sessions are removed.
 */
export function recordUploadSession(sessionKey: string): void {
	const timestamps = getUploadSessionTimestamps();
	timestamps[sessionKey] = Date.now();

	// Enforce max sessions limit
	const sessionKeys = Object.keys(timestamps);
	if (sessionKeys.length > MAX_UPLOAD_SESSIONS) {
		// Sort by timestamp (oldest first)
		const sortedKeys = sessionKeys.sort((a, b) => timestamps[a] - timestamps[b]);

		// Remove oldest sessions until we're at the limit
		const toRemove = sortedKeys.slice(0, sessionKeys.length - MAX_UPLOAD_SESSIONS);
		for (const oldKey of toRemove) {
			localStorage.removeItem(oldKey);
			delete timestamps[oldKey];
		}
	}

	localStorage.setItem(UPLOAD_SESSION_TIMESTAMPS_KEY, JSON.stringify(timestamps));
}

/**
 * Removes the timestamp record for an upload session.
 * Call this when an upload completes or is abandoned.
 */
export function removeUploadSessionRecord(sessionKey: string): void {
	const timestamps = getUploadSessionTimestamps();
	delete timestamps[sessionKey];
	localStorage.setItem(UPLOAD_SESSION_TIMESTAMPS_KEY, JSON.stringify(timestamps));
}

/**
 * Gets all upload session timestamps.
 */
function getUploadSessionTimestamps(): Record<string, number> {
	try {
		const stored = localStorage.getItem(UPLOAD_SESSION_TIMESTAMPS_KEY);
		if (stored) {
			return JSON.parse(stored);
		}
	} catch {
		// Invalid JSON, start fresh
	}
	return {};
}

/**
 * Cleans up stale upload sessions from localStorage.
 * Removes sessions older than 48 hours.
 * Call this on app startup.
 */
export function cleanupStaleUploadSessions(): void {
	const now = Date.now();
	const timestamps = getUploadSessionTimestamps();
	let hasChanges = false;

	// Check each tracked session
	for (const sessionKey of Object.keys(timestamps)) {
		const createdAt = timestamps[sessionKey];
		const ageMs = now - createdAt;

		if (ageMs > STALE_SESSION_THRESHOLD_MS) {
			// Session is stale, remove it
			localStorage.removeItem(sessionKey);
			delete timestamps[sessionKey];
			hasChanges = true;
		}
	}

	// Also scan for any upload session keys not in our timestamps tracking
	// (handles sessions created before this feature was added)
	for (let i = 0; i < localStorage.length; i++) {
		const key = localStorage.key(i);
		if (key && key.startsWith(UPLOAD_SESSION_PREFIX) && !(key in timestamps)) {
			// Untracked session - remove it since we don't know when it was created
			// This ensures cleanup of legacy sessions
			localStorage.removeItem(key);
			hasChanges = true;
		}
	}

	if (hasChanges) {
		localStorage.setItem(UPLOAD_SESSION_TIMESTAMPS_KEY, JSON.stringify(timestamps));
	}
}
