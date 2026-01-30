/**
 * Navigation authentication utilities
 *
 * Provides shared auth checking and nav bar updating logic
 * that can be used across multiple pages.
 */

import { csrfFetch, fetchCsrfToken, clearCsrfToken } from './csrf';
import { AuthMeResponseSchema, safeParse, type User } from './api-schemas';

export type { User };

export interface AuthResult {
	isAuthenticated: boolean;
	user: User | null;
}

export interface AuthCallbacks {
	onAuthenticated?: (user: User) => void;
	onUnauthenticated?: () => void;
}

/**
 * Check authentication status and update navigation accordingly.
 *
 * @param callbacks - Optional callbacks for page-specific logic
 * @returns AuthResult with isAuthenticated flag and user data
 */
export async function checkAuthAndUpdateNav(
	callbacks?: AuthCallbacks
): Promise<AuthResult> {
	const nav = document.getElementById('nav') as HTMLElement | null;
	const loginLink = document.getElementById('loginLink') as HTMLElement | null;
	const registerLink = document.getElementById(
		'registerLink'
	) as HTMLElement | null;

	if (!nav || !loginLink || !registerLink) {
		console.warn('Navigation elements not found');
		return { isAuthenticated: false, user: null };
	}

	try {
		const response = await fetch('/api/auth/me');
		if (response.ok) {
			const rawData = await response.json();

			// Validate response structure
			const data = safeParse(AuthMeResponseSchema, rawData);
			if (data?.user) {
				// User is authenticated
				loginLink.style.display = 'none';
				registerLink.style.display = 'none';

				// Fetch CSRF token for authenticated requests
				await fetchCsrfToken();

				// Add user email display (only if not already present)
				let userSpan = nav.querySelector('.nav-user') as HTMLSpanElement | null;
				if (!userSpan) {
					userSpan = document.createElement('span');
					userSpan.className = 'nav-user';
					nav.appendChild(userSpan);
				}
				userSpan.textContent = data.user.email;

				// Add Settings link if requested via data attribute (only if not already present)
				const showSettings = nav.dataset.showSettings === 'true';
				if (showSettings && !nav.querySelector('.nav-link[href="/settings"]')) {
					const settingsLink = document.createElement('a');
					settingsLink.href = '/settings';
					settingsLink.className = 'nav-link';
					settingsLink.textContent = 'Settings';
					nav.appendChild(settingsLink);
				}

				// Add logout button (only if not already present)
				if (!nav.querySelector('.nav-logout')) {
					const logoutBtn = document.createElement('button');
					logoutBtn.className = 'nav-logout';
					logoutBtn.textContent = 'Log out';
					logoutBtn.addEventListener('click', () => logout());
					nav.appendChild(logoutBtn);
				}

				// Call page-specific callback
				if (callbacks?.onAuthenticated) {
					callbacks.onAuthenticated(data.user);
				}

				return { isAuthenticated: true, user: data.user };
			}
		}
	} catch (err) {
		// Auth check failed - likely not logged in, but log if it's a network error
		if (err instanceof TypeError) {
			console.error('Auth check network error:', err.message);
		}
	}

	// Call page-specific callback for unauthenticated state
	if (callbacks?.onUnauthenticated) {
		callbacks.onUnauthenticated();
	}

	return { isAuthenticated: false, user: null };
}

/**
 * Log out the current user and reload the page.
 */
export async function logout(): Promise<void> {
	try {
		await csrfFetch('/api/auth/logout', { method: 'POST' });
		clearCsrfToken();
		window.location.reload();
	} catch (err) {
		console.error('Logout failed:', err);
	}
}
