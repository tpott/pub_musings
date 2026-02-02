import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	showRetranscribeProgress,
	updateRetranscribeProgress,
	hideRetranscribeProgress,
	collapseRetranscribeProgress,
	type RetranscribeProgressElements,
} from './retranscribe-progress';

function createMockEls(): RetranscribeProgressElements {
	return {
		container: {
			style: { display: '' },
			classList: {
				_classes: new Set<string>(),
				add(c: string) { this._classes.add(c); },
				remove(c: string) { this._classes.delete(c); },
				contains(c: string) { return this._classes.has(c); },
			},
		} as unknown as HTMLElement,
		fill: { style: { width: '' } } as unknown as HTMLElement,
		pct: { textContent: '' } as unknown as HTMLElement,
		msg: { textContent: '' } as unknown as HTMLElement,
	};
}

describe('retranscribe-progress', () => {
	beforeEach(() => { vi.useFakeTimers(); });
	afterEach(() => { vi.useRealTimers(); });

	describe('showRetranscribeProgress', () => {
		it('shows container and resets state', () => {
			const els = createMockEls();
			els.container.classList.add('collapsing');
			els.fill.style.width = '50%';
			els.pct.textContent = '50%';
			els.msg.textContent = 'Old status';

			showRetranscribeProgress(els);

			expect(els.container.style.display).toBe('block');
			expect(els.container.classList.contains('collapsing')).toBe(false);
			expect(els.fill.style.width).toBe('0%');
			expect(els.pct.textContent).toBe('');
			expect(els.msg.textContent).toBe('Starting...');
		});
	});

	describe('updateRetranscribeProgress', () => {
		it('extracts percentage from status text', () => {
			const els = createMockEls();
			updateRetranscribeProgress(els, 'Processing... (45%) - 2m remaining');

			expect(els.fill.style.width).toBe('45%');
			expect(els.pct.textContent).toBe('45%');
			expect(els.msg.textContent).toBe('Processing... (45%) - 2m remaining');
		});

		it('updates message without percentage when none in text', () => {
			const els = createMockEls();
			updateRetranscribeProgress(els, 'Waiting for server...');

			expect(els.fill.style.width).toBe('');
			expect(els.pct.textContent).toBe('');
			expect(els.msg.textContent).toBe('Waiting for server...');
		});

		it('handles 0% progress', () => {
			const els = createMockEls();
			updateRetranscribeProgress(els, 'Starting (0%)');

			expect(els.fill.style.width).toBe('0%');
			expect(els.pct.textContent).toBe('0%');
		});

		it('handles 100% progress', () => {
			const els = createMockEls();
			updateRetranscribeProgress(els, 'Almost done (100%)');

			expect(els.fill.style.width).toBe('100%');
			expect(els.pct.textContent).toBe('100%');
		});
	});

	describe('hideRetranscribeProgress', () => {
		it('hides container immediately', () => {
			const els = createMockEls();
			els.container.style.display = 'block';

			hideRetranscribeProgress(els);

			expect(els.container.style.display).toBe('none');
		});
	});

	describe('collapseRetranscribeProgress', () => {
		it('sets completion state', () => {
			const els = createMockEls();
			collapseRetranscribeProgress(els);

			expect(els.msg.textContent).toBe('Complete!');
			expect(els.fill.style.width).toBe('100%');
			expect(els.pct.textContent).toBe('100%');
		});

		it('adds collapsing class after 1500ms', () => {
			const els = createMockEls();
			collapseRetranscribeProgress(els);

			expect(els.container.classList.contains('collapsing')).toBe(false);

			vi.advanceTimersByTime(1500);
			expect(els.container.classList.contains('collapsing')).toBe(true);
		});

		it('hides and removes collapsing class after 2000ms total', () => {
			const els = createMockEls();
			els.container.style.display = 'block';
			collapseRetranscribeProgress(els);

			vi.advanceTimersByTime(2000);
			expect(els.container.style.display).toBe('none');
			expect(els.container.classList.contains('collapsing')).toBe(false);
		});
	});
});
