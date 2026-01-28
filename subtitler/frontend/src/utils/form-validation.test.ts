import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	setupBlurValidation,
	setupBlurValidations,
	createPasswordMatchValidator,
	createOptionalValidator,
	type BlurValidationConfig,
} from './form-validation';

describe('form-validation utility', () => {
	// Mock DOM elements
	let mockInput: HTMLInputElement;
	let mockErrorEl: HTMLElement;

	beforeEach(() => {
		// Create mock input element
		mockInput = {
			value: '',
			classList: {
				add: vi.fn(),
				remove: vi.fn(),
			},
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
		} as unknown as HTMLInputElement;

		// Create mock error element
		mockErrorEl = {
			textContent: '',
			style: { display: 'none' },
		} as unknown as HTMLElement;
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('setupBlurValidation', () => {
		it('should attach blur and input event listeners', () => {
			const validate = vi.fn().mockReturnValue(null);

			setupBlurValidation({ input: mockInput, errorEl: mockErrorEl, validate });

			expect(mockInput.addEventListener).toHaveBeenCalledTimes(2);
			expect(mockInput.addEventListener).toHaveBeenCalledWith(
				'blur',
				expect.any(Function)
			);
			expect(mockInput.addEventListener).toHaveBeenCalledWith(
				'input',
				expect.any(Function)
			);
		});

		it('should return cleanup function that removes listeners', () => {
			const validate = vi.fn().mockReturnValue(null);

			const cleanup = setupBlurValidation({
				input: mockInput,
				errorEl: mockErrorEl,
				validate,
			});

			cleanup();

			expect(mockInput.removeEventListener).toHaveBeenCalledTimes(2);
			expect(mockInput.removeEventListener).toHaveBeenCalledWith(
				'blur',
				expect.any(Function)
			);
			expect(mockInput.removeEventListener).toHaveBeenCalledWith(
				'input',
				expect.any(Function)
			);
		});

		it('should show error on blur when validation fails', () => {
			const validate = vi.fn().mockReturnValue('Invalid email');
			let blurHandler: () => void = () => {};

			mockInput.addEventListener = vi.fn((event, handler) => {
				if (event === 'blur') blurHandler = handler as () => void;
			});

			setupBlurValidation({ input: mockInput, errorEl: mockErrorEl, validate });

			mockInput.value = 'bad-email';
			blurHandler();

			expect(validate).toHaveBeenCalledWith('bad-email');
			expect(mockInput.classList.add).toHaveBeenCalledWith('input-error');
			expect(mockErrorEl.textContent).toBe('Invalid email');
			expect(mockErrorEl.style.display).toBe('block');
		});

		it('should clear error on blur when validation passes', () => {
			const validate = vi.fn().mockReturnValue(null);
			let blurHandler: () => void = () => {};

			mockInput.addEventListener = vi.fn((event, handler) => {
				if (event === 'blur') blurHandler = handler as () => void;
			});

			setupBlurValidation({ input: mockInput, errorEl: mockErrorEl, validate });

			mockInput.value = 'good@email.com';
			blurHandler();

			expect(validate).toHaveBeenCalledWith('good@email.com');
			expect(mockInput.classList.remove).toHaveBeenCalledWith('input-error');
			expect(mockErrorEl.style.display).toBe('none');
		});

		it('should clear error on input', () => {
			const validate = vi.fn().mockReturnValue(null);
			let inputHandler: () => void = () => {};

			mockInput.addEventListener = vi.fn((event, handler) => {
				if (event === 'input') inputHandler = handler as () => void;
			});

			setupBlurValidation({ input: mockInput, errorEl: mockErrorEl, validate });

			inputHandler();

			expect(mockInput.classList.remove).toHaveBeenCalledWith('input-error');
			expect(mockErrorEl.style.display).toBe('none');
		});

		it('should skip validation when condition returns false', () => {
			const validate = vi.fn().mockReturnValue('Error');
			const condition = vi.fn().mockReturnValue(false);
			let blurHandler: () => void = () => {};

			mockInput.addEventListener = vi.fn((event, handler) => {
				if (event === 'blur') blurHandler = handler as () => void;
			});

			setupBlurValidation({
				input: mockInput,
				errorEl: mockErrorEl,
				validate,
				condition,
			});

			blurHandler();

			expect(condition).toHaveBeenCalled();
			expect(validate).not.toHaveBeenCalled();
		});

		it('should run validation when condition returns true', () => {
			const validate = vi.fn().mockReturnValue(null);
			const condition = vi.fn().mockReturnValue(true);
			let blurHandler: () => void = () => {};

			mockInput.addEventListener = vi.fn((event, handler) => {
				if (event === 'blur') blurHandler = handler as () => void;
			});

			setupBlurValidation({
				input: mockInput,
				errorEl: mockErrorEl,
				validate,
				condition,
			});

			blurHandler();

			expect(condition).toHaveBeenCalled();
			expect(validate).toHaveBeenCalled();
		});
	});

	describe('setupBlurValidations', () => {
		it('should set up validation for multiple fields', () => {
			const mockInput2 = {
				...mockInput,
				addEventListener: vi.fn(),
				removeEventListener: vi.fn(),
				classList: { add: vi.fn(), remove: vi.fn() },
			} as unknown as HTMLInputElement;

			const mockErrorEl2 = {
				textContent: '',
				style: { display: 'none' },
			} as unknown as HTMLElement;

			const configs: BlurValidationConfig[] = [
				{
					input: mockInput,
					errorEl: mockErrorEl,
					validate: () => null,
				},
				{
					input: mockInput2,
					errorEl: mockErrorEl2,
					validate: () => null,
				},
			];

			setupBlurValidations(configs);

			expect(mockInput.addEventListener).toHaveBeenCalledTimes(2);
			expect(mockInput2.addEventListener).toHaveBeenCalledTimes(2);
		});

		it('should return cleanup function that cleans up all fields', () => {
			const mockInput2 = {
				...mockInput,
				addEventListener: vi.fn(),
				removeEventListener: vi.fn(),
				classList: { add: vi.fn(), remove: vi.fn() },
			} as unknown as HTMLInputElement;

			const mockErrorEl2 = {
				textContent: '',
				style: { display: 'none' },
			} as unknown as HTMLElement;

			const configs: BlurValidationConfig[] = [
				{
					input: mockInput,
					errorEl: mockErrorEl,
					validate: () => null,
				},
				{
					input: mockInput2,
					errorEl: mockErrorEl2,
					validate: () => null,
				},
			];

			const cleanup = setupBlurValidations(configs);
			cleanup();

			expect(mockInput.removeEventListener).toHaveBeenCalledTimes(2);
			expect(mockInput2.removeEventListener).toHaveBeenCalledTimes(2);
		});
	});

	describe('createPasswordMatchValidator', () => {
		it('should return null when passwords match', () => {
			const passwordInput = { value: 'password123' } as HTMLInputElement;
			const validator = createPasswordMatchValidator(passwordInput);

			expect(validator('password123')).toBeNull();
		});

		it('should return error when passwords do not match', () => {
			const passwordInput = { value: 'password123' } as HTMLInputElement;
			const validator = createPasswordMatchValidator(passwordInput);

			expect(validator('different')).toBe('Passwords do not match');
		});

		it('should return null for empty confirm password', () => {
			const passwordInput = { value: 'password123' } as HTMLInputElement;
			const validator = createPasswordMatchValidator(passwordInput);

			expect(validator('')).toBeNull();
		});

		it('should use current password value each time', () => {
			const passwordInput = { value: 'initial' } as HTMLInputElement;
			const validator = createPasswordMatchValidator(passwordInput);

			expect(validator('initial')).toBeNull();

			passwordInput.value = 'changed';
			expect(validator('initial')).toBe('Passwords do not match');
			expect(validator('changed')).toBeNull();
		});
	});

	describe('createOptionalValidator', () => {
		it('should return null for empty value', () => {
			const innerValidator = vi.fn().mockReturnValue('Error');
			const validator = createOptionalValidator(innerValidator);

			expect(validator('')).toBeNull();
			expect(innerValidator).not.toHaveBeenCalled();
		});

		it('should call inner validator for non-empty value', () => {
			const innerValidator = vi.fn().mockReturnValue(null);
			const validator = createOptionalValidator(innerValidator);

			const result = validator('test');

			expect(innerValidator).toHaveBeenCalledWith('test');
			expect(result).toBeNull();
		});

		it('should return inner validator error for non-empty value', () => {
			const innerValidator = vi.fn().mockReturnValue('Invalid format');
			const validator = createOptionalValidator(innerValidator);

			expect(validator('bad-input')).toBe('Invalid format');
		});
	});

	describe('module exports', () => {
		it('should export setupBlurValidation', () => {
			expect(typeof setupBlurValidation).toBe('function');
		});

		it('should export setupBlurValidations', () => {
			expect(typeof setupBlurValidations).toBe('function');
		});

		it('should export createPasswordMatchValidator', () => {
			expect(typeof createPasswordMatchValidator).toBe('function');
		});

		it('should export createOptionalValidator', () => {
			expect(typeof createOptionalValidator).toBe('function');
		});
	});
});
