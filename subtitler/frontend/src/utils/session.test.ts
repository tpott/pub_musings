import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
	getOrCreateSessionId,
	getSessionId,
	clearSessionId,
	hasSessionId,
	hasCookieConsent,
	hasCookieDeclined,
	hasCookieChoice,
	recordUploadSession,
	removeUploadSessionRecord,
	cleanupStaleUploadSessions,
	getMaxUploadSessions,
} from './session';

// Mock localStorage
const localStorageMock = (() => {
	let store: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => store[key] || null),
		setItem: vi.fn((key: string, value: string) => {
			store[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete store[key];
		}),
		clear: vi.fn(() => {
			store = {};
		}),
		// Helper to set values directly for testing
		_setStore: (newStore: Record<string, string>) => {
			store = { ...newStore };
		},
		_getStore: () => ({ ...store }),
	};
})();

// Mock crypto.getRandomValues
const cryptoMock = {
	getRandomValues: vi.fn((array: Uint8Array) => {
		for (let i = 0; i < array.length; i++) {
			array[i] = i + 1; // Deterministic values for testing
		}
		return array;
	}),
};

describe('session utilities', () => {
	beforeEach(() => {
		// Clear the internal store
		localStorageMock.clear();
		// Reset all spy call counts
		vi.clearAllMocks();
		vi.stubGlobal('localStorage', localStorageMock);
		vi.stubGlobal('crypto', cryptoMock);
	});

	describe('cookie consent functions', () => {
		it('hasCookieConsent should return false when no consent given', () => {
			expect(hasCookieConsent()).toBe(false);
		});

		it('hasCookieConsent should return true when consent is accepted', () => {
			localStorageMock._setStore({ 'subtitler:cookie_consent': 'accepted' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			expect(hasCookieConsent()).toBe(true);
		});

		it('hasCookieConsent should return false when consent is declined', () => {
			localStorageMock._setStore({ 'subtitler:cookie_consent': 'declined' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			expect(hasCookieConsent()).toBe(false);
		});

		it('hasCookieDeclined should return true when consent is declined', () => {
			localStorageMock._setStore({ 'subtitler:cookie_consent': 'declined' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			expect(hasCookieDeclined()).toBe(true);
		});

		it('hasCookieChoice should return false when no choice made', () => {
			expect(hasCookieChoice()).toBe(false);
		});

		it('hasCookieChoice should return true when accepted', () => {
			localStorageMock._setStore({ 'subtitler:cookie_consent': 'accepted' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			expect(hasCookieChoice()).toBe(true);
		});

		it('hasCookieChoice should return true when declined', () => {
			localStorageMock._setStore({ 'subtitler:cookie_consent': 'declined' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			expect(hasCookieChoice()).toBe(true);
		});
	});

	describe('getOrCreateSessionId', () => {
		it('should return empty string when no consent given', () => {
			const sessionId = getOrCreateSessionId();
			expect(sessionId).toBe('');
			expect(localStorageMock.setItem).not.toHaveBeenCalledWith(
				'subtitler:session_id',
				expect.any(String)
			);
		});

		it('should create a new session ID when consent given and none exists', () => {
			localStorageMock._setStore({ 'subtitler:cookie_consent': 'accepted' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			const sessionId = getOrCreateSessionId();
			expect(sessionId).toBe('0102030405060708090a0b0c0d0e0f10');
			expect(localStorageMock.setItem).toHaveBeenCalledWith(
				'subtitler:session_id',
				'0102030405060708090a0b0c0d0e0f10'
			);
		});

		it('should return existing session ID when consent given and one exists', () => {
			localStorageMock._setStore({
				'subtitler:cookie_consent': 'accepted',
				'subtitler:session_id': 'existing-session-id',
			});
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			const sessionId = getOrCreateSessionId();
			expect(sessionId).toBe('existing-session-id');
		});

		it('should generate 32-character hex string when consent given', () => {
			localStorageMock._setStore({ 'subtitler:cookie_consent': 'accepted' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			const sessionId = getOrCreateSessionId();
			expect(sessionId).toMatch(/^[0-9a-f]{32}$/);
		});
	});

	describe('getSessionId', () => {
		it('should return null when no consent given', () => {
			localStorageMock._setStore({ 'subtitler:session_id': 'test-session' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			const sessionId = getSessionId();
			expect(sessionId).toBeNull();
		});

		it('should return null when consent given but no session exists', () => {
			localStorageMock._setStore({ 'subtitler:cookie_consent': 'accepted' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			const sessionId = getSessionId();
			expect(sessionId).toBeNull();
		});

		it('should return existing session ID when consent given', () => {
			localStorageMock._setStore({
				'subtitler:cookie_consent': 'accepted',
				'subtitler:session_id': 'test-session',
			});
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			const sessionId = getSessionId();
			expect(sessionId).toBe('test-session');
		});
	});

	describe('clearSessionId', () => {
		it('should remove session ID from localStorage', () => {
			clearSessionId();
			expect(localStorageMock.removeItem).toHaveBeenCalledWith('subtitler:session_id');
		});
	});

	describe('hasSessionId', () => {
		it('should return false when no consent given even if session exists', () => {
			localStorageMock._setStore({ 'subtitler:session_id': 'test-session' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			expect(hasSessionId()).toBe(false);
		});

		it('should return false when consent given but no session exists', () => {
			localStorageMock._setStore({ 'subtitler:cookie_consent': 'accepted' });
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			expect(hasSessionId()).toBe(false);
		});

		it('should return true when consent given and session exists', () => {
			localStorageMock._setStore({
				'subtitler:cookie_consent': 'accepted',
				'subtitler:session_id': 'test-session',
			});
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			expect(hasSessionId()).toBe(true);
		});
	});

	describe('upload session cleanup', () => {
		beforeEach(() => {
			// Use real implementation for these tests
			localStorageMock.getItem.mockImplementation((key: string) =>
				localStorageMock._getStore()[key] || null
			);
			localStorageMock.setItem.mockImplementation((key: string, value: string) => {
				const store = localStorageMock._getStore();
				store[key] = value;
				localStorageMock._setStore(store);
			});
			localStorageMock.removeItem.mockImplementation((key: string) => {
				const store = localStorageMock._getStore();
				delete store[key];
				localStorageMock._setStore(store);
			});
		});

		describe('recordUploadSession', () => {
			it('should record session timestamp', () => {
				const sessionKey = 'subtitler:upload_session:test.mp4:12345';
				recordUploadSession(sessionKey);

				const timestamps = JSON.parse(
					localStorageMock._getStore()['subtitler:upload_session_timestamps'] || '{}'
				);
				expect(timestamps[sessionKey]).toBeDefined();
				expect(typeof timestamps[sessionKey]).toBe('number');
			});

			it('should record multiple sessions', () => {
				const session1 = 'subtitler:upload_session:file1.mp4:100';
				const session2 = 'subtitler:upload_session:file2.mp4:200';

				recordUploadSession(session1);
				recordUploadSession(session2);

				const timestamps = JSON.parse(
					localStorageMock._getStore()['subtitler:upload_session_timestamps'] || '{}'
				);
				expect(timestamps[session1]).toBeDefined();
				expect(timestamps[session2]).toBeDefined();
			});

			it('should enforce max sessions limit by removing oldest', () => {
				const maxSessions = getMaxUploadSessions();

				// Create more sessions than the max
				const sessions: string[] = [];
				for (let i = 0; i < maxSessions + 5; i++) {
					const sessionKey = `subtitler:upload_session:file${i}.mp4:${i}`;
					sessions.push(sessionKey);
					// Record session and also store a value for it (like the actual upload would)
					const store = localStorageMock._getStore();
					store[sessionKey] = `session-id-${i}`;
					localStorageMock._setStore(store);
					recordUploadSession(sessionKey);
				}

				// Verify we're at the max limit
				const timestamps = JSON.parse(
					localStorageMock._getStore()['subtitler:upload_session_timestamps'] || '{}'
				);
				expect(Object.keys(timestamps).length).toBe(maxSessions);

				// Verify oldest sessions were removed (first 5)
				for (let i = 0; i < 5; i++) {
					expect(timestamps[sessions[i]]).toBeUndefined();
					expect(localStorageMock._getStore()[sessions[i]]).toBeUndefined();
				}

				// Verify newest sessions are kept
				for (let i = 5; i < maxSessions + 5; i++) {
					expect(timestamps[sessions[i]]).toBeDefined();
				}
			});

			it('should not remove sessions when under the limit', () => {
				const maxSessions = getMaxUploadSessions();

				// Create sessions under the limit
				const sessions: string[] = [];
				for (let i = 0; i < maxSessions - 5; i++) {
					const sessionKey = `subtitler:upload_session:file${i}.mp4:${i}`;
					sessions.push(sessionKey);
					recordUploadSession(sessionKey);
				}

				// Verify all sessions are kept
				const timestamps = JSON.parse(
					localStorageMock._getStore()['subtitler:upload_session_timestamps'] || '{}'
				);
				expect(Object.keys(timestamps).length).toBe(maxSessions - 5);

				for (const session of sessions) {
					expect(timestamps[session]).toBeDefined();
				}
			});
		});

		describe('getMaxUploadSessions', () => {
			it('should return 50', () => {
				expect(getMaxUploadSessions()).toBe(50);
			});
		});

		describe('removeUploadSessionRecord', () => {
			it('should remove session timestamp', () => {
				const sessionKey = 'subtitler:upload_session:test.mp4:12345';
				recordUploadSession(sessionKey);
				removeUploadSessionRecord(sessionKey);

				const timestamps = JSON.parse(
					localStorageMock._getStore()['subtitler:upload_session_timestamps'] || '{}'
				);
				expect(timestamps[sessionKey]).toBeUndefined();
			});

			it('should not affect other sessions', () => {
				const session1 = 'subtitler:upload_session:file1.mp4:100';
				const session2 = 'subtitler:upload_session:file2.mp4:200';

				recordUploadSession(session1);
				recordUploadSession(session2);
				removeUploadSessionRecord(session1);

				const timestamps = JSON.parse(
					localStorageMock._getStore()['subtitler:upload_session_timestamps'] || '{}'
				);
				expect(timestamps[session1]).toBeUndefined();
				expect(timestamps[session2]).toBeDefined();
			});
		});

		describe('cleanupStaleUploadSessions', () => {
			it('should remove sessions older than 48 hours', () => {
				const sessionKey = 'subtitler:upload_session:old.mp4:12345';
				const staleTimestamp = Date.now() - 49 * 60 * 60 * 1000; // 49 hours ago

				// Set up stale session
				const store = localStorageMock._getStore();
				store[sessionKey] = 'some-session-id';
				store['subtitler:upload_session_timestamps'] = JSON.stringify({
					[sessionKey]: staleTimestamp,
				});
				localStorageMock._setStore(store);

				cleanupStaleUploadSessions();

				const finalStore = localStorageMock._getStore();
				expect(finalStore[sessionKey]).toBeUndefined();
			});

			it('should keep sessions younger than 48 hours', () => {
				const sessionKey = 'subtitler:upload_session:new.mp4:12345';
				const freshTimestamp = Date.now() - 1 * 60 * 60 * 1000; // 1 hour ago

				// Set up fresh session
				const store = localStorageMock._getStore();
				store[sessionKey] = 'some-session-id';
				store['subtitler:upload_session_timestamps'] = JSON.stringify({
					[sessionKey]: freshTimestamp,
				});
				localStorageMock._setStore(store);

				cleanupStaleUploadSessions();

				const finalStore = localStorageMock._getStore();
				expect(finalStore[sessionKey]).toBe('some-session-id');
			});

			it('should remove untracked legacy sessions', () => {
				const sessionKey = 'subtitler:upload_session:legacy.mp4:12345';

				// Set up untracked session (no timestamp)
				const store = localStorageMock._getStore();
				store[sessionKey] = 'some-session-id';
				store['subtitler:upload_session_timestamps'] = '{}';
				localStorageMock._setStore(store);

				// Mock localStorage.key and length for iteration
				const keys = Object.keys(store);
				Object.defineProperty(localStorageMock, 'length', {
					get: () => keys.length,
					configurable: true,
				});
				localStorageMock.key = vi.fn((index: number) => keys[index] || null);

				cleanupStaleUploadSessions();

				const finalStore = localStorageMock._getStore();
				expect(finalStore[sessionKey]).toBeUndefined();
			});

			it('should handle both stale and fresh sessions correctly', () => {
				const staleKey = 'subtitler:upload_session:old.mp4:100';
				const freshKey = 'subtitler:upload_session:new.mp4:200';
				const staleTimestamp = Date.now() - 50 * 60 * 60 * 1000; // 50 hours ago
				const freshTimestamp = Date.now() - 1 * 60 * 60 * 1000; // 1 hour ago

				// Set up both sessions
				const store = localStorageMock._getStore();
				store[staleKey] = 'stale-session-id';
				store[freshKey] = 'fresh-session-id';
				store['subtitler:upload_session_timestamps'] = JSON.stringify({
					[staleKey]: staleTimestamp,
					[freshKey]: freshTimestamp,
				});
				localStorageMock._setStore(store);

				cleanupStaleUploadSessions();

				const finalStore = localStorageMock._getStore();
				expect(finalStore[staleKey]).toBeUndefined();
				expect(finalStore[freshKey]).toBe('fresh-session-id');

				// Verify timestamps updated
				const timestamps = JSON.parse(finalStore['subtitler:upload_session_timestamps']);
				expect(timestamps[staleKey]).toBeUndefined();
				expect(timestamps[freshKey]).toBeDefined();
			});

			it('should handle invalid JSON in timestamps gracefully', () => {
				const sessionKey = 'subtitler:upload_session:test.mp4:12345';

				// Set up invalid JSON
				const store = localStorageMock._getStore();
				store[sessionKey] = 'some-session-id';
				store['subtitler:upload_session_timestamps'] = 'invalid-json';
				localStorageMock._setStore(store);

				// Mock localStorage.key and length for iteration
				const keys = Object.keys(store);
				Object.defineProperty(localStorageMock, 'length', {
					get: () => keys.length,
					configurable: true,
				});
				localStorageMock.key = vi.fn((index: number) => keys[index] || null);

				// Should not throw
				expect(() => cleanupStaleUploadSessions()).not.toThrow();

				// Untracked session should be removed
				const finalStore = localStorageMock._getStore();
				expect(finalStore[sessionKey]).toBeUndefined();
			});
		});
	});
});
