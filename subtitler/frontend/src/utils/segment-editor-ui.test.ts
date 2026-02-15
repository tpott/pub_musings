import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { TranscriptionSegment } from '../types/transcription';
import {
	createSegmentEditorState,
	setSegmentFeedback,
	updateFeedbackButtons,
	renderSegments,
	setupFeedbackHandler,
	navigateToSegment,
	type SegmentEditorElements,
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

beforeEach(() => {
	localStorageMock.clear();
	vi.stubGlobal('localStorage', localStorageMock);
	vi.clearAllMocks();
});

afterEach(() => {
	vi.restoreAllMocks();
});

// --- Tests ---

describe('updateFeedbackButtons', () => {
	it('should hide feedback when index is negative', () => {
		const state = createSegmentEditorState();
		const feedbackEl = {
			style: { display: '' },
			querySelectorAll: vi.fn(() => []),
			querySelector: vi.fn(() => null),
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
		const mockDot = { className: 'feedback-dot' };
		const mockOptions = { style: { display: 'flex' } };
		const feedbackEl = {
			style: { display: 'none' },
			querySelectorAll: vi.fn(() => [mockBtn]),
			querySelector: vi.fn((sel: string) => {
				if (sel === '.feedback-dot') return mockDot;
				if (sel === '.feedback-options') return mockOptions;
				return null;
			}),
		} as unknown as HTMLElement;

		updateFeedbackButtons(state, feedbackEl, 2);

		expect(feedbackEl.style.display).toBe('');
		expect(state.feedbackActiveIndex).toBe(2);
		// Options should collapse on segment change
		expect(mockOptions.style.display).toBe('none');
	});

	it('should toggle active class on matching feedback button', () => {
		const state = createSegmentEditorState();
		state.currentUploadId = 'upload-1';
		setSegmentFeedback('upload-1', 0, 'early');

		const btnGood = {
			getAttribute: vi.fn(() => 'good'),
			classList: { toggle: vi.fn() },
		};
		const btnEarly = {
			getAttribute: vi.fn(() => 'early'),
			classList: { toggle: vi.fn() },
		};
		const btnLate = {
			getAttribute: vi.fn(() => 'late'),
			classList: { toggle: vi.fn() },
		};
		const mockDot = { className: 'feedback-dot', classList: { add: vi.fn() } };
		const mockOptions = { style: { display: 'none' } };
		const feedbackEl = {
			style: { display: '' },
			querySelectorAll: vi.fn(() => [btnGood, btnEarly, btnLate]),
			querySelector: vi.fn((sel: string) => {
				if (sel === '.feedback-dot') return mockDot;
				if (sel === '.feedback-options') return mockOptions;
				return null;
			}),
		} as unknown as HTMLElement;

		updateFeedbackButtons(state, feedbackEl, 0);

		expect(btnGood.classList.toggle).toHaveBeenCalledWith('active', false);
		expect(btnEarly.classList.toggle).toHaveBeenCalledWith('active', true);
		expect(btnLate.classList.toggle).toHaveBeenCalledWith('active', false);
		// Dot should show early color
		expect(mockDot.classList.add).toHaveBeenCalledWith('early');
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
			querySelector: vi.fn(() => null),
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
