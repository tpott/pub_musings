import { escapeHtml } from './html';
import { csrfFetch } from './csrf';
import { SessionListResponseSchema, safeParse } from './api-schemas';

export interface SessionsElements {
	sessionsLoading: HTMLElement;
	sessionsList: HTMLElement;
}

export interface SessionsCallbacks {
	showError: (message: string) => void;
	showSuccess: (message: string) => void;
}

interface Session {
	id: string;
	ip_address: string;
	user_agent: string;
	created_at: string;
	expires_at: string;
	is_current: boolean;
}

function parseUserAgent(ua: string): string {
	if (!ua) return 'Unknown device';
	if (ua.includes('Chrome')) return 'Chrome Browser';
	if (ua.includes('Firefox')) return 'Firefox Browser';
	if (ua.includes('Safari') && !ua.includes('Chrome')) return 'Safari Browser';
	if (ua.includes('Edge')) return 'Edge Browser';
	if (ua.includes('MSIE') || ua.includes('Trident')) return 'Internet Explorer';
	return 'Web Browser';
}

function formatDate(dateStr: string): string {
	const date = new Date(dateStr);
	return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function setupSessions(els: SessionsElements, callbacks: SessionsCallbacks) {
	async function loadSessions() {
		const controller = new AbortController();
		const timeoutId = setTimeout(() => controller.abort(), 30000);

		try {
			const response = await fetch('/api/auth/sessions', {
				signal: controller.signal,
			});
			clearTimeout(timeoutId);

			if (!response.ok) {
				els.sessionsLoading.textContent = 'Failed to load sessions';
				return;
			}

			const rawData = await response.json();

			const data = safeParse(SessionListResponseSchema, rawData);
			if (!data) {
				els.sessionsLoading.textContent = 'Invalid response from server';
				return;
			}

			const sessions: Session[] = data.sessions;

			els.sessionsLoading.classList.add('hidden');
			els.sessionsList.classList.remove('hidden');

			if (sessions.length === 0) {
				els.sessionsList.innerHTML = '<div class="no-sessions">No active sessions found</div>';
				return;
			}

			els.sessionsList.innerHTML = sessions.map(session => `
				<div class="session-item ${session.is_current ? 'current' : ''}" data-id="${session.id}">
					<div class="session-info">
						<div class="session-device">
							${escapeHtml(parseUserAgent(session.user_agent))}
							${session.is_current ? '<span class="session-current-badge">Current</span>' : ''}
						</div>
						<div class="session-meta">
							${escapeHtml(session.ip_address || 'Unknown IP')} · Created ${formatDate(session.created_at)}
						</div>
					</div>
					${!session.is_current ? `<button class="btn btn-revoke" data-session-id="${session.id}">Revoke</button>` : ''}
				</div>
			`).join('');

		} catch (err) {
			clearTimeout(timeoutId);
			if (err instanceof Error && err.name === 'AbortError') {
				els.sessionsLoading.textContent = 'Request timed out. Please try again.';
			} else {
				els.sessionsLoading.textContent = 'Failed to load sessions';
			}
		}
	}

	// Use event delegation for revoke button clicks
	els.sessionsList.addEventListener('click', async (e) => {
		const target = e.target as HTMLElement;
		if (!target.classList.contains('btn-revoke')) return;

		const btn = target as HTMLButtonElement;
		const sessionId = btn.dataset.sessionId;
		if (!sessionId) return;

		btn.disabled = true;
		btn.textContent = 'Revoking...';

		try {
			const response = await csrfFetch(`/api/auth/sessions/${sessionId}`, {
				method: 'DELETE',
			});

			if (!response.ok) {
				const data = await response.json();
				callbacks.showError(data.error || 'Failed to revoke session');
				btn.disabled = false;
				btn.textContent = 'Revoke';
				return;
			}

			callbacks.showSuccess('Session revoked successfully');
			loadSessions();
		} catch (err) {
			callbacks.showError('Network error. Please try again.');
			btn.disabled = false;
			btn.textContent = 'Revoke';
		}
	});

	return { loadSessions };
}
