import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { TranscriptionSegment } from '../types/transcription';
import {
	createSegmentEditorState,
	getSegmentFeedback,
	setSegmentFeedback,
	updateFeedbackButtons,
	updateUndoRedoButtons,
	pushToHistory,
	performUndo,
	performRedo,
	enterEditMode,
	exitEditMode,
	addNewSegment,
	saveSegments,
	getCurrentSegmentIndex,
	isTextInputFocused,
	scrollIntoContainerView,
	renderSegments,
	setupFeedbackHandler,
	navigateToSegment,
	type SegmentEditorState,
	type SegmentEditorElements,
	type SegmentEditorCallbacks,
} from './segment-editor';

// --- localStorage mock ---
const localStorageMock = (() => {
	let store: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => store[key] || null),
		setItem: vi.fn((key: string, value: string) => {
			store[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete store[key];
		}),
		clear: vi.fn(() => {
			store = {};
		}),
		_setStore: (newStore: Record<string, string>) => {
			store = { ...newStore };
		},
	};
})();

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

function makeCallbacks(): SegmentEditorCallbacks {
	return {
		showStatus: vi.fn(),
	};
}

beforeEach(() => {
	localStorageMock.clear();
	vi.stubGlobal('localStorage', localStorageMock);
	vi.clearAllMocks();
});

afterEach(() => {
	vi.restoreAllMocks();
});

// --- Tests ---

describe('createSegmentEditorState', () => {
	it('should return a fresh state object', () => {
		const state = createSegmentEditorState();
		expect(state.transcriptionSegments).toEqual([]);
		expect(state.editedSegments).toEqual([]);
		expect(state.isEditMode).toBe(false);
		expect(state.hasUnsavedChanges).toBe(false);
		expect(state.undoStack).toEqual([]);
		expect(state.redoStack).toEqual([]);
		expect(state.feedbackActiveIndex).toBe(-1);
		expect(state.currentUploadId).toBeNull();
	});

	it('should return independent state objects', () => {
		const state1 = createSegmentEditorState();
		const state2 = createSegmentEditorState();
		state1.isEditMode = true;
		expect(state2.isEditMode).toBe(false);
	});
});

describe('getSegmentFeedback / setSegmentFeedback', () => {
	it('should return null when no feedback stored', () => {
		expect(getSegmentFeedback('upload-1', 0)).toBeNull();
	});

	it('should return null when uploadId is null', () => {
		expect(getSegmentFeedback(null, 0)).toBeNull();
	});

	it('should store and retrieve feedback', () => {
		setSegmentFeedback('upload-1', 0, 'good');
		expect(getSegmentFeedback('upload-1', 0)).toBe('good');
	});

	it('should store feedback for different segments independently', () => {
		setSegmentFeedback('upload-1', 0, 'good');
		setSegmentFeedback('upload-1', 1, 'misaligned');
		expect(getSegmentFeedback('upload-1', 0)).toBe('good');
		expect(getSegmentFeedback('upload-1', 1)).toBe('misaligned');
	});

	it('should remove feedback when set to null', () => {
		setSegmentFeedback('upload-1', 0, 'good');
		setSegmentFeedback('upload-1', 0, null);
		expect(getSegmentFeedback('upload-1', 0)).toBeNull();
	});

	it('should not store feedback when uploadId is null', () => {
		setSegmentFeedback(null, 0, 'good');
		expect(localStorage.setItem).not.toHaveBeenCalled();
	});

	it('should use correct localStorage key format', () => {
		setSegmentFeedback('abc123', 0, 'good');
		expect(localStorage.setItem).toHaveBeenCalledWith(
			'subtitler:feedback:abc123',
			expect.any(String)
		);
	});

	it('should handle corrupted localStorage data gracefully', () => {
		localStorageMock._setStore({ 'subtitler:feedback:upload-1': 'not-json{' });
		expect(getSegmentFeedback('upload-1', 0)).toBeNull();
	});
});

