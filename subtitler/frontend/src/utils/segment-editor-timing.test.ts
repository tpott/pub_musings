import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { TranscriptionSegment } from '../types/transcription';
import {
	createSegmentEditorState,
	adjustSegmentTime,
	type SegmentEditorState,
	type SegmentEditorElements,
} from './segment-editor';

// --- Mock csrfFetch ---
vi.mock('./csrf', () => ({
	csrfFetch: vi.fn(),
}));

// --- Mock bionic ---
vi.mock('./bionic', () => ({
	renderBionicText: vi.fn((text: string) => `<b>${text}</b>`),
	isBionicEnabled: vi.fn(() => false),
}));

// --- Mock html ---
vi.mock('./html', () => ({
	escapeHtml: vi.fn((text: string) => text),
}));

// --- Mock dom ---
vi.mock('./dom', () => ({
	getOptionalElement: vi.fn(() => null),
}));

// --- Mock transcription-polling ---
vi.mock('./transcription-polling', () => ({
	formatTime: vi.fn((s: number) => {
		const mins = Math.floor(s / 60);
		const secs = Math.floor(s % 60);
		return `${mins}:${secs.toString().padStart(2, '0')}`;
	}),
}));

// --- Mock validation ---
vi.mock('./validation', () => ({
	parseTimeString: vi.fn((str: string) => {
		const parts = str.split(':');
		if (parts.length !== 3) return { error: 'Invalid time format' };
		const seconds = parseInt(parts[0]) * 3600 + parseInt(parts[1]) * 60 + parseFloat(parts[2]);
		if (isNaN(seconds)) return { error: 'Invalid time format' };
		return { seconds };
	}),
}));

function makeSegments(count: number): TranscriptionSegment[] {
	return Array.from({ length: count }, (_, i) => ({
		id: i,
		start: i * 5,
		end: (i + 1) * 5,
		text: `Segment ${i}`,
	}));
}

function makeMockElements(): SegmentEditorElements {
	const segmentsEl = {
		innerHTML: '',
		querySelectorAll: vi.fn(() => []),
		style: {},
	};
	return {
		segments: segmentsEl as unknown as HTMLElement,
		fullText: { style: { display: '' }, textContent: '' } as unknown as HTMLElement,
		editBtn: { classList: { add: vi.fn(), remove: vi.fn() }, textContent: '' } as unknown as HTMLButtonElement,
		undoBtn: { disabled: false } as unknown as HTMLButtonElement,
		redoBtn: { disabled: false } as unknown as HTMLButtonElement,
		saveBtn: { disabled: false, textContent: '' } as unknown as HTMLButtonElement,
		editActions: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		addSegmentBtn: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLButtonElement,
		unsavedIndicator: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		subtitleFeedback: {
			style: { display: '' },
			querySelectorAll: vi.fn(() => []),
			addEventListener: vi.fn(),
		} as unknown as HTMLElement,
		previewVideo: { currentTime: 0, play: vi.fn(() => Promise.resolve()) } as unknown as HTMLVideoElement,
	};
}

beforeEach(() => {
	vi.clearAllMocks();
});

afterEach(() => {
	vi.restoreAllMocks();
});

describe('adjustSegmentTime', () => {
	it('moves start earlier by 0.1s', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'earlier', false);

		expect(state.editedSegments[0].start).toBeCloseTo(1.9, 5);
	});

	it('moves start later by 0.1s', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'later', false);

		expect(state.editedSegments[0].start).toBeCloseTo(2.1, 5);
	});

	it('moves end earlier by 0.1s', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'end', 'earlier', false);

		expect(state.editedSegments[0].end).toBeCloseTo(4.9, 5);
	});

	it('moves end later by 0.1s', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'end', 'later', false);

		expect(state.editedSegments[0].end).toBeCloseTo(5.1, 5);
	});

	it('uses 0.5s step when shift is held', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'earlier', true);

		expect(state.editedSegments[0].start).toBeCloseTo(1.5, 5);
	});

	it('uses 0.5s step for end when shift is held', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'end', 'later', true);

		expect(state.editedSegments[0].end).toBeCloseTo(5.5, 5);
	});

	it('prevents start from going below 0', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 0.05, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'earlier', false);

		expect(state.editedSegments[0].start).toBe(0);
	});

	it('prevents start from exceeding end - 0.1', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 4.95, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'later', false);

		expect(state.editedSegments[0].start).toBeCloseTo(4.9, 5);
	});

	it('prevents end from going below start + 0.1', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 2.05, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'end', 'earlier', false);

		expect(state.editedSegments[0].end).toBeCloseTo(2.1, 5);
	});

	it('pushes to undo history before changing', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'earlier', false);

		expect(state.undoStack).toHaveLength(1);
		// Undo history should have the original start value
		expect(state.undoStack[0][0].start).toBe(2.0);
	});

	it('marks state as unsaved', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'earlier', false);

		expect(state.hasUnsavedChanges).toBe(true);
		expect(els.unsavedIndicator.classList.add).toHaveBeenCalledWith('visible');
	});

	it('clears redo stack when adjusting', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 5.0, text: 'test' }];
		state.redoStack = [makeSegments(1)];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'earlier', false);

		expect(state.redoStack).toHaveLength(0);
	});

	it('handles multiple adjustments accumulating correctly', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 2.0, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'earlier', false);
		adjustSegmentTime(state, els, 0, 'start', 'earlier', false);
		adjustSegmentTime(state, els, 0, 'start', 'earlier', false);

		expect(state.editedSegments[0].start).toBeCloseTo(1.7, 5);
		expect(state.undoStack).toHaveLength(3);
	});

	it('works with segments that have fractional times', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [{ id: 0, start: 1.234, end: 3.567, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'later', false);

		expect(state.editedSegments[0].start).toBeCloseTo(1.334, 5);
	});

	it('rounds to avoid floating-point drift', () => {
		const state = createSegmentEditorState();
		// 0.1 + 0.2 = 0.30000000000000004 in floating point
		state.editedSegments = [{ id: 0, start: 0.2, end: 5.0, text: 'test' }];
		state.isEditMode = true;
		const els = makeMockElements();

		adjustSegmentTime(state, els, 0, 'start', 'earlier', false);

		// Should be 0.1, not 0.09999... or similar
		expect(state.editedSegments[0].start).toBeCloseTo(0.1, 5);
	});
});
