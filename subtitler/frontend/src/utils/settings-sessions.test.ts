import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
	setupSessions,
	type SessionsElements,
	type SessionsCallbacks,
} from './settings-sessions';

// --- Mocks ---

vi.mock('./html', () => ({
	escapeHtml: vi.fn((s: string) => s),
}));

vi.mock('./csrf', () => ({
	csrfFetch: vi.fn(),
}));

vi.mock('./api-schemas', () => ({
	SessionListResponseSchema: {},
	safeParse: vi.fn((_schema: unknown, data: unknown) => data),
}));

import { csrfFetch } from './csrf';
import { safeParse } from './api-schemas';

const mockCsrfFetch = vi.mocked(csrfFetch);
const mockSafeParse = vi.mocked(safeParse);

function makeMockElements(): SessionsElements {
	return {
		sessionsLoading: {
			textContent: '',
			classList: { add: vi.fn(), remove: vi.fn() },
		} as unknown as HTMLElement,
		sessionsList: {
			innerHTML: '',
			classList: { add: vi.fn(), remove: vi.fn() },
			addEventListener: vi.fn(),
		} as unknown as HTMLElement,
	};
}

function makeMockCallbacks(): SessionsCallbacks {
	return {
		showError: vi.fn(),
		showSuccess: vi.fn(),
	};
}

function makeSession(overrides: Record<string, unknown> = {}) {
	return {
		id: 'sess-1',
		ip_address: '192.168.1.1',
		user_agent: 'Mozilla/5.0 Chrome/120',
		created_at: '2026-01-15T10:00:00Z',
		expires_at: '2026-01-16T10:00:00Z',
		is_current: false,
		...overrides,
	};
}