describe('updateFeedbackButtons', () => {
	it('should hide feedback when index is negative', () => {
		const state = createSegmentEditorState();
		const feedbackEl = {
			style: { display: '' },
			querySelectorAll: vi.fn(() => []),
		} as unknown as HTMLElement;

		updateFeedbackButtons(state, feedbackEl, -1);

		expect(feedbackEl.style.display).toBe('none');
		expect(state.feedbackActiveIndex).toBe(-1);
	});

	it('should show feedback and update active index for valid index', () => {
		const state = createSegmentEditorState();
		state.currentUploadId = 'upload-1';
		const mockBtn = {
			getAttribute: vi.fn(() => 'good'),
			classList: { toggle: vi.fn() },
		};
		const feedbackEl = {
			style: { display: 'none' },
			querySelectorAll: vi.fn(() => [mockBtn]),
		} as unknown as HTMLElement;

		updateFeedbackButtons(state, feedbackEl, 2);

		expect(feedbackEl.style.display).toBe('');
		expect(state.feedbackActiveIndex).toBe(2);
	});

	it('should toggle active class on matching feedback button', () => {
		const state = createSegmentEditorState();
		state.currentUploadId = 'upload-1';
		setSegmentFeedback('upload-1', 0, 'misaligned');

		const btnGood = {
			getAttribute: vi.fn(() => 'good'),
			classList: { toggle: vi.fn() },
		};
		const btnMisaligned = {
			getAttribute: vi.fn(() => 'misaligned'),
			classList: { toggle: vi.fn() },
		};
		const feedbackEl = {
			style: { display: '' },
			querySelectorAll: vi.fn(() => [btnGood, btnMisaligned]),
		} as unknown as HTMLElement;

		updateFeedbackButtons(state, feedbackEl, 0);

		expect(btnGood.classList.toggle).toHaveBeenCalledWith('active', false);
		expect(btnMisaligned.classList.toggle).toHaveBeenCalledWith('active', true);
	});
});

describe('updateUndoRedoButtons', () => {
	it('should disable both when stacks are empty', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();

		updateUndoRedoButtons(state, els);

		expect(els.undoBtn.disabled).toBe(true);
		expect(els.redoBtn.disabled).toBe(true);
	});

	it('should enable undo when undo stack has items', () => {
		const state = createSegmentEditorState();
		state.undoStack.push(makeSegments(1));
		const els = makeMockElements();

		updateUndoRedoButtons(state, els);

		expect(els.undoBtn.disabled).toBe(false);
		expect(els.redoBtn.disabled).toBe(true);
	});

	it('should enable redo when redo stack has items', () => {
		const state = createSegmentEditorState();
		state.redoStack.push(makeSegments(1));
		const els = makeMockElements();

		updateUndoRedoButtons(state, els);

		expect(els.undoBtn.disabled).toBe(true);
		expect(els.redoBtn.disabled).toBe(false);
	});
});

describe('pushToHistory', () => {
	it('should push current segments to undo stack', () => {
		const state = createSegmentEditorState();
		state.editedSegments = makeSegments(2);
		const els = makeMockElements();

		pushToHistory(state, els);

		expect(state.undoStack).toHaveLength(1);
		expect(state.undoStack[0]).toEqual(makeSegments(2));
	});

	it('should clear redo stack on push', () => {
		const state = createSegmentEditorState();
		state.editedSegments = makeSegments(1);
		state.redoStack = [makeSegments(2)];
		const els = makeMockElements();

		pushToHistory(state, els);

		expect(state.redoStack).toHaveLength(0);
	});

	it('should limit undo stack to MAX_HISTORY_SIZE', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();

		// Fill beyond MAX_HISTORY_SIZE (50)
		for (let i = 0; i < 55; i++) {
			state.editedSegments = [{ id: 0, start: i, end: i + 1, text: `s${i}` }];
			pushToHistory(state, els);
		}

		expect(state.undoStack.length).toBeLessThanOrEqual(50);
	});

	it('should make deep copy of segments (not reference)', () => {
		const state = createSegmentEditorState();
		state.editedSegments = makeSegments(1);
		const els = makeMockElements();

		pushToHistory(state, els);
		state.editedSegments[0].text = 'modified';

		expect(state.undoStack[0][0].text).toBe('Segment 0');
	});
});

