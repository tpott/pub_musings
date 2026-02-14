import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
	createFeedbackFormState,
	resetForm,
	setSubmitSending,
	setSubmitReady,
	type FeedbackFormElements,
	type FeedbackFormState,
} from './feedback-form';

function createMockElements(): FeedbackFormElements {
	const form = {
		reset: vi.fn(),
		style: { display: '' },
	} as unknown as HTMLFormElement;

	const success = {
		style: { display: 'none' },
	} as unknown as HTMLElement;

	const submitBtn = {
		disabled: false,
		textContent: 'Send Feedback',
	} as unknown as HTMLButtonElement;

	const charCount = {
		textContent: '0 / 10,240',
	} as unknown as HTMLElement;

	// Mock stars as a NodeList-like array with forEach
	const starMocks = Array.from({ length: 5 }, () => ({
		classList: {
			_classes: new Set<string>(),
			add(cls: string) { this._classes.add(cls); },
			remove(cls: string) { this._classes.delete(cls); },
			contains(cls: string) { return this._classes.has(cls); },
		},
	}));
	const stars = {
		forEach: (fn: (s: typeof starMocks[0], i: number) => void) => starMocks.forEach(fn),
		length: 5,
		[Symbol.iterator]: function* () { yield* starMocks; },
	} as unknown as NodeListOf<Element>;

	return { feedbackForm: form, feedbackSuccess: success, submitBtn, charCount, stars };
}

describe('feedback-form', () => {
	let elements: FeedbackFormElements;
	let state: FeedbackFormState;

	beforeEach(() => {
		elements = createMockElements();
		state = createFeedbackFormState();
	});

	describe('createFeedbackFormState', () => {
		it('initializes with null rating and empty overflow', () => {
			expect(state.selectedRating).toBeNull();
			expect(state.previousOverflow).toBe('');
		});
	});

	describe('setSubmitSending', () => {
		it('disables button and shows Sending... text', () => {
			setSubmitSending(elements.submitBtn);

			expect(elements.submitBtn.disabled).toBe(true);
			expect(elements.submitBtn.textContent).toBe('Sending...');
		});
	});

	describe('setSubmitReady', () => {
		it('enables button and restores Send Feedback text', () => {
			setSubmitSending(elements.submitBtn);

			setSubmitReady(elements.submitBtn);

			expect(elements.submitBtn.disabled).toBe(false);
			expect(elements.submitBtn.textContent).toBe('Send Feedback');
		});
	});

	describe('resetForm', () => {
		it('restores submit button text after successful submission', () => {
			// Simulate successful submission flow
			setSubmitSending(elements.submitBtn);
			elements.feedbackForm.style.display = 'none';
			(elements.feedbackSuccess as { style: { display: string } }).style.display = 'block';

			resetForm(elements, state);

			expect(elements.submitBtn.textContent).toBe('Send Feedback');
			expect(elements.submitBtn.disabled).toBe(false);
		});

		it('restores form visibility and hides success message', () => {
			elements.feedbackForm.style.display = 'none';
			(elements.feedbackSuccess as { style: { display: string } }).style.display = 'block';

			resetForm(elements, state);

			expect(elements.feedbackForm.style.display).toBe('block');
			expect((elements.feedbackSuccess as { style: { display: string } }).style.display).toBe('none');
		});

		it('calls form.reset()', () => {
			resetForm(elements, state);

			expect(elements.feedbackForm.reset).toHaveBeenCalled();
		});

		it('clears selected rating', () => {
			state.selectedRating = 4;

			resetForm(elements, state);

			expect(state.selectedRating).toBeNull();
		});

		it('removes active class from stars', () => {
			// Add active class to all stars
			elements.stars.forEach(s => (s as unknown as { classList: { add: (c: string) => void } }).classList.add('active'));

			resetForm(elements, state);

			elements.stars.forEach(s => {
				expect((s as unknown as { classList: { contains: (c: string) => boolean } }).classList.contains('active')).toBe(false);
			});
		});

		it('resets character count', () => {
			elements.charCount.textContent = '500 / 10,240';

			resetForm(elements, state);

			expect(elements.charCount.textContent).toBe('0 / 10,240');
		});

		it('handles multiple reset cycles (regression: button text must reset each time)', () => {
			// First submission cycle
			setSubmitSending(elements.submitBtn);
			resetForm(elements, state);
			expect(elements.submitBtn.textContent).toBe('Send Feedback');

			// Second submission cycle
			setSubmitSending(elements.submitBtn);
			expect(elements.submitBtn.textContent).toBe('Sending...');
			resetForm(elements, state);
			expect(elements.submitBtn.textContent).toBe('Send Feedback');
		});
	});
});
