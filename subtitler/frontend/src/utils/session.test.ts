import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
	getOrCreateSessionId,
	getSessionId,
	clearSessionId,
	hasSessionId,
	hasCookieConsent,
	hasCookieDeclined,
	hasCookieChoice,
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
});