describe('performUndo', () => {
	it('should do nothing when undo stack is empty', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		const originalSegments = makeSegments(1);
		state.editedSegments = originalSegments;

		performUndo(state, els);

		expect(state.editedSegments).toEqual(originalSegments);
	});

	it('should restore previous state from undo stack', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		const previousSegments = makeSegments(2);
		state.undoStack.push(JSON.parse(JSON.stringify(previousSegments)));
		state.editedSegments = makeSegments(3);
		state.transcriptionSegments = makeSegments(2);

		performUndo(state, els);

		expect(state.editedSegments).toHaveLength(2);
	});

	it('should move current state to redo stack', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.undoStack.push(makeSegments(1));
		state.editedSegments = makeSegments(2);

		performUndo(state, els);

		expect(state.redoStack).toHaveLength(1);
		expect(state.redoStack[0]).toHaveLength(2);
	});

	it('should reassign segment IDs after undo', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		const segs = makeSegments(3);
		segs[0].id = 99;
		segs[1].id = 88;
		segs[2].id = 77;
		state.undoStack.push(segs);
		state.editedSegments = makeSegments(1);
		state.transcriptionSegments = [];

		performUndo(state, els);

		expect(state.editedSegments[0].id).toBe(0);
		expect(state.editedSegments[1].id).toBe(1);
		expect(state.editedSegments[2].id).toBe(2);
	});

	it('should clear unsaved indicator when matching original', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		const segs = makeSegments(2);
		state.transcriptionSegments = JSON.parse(JSON.stringify(segs));
		state.undoStack.push(JSON.parse(JSON.stringify(segs)));
		state.editedSegments = makeSegments(3);

		performUndo(state, els);

		expect(els.unsavedIndicator.classList.remove).toHaveBeenCalledWith('visible');
	});
});

describe('performRedo', () => {
	it('should do nothing when redo stack is empty', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.editedSegments = makeSegments(1);

		performRedo(state, els);

		expect(state.editedSegments).toHaveLength(1);
	});

	it('should restore state from redo stack', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.redoStack.push(makeSegments(3));
		state.editedSegments = makeSegments(1);

		performRedo(state, els);

		expect(state.editedSegments).toHaveLength(3);
	});

	it('should push current state to undo stack', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.redoStack.push(makeSegments(2));
		state.editedSegments = makeSegments(1);

		performRedo(state, els);

		expect(state.undoStack).toHaveLength(1);
		expect(state.undoStack[0]).toHaveLength(1);
	});

	it('should mark as unsaved after redo', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.redoStack.push(makeSegments(2));
		state.editedSegments = makeSegments(1);

		performRedo(state, els);

		expect(els.unsavedIndicator.classList.add).toHaveBeenCalledWith('visible');
	});
});

describe('enterEditMode', () => {
	it('should set isEditMode to true', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.transcriptionSegments = makeSegments(2);

		enterEditMode(state, els);

		expect(state.isEditMode).toBe(true);
	});

	it('should copy transcription segments to editedSegments', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.transcriptionSegments = makeSegments(2);

		enterEditMode(state, els);

		expect(state.editedSegments).toEqual(state.transcriptionSegments);
		// Should be a deep copy, not reference
		state.editedSegments[0].text = 'changed';
		expect(state.transcriptionSegments[0].text).toBe('Segment 0');
	});

	it('should update UI elements', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.transcriptionSegments = makeSegments(1);

		enterEditMode(state, els);

		expect(els.editBtn.classList.add).toHaveBeenCalledWith('active');
		expect(els.editBtn.textContent).toBe('Editing...');
		expect(els.editActions.classList.add).toHaveBeenCalledWith('visible');
		expect(els.addSegmentBtn.classList.add).toHaveBeenCalledWith('visible');
		expect(els.fullText.style.display).toBe('none');
		expect(els.subtitleFeedback.style.display).toBe('none');
	});

	it('should clear undo/redo history', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.transcriptionSegments = makeSegments(1);
		state.undoStack = [makeSegments(1)];
		state.redoStack = [makeSegments(1)];

		enterEditMode(state, els);

		expect(state.undoStack).toHaveLength(0);
		expect(state.redoStack).toHaveLength(0);
	});
});

describe('exitEditMode', () => {
	it('should set isEditMode to false', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.isEditMode = true;

		exitEditMode(state, els);

		expect(state.isEditMode).toBe(false);
	});

	it('should restore UI elements', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.isEditMode = true;

		exitEditMode(state, els);

		expect(els.editBtn.classList.remove).toHaveBeenCalledWith('active');
		expect(els.editBtn.textContent).toBe('Edit Subtitles');
		expect(els.editActions.classList.remove).toHaveBeenCalledWith('visible');
		expect(els.addSegmentBtn.classList.remove).toHaveBeenCalledWith('visible');
		expect(els.fullText.style.display).toBe('block');
	});

	it('should clear unsaved changes', () => {
		const state = createSegmentEditorState();
		const els = makeMockElements();
		state.isEditMode = true;
		state.hasUnsavedChanges = true;

		exitEditMode(state, els);

		expect(state.hasUnsavedChanges).toBe(false);
		expect(els.unsavedIndicator.classList.remove).toHaveBeenCalledWith('visible');
	});
});

