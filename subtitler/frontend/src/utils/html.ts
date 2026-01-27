/**
 * HTML escaping utility to prevent XSS attacks
 */

/**
 * Escapes HTML special characters in a string to prevent XSS.
 * Uses the browser's built-in text content escaping.
 *
 * @param text - The string to escape
 * @returns The escaped string safe for use in innerHTML
 *
 * @example
 * escapeHtml('<script>alert("xss")</script>')
 * // Returns: '&lt;script&gt;alert("xss")&lt;/script&gt;'
 */
export function escapeHtml(text: string): string {
	const div = document.createElement('div');
	div.textContent = text;
	return div.innerHTML;
}