describe('settings-sessions', () => {
	let els: SessionsElements;
	let callbacks: SessionsCallbacks;

	beforeEach(() => {
		vi.clearAllMocks();
		els = makeMockElements();
		callbacks = makeMockCallbacks();
		// Default: fetch returns ok with sessions
		vi.stubGlobal('fetch', vi.fn());
		vi.stubGlobal('setTimeout', vi.fn((_fn: () => void, _ms: number) => 1));
		vi.stubGlobal('clearTimeout', vi.fn());
	});

	describe('setupSessions', () => {
		it('should register click listener on sessionsList for delegation', () => {
			setupSessions(els, callbacks);
			expect(els.sessionsList.addEventListener).toHaveBeenCalledWith(
				'click',
				expect.any(Function)
			);
		});

		it('should return object with loadSessions function', () => {
			const result = setupSessions(els, callbacks);
			expect(result).toHaveProperty('loadSessions');
			expect(typeof result.loadSessions).toBe('function');
		});
	});

	describe('loadSessions', () => {
		it('should fetch sessions from /api/auth/sessions', async () => {
			const mockFetch = vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [] }),
			});
			vi.stubGlobal('fetch', mockFetch);

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(mockFetch).toHaveBeenCalledWith('/api/auth/sessions', {
				signal: expect.any(AbortSignal),
			});
		});

		it('should set timeout for abort controller', async () => {
			const mockSetTimeout = vi.fn((_fn: () => void, _ms: number) => 42);
			vi.stubGlobal('setTimeout', mockSetTimeout);
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [] }),
			}));

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(mockSetTimeout).toHaveBeenCalledWith(expect.any(Function), 30000);
		});

		it('should clear timeout after successful fetch', async () => {
			const mockClearTimeout = vi.fn();
			vi.stubGlobal('clearTimeout', mockClearTimeout);
			vi.stubGlobal('setTimeout', vi.fn(() => 99));
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [] }),
			}));

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(mockClearTimeout).toHaveBeenCalledWith(99);
		});

		it('should show error when response is not ok', async () => {
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: false,
				json: () => Promise.resolve({}),
			}));

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsLoading.textContent).toBe('Failed to load sessions');
		});

		it('should show error when safeParse returns null', async () => {
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [] }),
			}));
			mockSafeParse.mockReturnValue(null);

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsLoading.textContent).toBe('Invalid response from server');
		});

		it('should hide loading and show list on success', async () => {
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [makeSession()] }),
			}));
			mockSafeParse.mockImplementation((_s, d) => d as ReturnType<typeof mockSafeParse>);

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(
				(els.sessionsLoading.classList as unknown as { add: ReturnType<typeof vi.fn> }).add
			).toHaveBeenCalledWith('hidden');
			expect(
				(els.sessionsList.classList as unknown as { remove: ReturnType<typeof vi.fn> }).remove
			).toHaveBeenCalledWith('hidden');
		});

		it('should show no sessions message when list is empty', async () => {
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [] }),
			}));
			mockSafeParse.mockImplementation((_s, d) => d as ReturnType<typeof mockSafeParse>);

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsList.innerHTML).toContain('No active sessions found');
		});

		it('should render session items with correct data', async () => {
			const session = makeSession({
				ip_address: '10.0.0.1',
				user_agent: 'Mozilla/5.0 Chrome/120',
			});
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [session] }),
			}));
			mockSafeParse.mockImplementation((_s, d) => d as ReturnType<typeof mockSafeParse>);

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsList.innerHTML).toContain('Chrome Browser');
			expect(els.sessionsList.innerHTML).toContain('10.0.0.1');
			expect(els.sessionsList.innerHTML).toContain('btn-revoke');
		});

		it('should mark current session with current class and badge', async () => {
			const session = makeSession({ is_current: true });
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [session] }),
			}));
			mockSafeParse.mockImplementation((_s, d) => d as ReturnType<typeof mockSafeParse>);

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsList.innerHTML).toContain('current');
			expect(els.sessionsList.innerHTML).toContain('session-current-badge');
		});

		it('should not show revoke button for current session', async () => {
			const session = makeSession({ is_current: true });
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [session] }),
			}));
			mockSafeParse.mockImplementation((_s, d) => d as ReturnType<typeof mockSafeParse>);

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsList.innerHTML).not.toContain('btn-revoke');
		});

		it('should show revoke button for non-current session', async () => {
			const session = makeSession({ is_current: false });
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [session] }),
			}));
			mockSafeParse.mockImplementation((_s, d) => d as ReturnType<typeof mockSafeParse>);

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsList.innerHTML).toContain('btn-revoke');
			expect(els.sessionsList.innerHTML).toContain('data-session-id="sess-1"');
		});

		it('should display Unknown IP when ip_address is empty', async () => {
			const session = makeSession({ ip_address: '' });
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [session] }),
			}));
			mockSafeParse.mockImplementation((_s, d) => d as ReturnType<typeof mockSafeParse>);

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsList.innerHTML).toContain('Unknown IP');
		});

		it('should handle fetch abort error', async () => {
			const abortError = new Error('AbortError');
			abortError.name = 'AbortError';
			vi.stubGlobal('fetch', vi.fn().mockRejectedValue(abortError));

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsLoading.textContent).toBe('Request timed out. Please try again.');
		});

		it('should handle generic fetch error', async () => {
			vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Network error')));

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsLoading.textContent).toBe('Failed to load sessions');
		});

		it('should render multiple sessions', async () => {
			const sessions = [
				makeSession({ id: 'sess-1', is_current: true }),
				makeSession({ id: 'sess-2', is_current: false }),
				makeSession({ id: 'sess-3', is_current: false }),
			];
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions }),
			}));
			mockSafeParse.mockImplementation((_s, d) => d as ReturnType<typeof mockSafeParse>);

			const { loadSessions } = setupSessions(els, callbacks);
			await loadSessions();

			expect(els.sessionsList.innerHTML).toContain('data-id="sess-1"');
			expect(els.sessionsList.innerHTML).toContain('data-id="sess-2"');
			expect(els.sessionsList.innerHTML).toContain('data-id="sess-3"');
		});
	});

	describe('parseUserAgent (via rendering)', () => {
		async function renderSessionWithUA(ua: string): Promise<string> {
			const session = makeSession({ user_agent: ua });
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [session] }),
			}));
			mockSafeParse.mockImplementation((_s, d) => d as ReturnType<typeof mockSafeParse>);
			const newEls = makeMockElements();
			const { loadSessions } = setupSessions(newEls, callbacks);
			await loadSessions();
			return newEls.sessionsList.innerHTML;
		}

		it('should identify Chrome browser', async () => {
			const html = await renderSessionWithUA('Mozilla/5.0 (X11; Linux) Chrome/120');
			expect(html).toContain('Chrome Browser');
		});

		it('should identify Firefox browser', async () => {
			const html = await renderSessionWithUA('Mozilla/5.0 (X11; Linux) Firefox/120');
			expect(html).toContain('Firefox Browser');
		});

		it('should identify Safari browser (not Chrome)', async () => {
			const html = await renderSessionWithUA('Mozilla/5.0 (Macintosh) Safari/537');
			expect(html).toContain('Safari Browser');
		});

		it('should identify Edge browser', async () => {
			const html = await renderSessionWithUA('Mozilla/5.0 Edge/120');
			expect(html).toContain('Edge Browser');
		});

		it('should identify Internet Explorer via MSIE', async () => {
			const html = await renderSessionWithUA('Mozilla/4.0 (compatible; MSIE 11.0)');
			expect(html).toContain('Internet Explorer');
		});

		it('should identify Internet Explorer via Trident', async () => {
			const html = await renderSessionWithUA('Mozilla/5.0 (Trident/7.0) like Gecko');
			expect(html).toContain('Internet Explorer');
		});

		it('should show Unknown device for empty user agent', async () => {
			const html = await renderSessionWithUA('');
			expect(html).toContain('Unknown device');
		});

		it('should show Web Browser for unrecognized user agent', async () => {
			const html = await renderSessionWithUA('CustomBot/1.0');
			expect(html).toContain('Web Browser');
		});
	});

	describe('revoke session', () => {
		async function setupWithSession() {
			const session = makeSession({ id: 'sess-revoke', is_current: false });
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({ sessions: [session] }),
			}));
			mockSafeParse.mockImplementation((_s, d) => d as ReturnType<typeof mockSafeParse>);

			const newEls = makeMockElements();
			const newCallbacks = makeMockCallbacks();
			const { loadSessions } = setupSessions(newEls, newCallbacks);
			await loadSessions();

			// Get the click handler registered on sessionsList
			const addEventListenerMock = vi.mocked(newEls.sessionsList.addEventListener);
			const clickHandler = addEventListenerMock.mock.calls.find(
				(call) => call[0] === 'click'
			)?.[1] as (e: Event) => Promise<void>;

			return { els: newEls, callbacks: newCallbacks, clickHandler, loadSessions };
		}

		it('should call csrfFetch with DELETE method on revoke', async () => {
			const { clickHandler } = await setupWithSession();

			mockCsrfFetch.mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({}),
			} as Response);

			// Simulate clicking a revoke button
			const mockBtn = {
				classList: { contains: vi.fn((cls: string) => cls === 'btn-revoke') },
				dataset: { sessionId: 'sess-revoke' },
				disabled: false,
				textContent: 'Revoke',
			};

			await clickHandler({ target: mockBtn } as unknown as Event);

			expect(mockCsrfFetch).toHaveBeenCalledWith('/api/auth/sessions/sess-revoke', {
				method: 'DELETE',
			});
		});

		it('should disable button while revoking', async () => {
			const { clickHandler } = await setupWithSession();

			// Make csrfFetch hang so we can check intermediate state
			let resolveRevoke!: (value: Response) => void;
			mockCsrfFetch.mockReturnValue(
				new Promise<Response>((resolve) => {
					resolveRevoke = resolve;
				})
			);

			const mockBtn = {
				classList: { contains: vi.fn((cls: string) => cls === 'btn-revoke') },
				dataset: { sessionId: 'sess-revoke' },
				disabled: false,
				textContent: 'Revoke',
			};

			const promise = clickHandler({ target: mockBtn } as unknown as Event);

			expect(mockBtn.disabled).toBe(true);
			expect(mockBtn.textContent).toBe('Revoking...');

			resolveRevoke({ ok: true, json: () => Promise.resolve({}) } as Response);
			await promise;
		});

		it('should show success message on successful revoke', async () => {
			const { clickHandler, callbacks: cb } = await setupWithSession();

			mockCsrfFetch.mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({}),
			} as Response);

			const mockBtn = {
				classList: { contains: vi.fn((cls: string) => cls === 'btn-revoke') },
				dataset: { sessionId: 'sess-revoke' },
				disabled: false,
				textContent: 'Revoke',
			};

			await clickHandler({ target: mockBtn } as unknown as Event);

			expect(cb.showSuccess).toHaveBeenCalledWith('Session revoked successfully');
		});

		it('should show error when revoke fails', async () => {
			const { clickHandler, callbacks: cb } = await setupWithSession();

			mockCsrfFetch.mockResolvedValue({
				ok: false,
				json: () => Promise.resolve({ error: 'Session not found' }),
			} as unknown as Response);

			const mockBtn = {
				classList: { contains: vi.fn((cls: string) => cls === 'btn-revoke') },
				dataset: { sessionId: 'sess-revoke' },
				disabled: false,
				textContent: 'Revoke',
			};

			await clickHandler({ target: mockBtn } as unknown as Event);

			expect(cb.showError).toHaveBeenCalledWith('Session not found');
			expect(mockBtn.disabled).toBe(false);
			expect(mockBtn.textContent).toBe('Revoke');
		});

		it('should show default error message when no error field', async () => {
			const { clickHandler, callbacks: cb } = await setupWithSession();

			mockCsrfFetch.mockResolvedValue({
				ok: false,
				json: () => Promise.resolve({}),
			} as unknown as Response);

			const mockBtn = {
				classList: { contains: vi.fn((cls: string) => cls === 'btn-revoke') },
				dataset: { sessionId: 'sess-revoke' },
				disabled: false,
				textContent: 'Revoke',
			};

			await clickHandler({ target: mockBtn } as unknown as Event);

			expect(cb.showError).toHaveBeenCalledWith('Failed to revoke session');
		});

		it('should handle network error during revoke', async () => {
			const { clickHandler, callbacks: cb } = await setupWithSession();

			mockCsrfFetch.mockRejectedValue(new Error('Network error'));

			const mockBtn = {
				classList: { contains: vi.fn((cls: string) => cls === 'btn-revoke') },
				dataset: { sessionId: 'sess-revoke' },
				disabled: false,
				textContent: 'Revoke',
			};

			await clickHandler({ target: mockBtn } as unknown as Event);

			expect(cb.showError).toHaveBeenCalledWith('Network error. Please try again.');
			expect(mockBtn.disabled).toBe(false);
			expect(mockBtn.textContent).toBe('Revoke');
		});

		it('should ignore clicks on non-revoke elements', async () => {
			const { clickHandler } = await setupWithSession();

			const mockTarget = {
				classList: { contains: vi.fn(() => false) },
			};

			await clickHandler({ target: mockTarget } as unknown as Event);

			expect(mockCsrfFetch).not.toHaveBeenCalled();
		});

		it('should ignore clicks without session ID', async () => {
			const { clickHandler } = await setupWithSession();

			const mockBtn = {
				classList: { contains: vi.fn((cls: string) => cls === 'btn-revoke') },
				dataset: {},
				disabled: false,
				textContent: 'Revoke',
			};

			await clickHandler({ target: mockBtn } as unknown as Event);

			expect(mockCsrfFetch).not.toHaveBeenCalled();
		});
	});
});