describe('addNewSegment', () => {
	it('should add segment starting at 0 when no segments exist', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [];
		const els = makeMockElements();

		addNewSegment(state, els);

		expect(state.editedSegments).toHaveLength(1);
		expect(state.editedSegments[0].start).toBe(0);
		expect(state.editedSegments[0].end).toBe(3);
		expect(state.editedSegments[0].text).toBe('');
	});

	it('should add segment starting at end of last segment', () => {
		const state = createSegmentEditorState();
		state.editedSegments = makeSegments(2); // ends at 10
		const els = makeMockElements();

		addNewSegment(state, els);

		expect(state.editedSegments).toHaveLength(3);
		expect(state.editedSegments[2].start).toBe(10);
		expect(state.editedSegments[2].end).toBe(13);
	});

	it('should push to history before adding', () => {
		const state = createSegmentEditorState();
		state.editedSegments = makeSegments(1);
		const els = makeMockElements();

		addNewSegment(state, els);

		expect(state.undoStack).toHaveLength(1);
		expect(state.undoStack[0]).toHaveLength(1);
	});

	it('should mark as unsaved', () => {
		const state = createSegmentEditorState();
		state.editedSegments = [];
		const els = makeMockElements();

		addNewSegment(state, els);

		expect(state.hasUnsavedChanges).toBe(true);
		expect(els.unsavedIndicator.classList.add).toHaveBeenCalledWith('visible');
	});

	it('should assign correct id to new segment', () => {
		const state = createSegmentEditorState();
		state.editedSegments = makeSegments(3);
		const els = makeMockElements();

		addNewSegment(state, els);

		expect(state.editedSegments[3].id).toBe(3);
	});
});

describe('getCurrentSegmentIndex', () => {
	it('should return index of segment containing current time', () => {
		const segments = makeSegments(5); // 0-5, 5-10, 10-15, 15-20, 20-25
		expect(getCurrentSegmentIndex(segments, 7)).toBe(1);
	});

	it('should return first segment for time at segment start', () => {
		const segments = makeSegments(3);
		expect(getCurrentSegmentIndex(segments, 0)).toBe(0);
	});

	it('should return segment for time at boundary (matches first segment)', () => {
		const segments = makeSegments(3);
		// Time 5 matches seg 0 (end=5) since condition is <=, returns first match
		expect(getCurrentSegmentIndex(segments, 5)).toBe(0);
	});

	it('should return previous segment for time between segments', () => {
		const segments = [
			{ id: 0, start: 0, end: 3, text: 'a' },
			{ id: 1, start: 5, end: 8, text: 'b' },
		];
		// Time 4 is between segments: after seg 0 ends (3) but before seg 1 starts (5)
		expect(getCurrentSegmentIndex(segments, 4)).toBe(0);
	});

	it('should return last segment for time beyond all segments', () => {
		const segments = makeSegments(3); // ends at 15
		expect(getCurrentSegmentIndex(segments, 100)).toBe(2);
	});

	it('should return 0 for time before any segment with gap', () => {
		const segments = [
			{ id: 0, start: 5, end: 10, text: 'a' },
			{ id: 1, start: 10, end: 15, text: 'b' },
		];
		expect(getCurrentSegmentIndex(segments, 2)).toBe(0);
	});
});

describe('isTextInputFocused', () => {
	let mockDocument: { activeElement: unknown };

	beforeEach(() => {
		mockDocument = { activeElement: null };
		vi.stubGlobal('document', mockDocument);
	});

	it('should return false when no element is focused', () => {
		mockDocument.activeElement = null;
		expect(isTextInputFocused()).toBe(false);
	});

	it('should return true when input is focused', () => {
		mockDocument.activeElement = { tagName: 'INPUT', isContentEditable: false };
		expect(isTextInputFocused()).toBe(true);
	});

	it('should return true when textarea is focused', () => {
		mockDocument.activeElement = { tagName: 'TEXTAREA', isContentEditable: false };
		expect(isTextInputFocused()).toBe(true);
	});

	it('should return true when contentEditable element is focused', () => {
		mockDocument.activeElement = { tagName: 'DIV', isContentEditable: true };
		expect(isTextInputFocused()).toBe(true);
	});

	it('should return false when a non-input element is focused', () => {
		mockDocument.activeElement = { tagName: 'DIV', isContentEditable: false };
		expect(isTextInputFocused()).toBe(false);
	});
});

