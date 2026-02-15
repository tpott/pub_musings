/**
 * Feedback form state management utilities.
 * Extracted from FeedbackButton.astro for testability.
 */

export interface FeedbackFormElements {
	feedbackForm: HTMLFormElement;
	feedbackSuccess: HTMLElement;
	submitBtn: HTMLButtonElement;
	charCount: HTMLElement;
	stars: NodeListOf<Element>;
}

export interface FeedbackFormState {
	selectedRating: number | null;
	previousOverflow: string;
}

export function createFeedbackFormState(): FeedbackFormState {
	return {
		selectedRating: null,
		previousOverflow: '',
	};
}

/**
 * Reset the feedback form to its initial state.
 * Called after modal close to prepare for next use.
 */
export function resetForm(elements: FeedbackFormElements, state: FeedbackFormState): void {
	elements.feedbackForm.reset();
	elements.feedbackForm.style.display = 'block';
	elements.feedbackSuccess.style.display = 'none';
	state.selectedRating = null;
	elements.stars.forEach(s => s.classList.remove('active'));
	elements.charCount.textContent = '0 / 10,240';
	elements.submitBtn.disabled = false;
	elements.submitBtn.textContent = 'Send Feedback';
}

/**
 * Set the submit button to its "sending" state.
 */
export function setSubmitSending(submitBtn: HTMLButtonElement): void {
	submitBtn.disabled = true;
	submitBtn.textContent = 'Sending...';
}

/**
 * Restore the submit button after a failed submission.
 */
export function setSubmitReady(submitBtn: HTMLButtonElement): void {
	submitBtn.disabled = false;
	submitBtn.textContent = 'Send Feedback';
}
