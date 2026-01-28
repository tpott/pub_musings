/**
 * Form field error handling utilities
 *
 * Shared utilities for displaying and clearing form validation errors.
 * Used across login, register, settings, forgot-password, and reset-password pages.
 */

/**
 * Clear error state from a form field
 * @param input - The input element to clear error from
 * @param errorEl - The error message element to hide
 */
export function clearFieldError(
	input: HTMLInputElement,
	errorEl: HTMLElement
): void {
	input.classList.remove('input-error');
	errorEl.style.display = 'none';
	errorEl.textContent = '';
}

/**
 * Show error state on a form field
 * @param input - The input element to mark as having an error
 * @param errorEl - The error message element to show
 * @param message - The error message to display
 */
export function showFieldError(
	input: HTMLInputElement,
	errorEl: HTMLElement,
	message: string
): void {
	input.classList.add('input-error');
	errorEl.textContent = message;
	errorEl.style.display = 'block';
}