describe('scrollIntoContainerView', () => {
	it('should scroll up when element is above container viewport', () => {
		const container = {
			scrollTop: 100,
			getBoundingClientRect: () => ({ top: 100, bottom: 400 }),
		} as unknown as HTMLElement;
		const element = {
			getBoundingClientRect: () => ({ top: 50, bottom: 80 }),
		} as unknown as HTMLElement;

		scrollIntoContainerView(container, element);

		// Should scroll up by 50 (containerTop - elementTop = 100 - 50)
		expect(container.scrollTop).toBe(50);
	});

	it('should scroll down when element is below container viewport', () => {
		const container = {
			scrollTop: 0,
			getBoundingClientRect: () => ({ top: 0, bottom: 300 }),
		} as unknown as HTMLElement;
		const element = {
			getBoundingClientRect: () => ({ top: 280, bottom: 350 }),
		} as unknown as HTMLElement;

		scrollIntoContainerView(container, element);

		// Should scroll down by 50 (elementBottom - containerBottom = 350 - 300)
		expect(container.scrollTop).toBe(50);
	});

	it('should not scroll when element is fully visible', () => {
		const container = {
			scrollTop: 0,
			getBoundingClientRect: () => ({ top: 0, bottom: 400 }),
		} as unknown as HTMLElement;
		const element = {
			getBoundingClientRect: () => ({ top: 100, bottom: 200 }),
		} as unknown as HTMLElement;

		scrollIntoContainerView(container, element);

		expect(container.scrollTop).toBe(0);
	});
});

describe('saveSegments', () => {
	it('should do nothing when currentUploadId is null', async () => {
		const { csrfFetch } = await import('./csrf');
		const state = createSegmentEditorState();
		state.currentUploadId = null;
		const els = makeMockElements();
		const callbacks = makeCallbacks();

		await saveSegments(state, els, callbacks);

		expect(csrfFetch).not.toHaveBeenCalled();
	});

	it('should call csrfFetch with correct endpoint', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({ status: 'ok' }),
		});

		const state = createSegmentEditorState();
		state.currentUploadId = 'upload-123';
		state.editedSegments = makeSegments(2);
		const els = makeMockElements();
		const callbacks = makeCallbacks();

		await saveSegments(state, els, callbacks);

		expect(mockCsrfFetch).toHaveBeenCalledWith(
			'/api/transcribe/upload-123/segments',
			expect.objectContaining({
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
			})
		);
	});

	it('should show success status on successful save', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({ status: 'ok' }),
		});

		const state = createSegmentEditorState();
		state.currentUploadId = 'upload-123';
		state.editedSegments = makeSegments(2);
		state.isEditMode = true;
		const els = makeMockElements();
		const callbacks = makeCallbacks();

		await saveSegments(state, els, callbacks);

		expect(callbacks.showStatus).toHaveBeenCalledWith('Subtitles saved successfully!', 'success');
	});

	it('should show error status on failed save', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: false,
			json: async () => ({ error: 'Validation failed' }),
		});

		const state = createSegmentEditorState();
		state.currentUploadId = 'upload-123';
		state.editedSegments = makeSegments(1);
		const els = makeMockElements();
		const callbacks = makeCallbacks();

		await saveSegments(state, els, callbacks);

		expect(callbacks.showStatus).toHaveBeenCalledWith(
			expect.stringContaining('Failed to save'),
			'error'
		);
	});

	it('should disable save button during save and re-enable after', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({ status: 'ok' }),
		});

		const state = createSegmentEditorState();
		state.currentUploadId = 'upload-123';
		state.editedSegments = makeSegments(1);
		state.isEditMode = true;
		const els = makeMockElements();
		const callbacks = makeCallbacks();

		await saveSegments(state, els, callbacks);

		expect(els.saveBtn.disabled).toBe(false);
		expect(els.saveBtn.textContent).toBe('Save Changes');
	});

	it('should update transcriptionSegments to match edited on success', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({ status: 'ok' }),
		});

		const state = createSegmentEditorState();
		state.currentUploadId = 'upload-123';
		state.editedSegments = makeSegments(3);
		state.transcriptionSegments = makeSegments(1);
		state.isEditMode = true;
		const els = makeMockElements();
		const callbacks = makeCallbacks();

		await saveSegments(state, els, callbacks);

		expect(state.transcriptionSegments).toHaveLength(3);
	});

	it('should handle fetch rejection', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockRejectedValueOnce(new Error('Network error'));

		const state = createSegmentEditorState();
		state.currentUploadId = 'upload-123';
		state.editedSegments = makeSegments(1);
		const els = makeMockElements();
		const callbacks = makeCallbacks();

		await saveSegments(state, els, callbacks);

		expect(callbacks.showStatus).toHaveBeenCalledWith(
			expect.stringContaining('Network error'),
			'error'
		);
		expect(els.saveBtn.disabled).toBe(false);
	});
});

