/**
 * Form blur/input validation utilities
 *
 * Reusable validation patterns for form fields that validate on blur
 * and clear errors on input. Used across login, register, settings pages.
 */

import { clearFieldError, showFieldError } from './form-errors';

/**
 * Configuration for setting up blur validation on a field
 */
export interface BlurValidationConfig {
	/** The input element to validate */
	input: HTMLInputElement;
	/** The error element to show/hide messages */
	errorEl: HTMLElement;
	/** Validation function - returns error message or null if valid */
	validate: (value: string) => string | null;
	/** Optional: only validate if a condition is met */
	condition?: () => boolean;
}

/**
 * Set up blur validation for a single field
 *
 * Attaches blur listener that runs validation, and input listener that clears errors.
 *
 * @param config - The validation configuration
 * @returns Cleanup function to remove event listeners
 *
 * @example
 * ```ts
 * const cleanup = setupBlurValidation({
 *   input: emailInput,
 *   errorEl: emailError,
 *   validate: validateEmail
 * });
 * // Later, if needed:
 * cleanup();
 * ```
 */
export function setupBlurValidation(config: BlurValidationConfig): () => void {
	const { input, errorEl, validate, condition } = config;

	const handleBlur = () => {
		// Skip validation if condition is provided and returns false
		if (condition && !condition()) {
			return;
		}

		const error = validate(input.value);
		if (error) {
			showFieldError(input, errorEl, error);
		} else {
			clearFieldError(input, errorEl);
		}
	};

	const handleInput = () => {
		clearFieldError(input, errorEl);
	};

	input.addEventListener('blur', handleBlur);
	input.addEventListener('input', handleInput);

	// Return cleanup function
	return () => {
		input.removeEventListener('blur', handleBlur);
		input.removeEventListener('input', handleInput);
	};
}

/**
 * Set up blur validation for multiple fields at once
 *
 * @param configs - Array of validation configurations
 * @returns Cleanup function to remove all event listeners
 *
 * @example
 * ```ts
 * const cleanup = setupBlurValidations([
 *   { input: emailInput, errorEl: emailError, validate: validateEmail },
 *   { input: passwordInput, errorEl: passwordError, validate: validatePassword }
 * ]);
 * ```
 */
export function setupBlurValidations(
	configs: BlurValidationConfig[]
): () => void {
	const cleanups = configs.map(setupBlurValidation);

	return () => {
		cleanups.forEach((cleanup) => cleanup());
	};
}

/**
 * Create a password match validator
 *
 * @param passwordInput - The original password input to compare against
 * @returns Validation function for confirm password field
 *
 * @example
 * ```ts
 * setupBlurValidation({
 *   input: confirmPasswordInput,
 *   errorEl: confirmPasswordError,
 *   validate: createPasswordMatchValidator(passwordInput)
 * });
 * ```
 */
export function createPasswordMatchValidator(
	passwordInput: HTMLInputElement
): (value: string) => string | null {
	return (value: string) => {
		if (value && value !== passwordInput.value) {
			return 'Passwords do not match';
		}
		return null;
	};
}

/**
 * Create a conditional validator that only validates when value is non-empty
 *
 * @param validate - The underlying validation function
 * @returns Validation function that returns null for empty values
 *
 * @example
 * ```ts
 * // Only validate TOTP code if user has entered something
 * setupBlurValidation({
 *   input: totpInput,
 *   errorEl: totpError,
 *   validate: createOptionalValidator(validateTotpCode)
 * });
 * ```
 */
export function createOptionalValidator(
	validate: (value: string) => string | null
): (value: string) => string | null {
	return (value: string) => {
		if (!value) {
			return null;
		}
		return validate(value);
	};
}
