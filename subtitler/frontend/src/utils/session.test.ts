import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
	getOrCreateSessionId,
	getSessionId,
	clearSessionId,
	hasSessionId,
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

	describe('getOrCreateSessionId', () => {
		it('should create a new session ID when none exists', () => {
			const sessionId = getOrCreateSessionId();
			expect(sessionId).toBe('0102030405060708090a0b0c0d0e0f10');
			expect(localStorageMock.setItem).toHaveBeenCalledWith(
				'session_id',
				'0102030405060708090a0b0c0d0e0f10'
			);
		});

		it('should return existing session ID when one exists', () => {
			localStorageMock.getItem.mockReturnValueOnce('existing-session-id');
			const sessionId = getOrCreateSessionId();
			expect(sessionId).toBe('existing-session-id');
			expect(localStorageMock.setItem).not.toHaveBeenCalled();
		});

		it('should generate 32-character hex string', () => {
			const sessionId = getOrCreateSessionId();
			expect(sessionId).toMatch(/^[0-9a-f]{32}$/);
		});
	});

	describe('getSessionId', () => {
		it('should return null when no session exists', () => {
			const sessionId = getSessionId();
			expect(sessionId).toBeNull();
		});

		it('should return existing session ID', () => {
			localStorageMock.getItem.mockReturnValueOnce('test-session');
			const sessionId = getSessionId();
			expect(sessionId).toBe('test-session');
		});
	});

	describe('clearSessionId', () => {
		it('should remove session ID from localStorage', () => {
			clearSessionId();
			expect(localStorageMock.removeItem).toHaveBeenCalledWith('session_id');
		});
	});

	describe('hasSessionId', () => {
		it('should return false when no session exists', () => {
			expect(hasSessionId()).toBe(false);
		});

		it('should return true when session exists', () => {
			localStorageMock.getItem.mockReturnValueOnce('test-session');
			expect(hasSessionId()).toBe(true);
		});
	});
});