describe('renderSegments', () => {
	it('should render segments as HTML', () => {
		const state = createSegmentEditorState();
		state.transcriptionSegments = makeSegments(2);
		const els = makeMockElements();

		renderSegments(state, els);

		expect(els.segments.innerHTML).toContain('data-index="0"');
		expect(els.segments.innerHTML).toContain('data-index="1"');
		expect(els.segments.innerHTML).toContain('Segment 0');
		expect(els.segments.innerHTML).toContain('Segment 1');
	});

	it('should render edited segments in edit mode', () => {
		const state = createSegmentEditorState();
		state.isEditMode = true;
		state.transcriptionSegments = makeSegments(1);
		state.editedSegments = [{ id: 0, start: 0, end: 5, text: 'Edited text' }];
		const els = makeMockElements();

		renderSegments(state, els);

		expect(els.segments.innerHTML).toContain('Edited text');
		expect(els.segments.innerHTML).toContain('editing');
	});

	it('should not render when segments are empty', () => {
		const state = createSegmentEditorState();
		state.transcriptionSegments = [];
		const els = makeMockElements();
		els.segments.innerHTML = 'previous';

		renderSegments(state, els);

		expect(els.segments.innerHTML).toBe('previous');
	});
});

describe('setupFeedbackHandler', () => {
	it('should attach click event listener to feedback element', () => {
		const state = createSegmentEditorState();
		const feedbackEl = {
			addEventListener: vi.fn(),
		} as unknown as HTMLElement;

		setupFeedbackHandler(state, feedbackEl);

		expect(feedbackEl.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
	});
});

describe('navigateToSegment', () => {
	function makeMockSegmentEl() {
		return {
			classList: { add: vi.fn(), remove: vi.fn() },
			getBoundingClientRect: vi.fn(() => ({ top: 100, bottom: 130 })),
		};
	}

	it('should do nothing when no segments exist', () => {
		const state = createSegmentEditorState();
		state.transcriptionSegments = [];
		const els = makeMockElements();

		navigateToSegment(state, els, els.previewVideo, 0);

		expect(els.previewVideo.currentTime).toBe(0);
	});

	it('should clamp index to valid range', () => {
		const state = createSegmentEditorState();
		state.transcriptionSegments = makeSegments(3);
		const els = makeMockElements();
		// Add getBoundingClientRect to segments container for scrollIntoContainerView
		(els.segments as unknown as Record<string, unknown>).getBoundingClientRect =
			vi.fn(() => ({ top: 0, bottom: 400 }));
		(els.segments as unknown as Record<string, unknown>).scrollTop = 0;
		(els.segments.querySelectorAll as ReturnType<typeof vi.fn>).mockReturnValue(
			[makeMockSegmentEl(), makeMockSegmentEl(), makeMockSegmentEl()]
		);

		navigateToSegment(state, els, els.previewVideo, 10);

		// Should clamp to last segment (index 2, start = 10)
		expect(els.previewVideo.currentTime).toBe(10);
	});

	it('should clamp negative index to 0', () => {
		const state = createSegmentEditorState();
		state.transcriptionSegments = makeSegments(3);
		const els = makeMockElements();
		(els.segments as unknown as Record<string, unknown>).getBoundingClientRect =
			vi.fn(() => ({ top: 0, bottom: 400 }));
		(els.segments as unknown as Record<string, unknown>).scrollTop = 0;
		(els.segments.querySelectorAll as ReturnType<typeof vi.fn>).mockReturnValue(
			[makeMockSegmentEl(), makeMockSegmentEl(), makeMockSegmentEl()]
		);

		navigateToSegment(state, els, els.previewVideo, -5);

		expect(els.previewVideo.currentTime).toBe(0);
	});
});
