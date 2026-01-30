import { describe, it, expect, beforeEach, vi } from 'vitest';
import { clearFieldError, showFieldError } from './form-errors';

// Create mock element classes
class MockClassList {
	private classes: Set<string> = new Set();

	add(className: string) {
		this.classes.add(className);
	}

	remove(className: string) {
		this.classes.delete(className);
	}

	contains(className: string): boolean {
		return this.classes.has(className);
	}
}

// Create mock HTMLElement-like objects
function createMockInput(): HTMLInputElement {
	const attributes = new Map<string, string>();
	return {
		classList: new MockClassList(),
		setAttribute(name: string, value: string) {
			attributes.set(name, value);
		},
		getAttribute(name: string): string | null {
			return attributes.get(name) ?? null;
		},
		removeAttribute(name: string) {
			attributes.delete(name);
		},
	} as unknown as HTMLInputElement;
}

function createMockElement(): HTMLElement {
	return {
		style: { display: '' },
		textContent: '',
	} as unknown as HTMLElement;
}

describe('form-errors utility', () => {
	let mockInput: HTMLInputElement;
	let mockErrorEl: HTMLElement;

	beforeEach(() => {
		// Create mock elements
		mockInput = createMockInput();
		mockErrorEl = createMockElement();
	});

	describe('clearFieldError', () => {
		it('should remove input-error class from input', () => {
			mockInput.classList.add('input-error');
			clearFieldError(mockInput, mockErrorEl);
			expect(mockInput.classList.contains('input-error')).toBe(false);
		});

		it('should remove aria-invalid attribute from input', () => {
			mockInput.setAttribute('aria-invalid', 'true');
			clearFieldError(mockInput, mockErrorEl);
			expect(mockInput.getAttribute('aria-invalid')).toBe(null);
		});

		it('should hide error element', () => {
			mockErrorEl.style.display = 'block';
			clearFieldError(mockInput, mockErrorEl);
			expect(mockErrorEl.style.display).toBe('none');
		});

		it('should clear error element text content', () => {
			mockErrorEl.textContent = 'Error message';
			clearFieldError(mockInput, mockErrorEl);
			expect(mockErrorEl.textContent).toBe('');
		});

		it('should work when input has no error class', () => {
			clearFieldError(mockInput, mockErrorEl);
			expect(mockInput.classList.contains('input-error')).toBe(false);
		});
	});

	describe('showFieldError', () => {
		it('should add input-error class to input', () => {
			showFieldError(mockInput, mockErrorEl, 'Error');
			expect(mockInput.classList.contains('input-error')).toBe(true);
		});

		it('should set aria-invalid attribute on input', () => {
			showFieldError(mockInput, mockErrorEl, 'Error');
			expect(mockInput.getAttribute('aria-invalid')).toBe('true');
		});

		it('should set error element text content', () => {
			showFieldError(mockInput, mockErrorEl, 'Invalid email');
			expect(mockErrorEl.textContent).toBe('Invalid email');
		});

		it('should show error element', () => {
			mockErrorEl.style.display = 'none';
			showFieldError(mockInput, mockErrorEl, 'Error');
			expect(mockErrorEl.style.display).toBe('block');
		});

		it('should handle empty message', () => {
			showFieldError(mockInput, mockErrorEl, '');
			expect(mockErrorEl.textContent).toBe('');
			expect(mockInput.classList.contains('input-error')).toBe(true);
		});

		it('should handle long messages', () => {
			const longMessage = 'A'.repeat(500);
			showFieldError(mockInput, mockErrorEl, longMessage);
			expect(mockErrorEl.textContent).toBe(longMessage);
		});
	});

	describe('clearing after showing', () => {
		it('should properly reset state after show then clear', () => {
			showFieldError(mockInput, mockErrorEl, 'Error occurred');
			expect(mockInput.classList.contains('input-error')).toBe(true);
			expect(mockInput.getAttribute('aria-invalid')).toBe('true');
			expect(mockErrorEl.textContent).toBe('Error occurred');
			expect(mockErrorEl.style.display).toBe('block');

			clearFieldError(mockInput, mockErrorEl);
			expect(mockInput.classList.contains('input-error')).toBe(false);
			expect(mockInput.getAttribute('aria-invalid')).toBe(null);
			expect(mockErrorEl.textContent).toBe('');
			expect(mockErrorEl.style.display).toBe('none');
		});
	});
});
