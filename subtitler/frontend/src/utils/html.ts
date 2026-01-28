/**
 * HTML escaping utility to prevent XSS attacks
 */

/**
 * Escapes HTML special characters in a string to prevent XSS.
 * Uses the browser's built-in text content escaping when available,
 * falls back to regex replacement for non-browser environments (tests, SSR).
 *
 * @param text - The string to escape
 * @returns The escaped string safe for use in innerHTML
 *
 * @example
 * escapeHtml('<script>alert("xss")</script>')
 * // Returns: '&lt;script&gt;alert("xss")&lt;/script&gt;'
 */
export function escapeHtml(text: string): string {
	// Use DOM-based escaping in browser environment (more robust)
	if (typeof document !== 'undefined') {
		const div = document.createElement('div');
		div.textContent = text;
		return div.innerHTML;
	}
	// Fallback for non-browser environments (Node.js tests, SSR)
	return text
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#x27;');
}
