import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	setupSubtitleSync,
	createSubtitleUpdater,
	createSpeedUIState,
	updateSpeedUI,
	showSpeedIndicator,
	createBurnState,
	startBurn,
	type SpeedUIState,
	type BurnState,
	type BurnElements,
} from './subtitle-sync';
import type { SegmentEditorState } from './segment-editor';
import { createSegmentEditorState } from './segment-editor';

// --- Mocks ---

vi.mock('./csrf', () => ({
	csrfFetch: vi.fn(),
}));

vi.mock('./bionic', () => ({
	renderBionicText: vi.fn((text: string) => `<b>${text}</b>`),
	isBionicEnabled: vi.fn(() => false),
}));

vi.mock('./playback-speed', () => ({
	PLAYBACK_SPEEDS: [0.8, 0.9, 1.0],
	getSavedSpeed: vi.fn(() => 1.0),
	setPlaybackSpeed: vi.fn(),
	increaseSpeed: vi.fn((s: number) => Math.min(s + 0.1, 1.0)),
	decreaseSpeed: vi.fn((s: number) => Math.max(s - 0.1, 0.8)),
	formatSpeed: vi.fn((s: number) => `${s}x`),
}));

vi.mock('./transcription-polling', () => ({
	formatDuration: vi.fn((s: number) => `${s}s`),
}));

vi.mock('./segment-editor', async (importOriginal) => {
	const actual = await importOriginal<typeof import('./segment-editor')>();
	return {
		...actual,
		updateFeedbackButtons: vi.fn(),
		scrollIntoContainerView: vi.fn(),
		isTextInputFocused: vi.fn(() => false),
		getCurrentSegmentIndex: vi.fn(() => 0),
		navigateToSegment: vi.fn(),
		performUndo: vi.fn(),
		performRedo: vi.fn(),
	};
});

function makeSegments(count: number) {
	return Array.from({ length: count }, (_, i) => ({
		id: i,
		start: i * 5,
		end: (i + 1) * 5,
		text: `Segment ${i}`,
	}));
}

function makeMockBurnElements(): BurnElements {
	return {
		burnBtn: { disabled: false, textContent: '' } as unknown as HTMLButtonElement,
		burnStatus: { classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		burnStatusText: { textContent: '' } as unknown as HTMLElement,
		burnEtaText: { textContent: '', style: { display: '' } } as unknown as HTMLElement,
		burnProgressFill: { style: { width: '' } } as unknown as HTMLElement,
		downloadBurnedBtn: { style: { display: '' }, href: '' } as unknown as HTMLAnchorElement,
	};
}

beforeEach(() => {
	vi.clearAllMocks();
	vi.useFakeTimers();
});

afterEach(() => {
	vi.useRealTimers();
	vi.restoreAllMocks();
});

// --- Tests ---

describe('setupSubtitleSync', () => {
	it('should add timeupdate listener and set active', () => {
		const video = {
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
		} as unknown as HTMLVideoElement;
		const updateFn = vi.fn();
		const syncState = { active: false };

		setupSubtitleSync(video, updateFn, syncState);

		expect(video.addEventListener).toHaveBeenCalledWith('timeupdate', updateFn);
		expect(syncState.active).toBe(true);
	});

	it('should remove existing listener before adding new one when active', () => {
		const video = {
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
		} as unknown as HTMLVideoElement;
		const updateFn = vi.fn();
		const syncState = { active: true };

		setupSubtitleSync(video, updateFn, syncState);

		expect(video.removeEventListener).toHaveBeenCalledWith('timeupdate', updateFn);
		expect(video.addEventListener).toHaveBeenCalledWith('timeupdate', updateFn);
	});

	it('should not remove listener when not active', () => {
		const video = {
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
		} as unknown as HTMLVideoElement;
		const updateFn = vi.fn();
		const syncState = { active: false };

		setupSubtitleSync(video, updateFn, syncState);

		expect(video.removeEventListener).not.toHaveBeenCalled();
	});
});

describe('createSubtitleUpdater', () => {
	it('should return a function', () => {
		const state = createSegmentEditorState();
		const video = { currentTime: 0 } as unknown as HTMLVideoElement;
		const subtitle = { textContent: '', innerHTML: '' } as unknown as HTMLElement;
		const feedback = {} as unknown as HTMLElement;
		const container = {} as unknown as HTMLElement;

		const updater = createSubtitleUpdater(state, video, subtitle, feedback, container);

		expect(typeof updater).toBe('function');
	});

	it('should set subtitle text when time matches a segment', () => {
		const state = createSegmentEditorState();
		state.transcriptionSegments = makeSegments(3);
		state.cachedSegmentElements = [];
		const video = { currentTime: 7 } as unknown as HTMLVideoElement;
		const subtitle = { textContent: '', innerHTML: '' } as unknown as HTMLElement;
		const feedback = {} as unknown as HTMLElement;
		const container = {} as unknown as HTMLElement;

		const updater = createSubtitleUpdater(state, video, subtitle, feedback, container);
		updater();

		expect(subtitle.textContent).toBe('Segment 1');
	});

	it('should clear subtitle text when no segment matches', () => {
		const state = createSegmentEditorState();
		state.transcriptionSegments = [
			{ id: 0, start: 0, end: 3, text: 'Hello' },
		];
		state.cachedSegmentElements = [];
		const video = { currentTime: 10 } as unknown as HTMLVideoElement;
		const subtitle = { textContent: 'old text', innerHTML: '' } as unknown as HTMLElement;
		const feedback = {} as unknown as HTMLElement;
		const container = {} as unknown as HTMLElement;

		const updater = createSubtitleUpdater(state, video, subtitle, feedback, container);
		updater();

		expect(subtitle.textContent).toBe('');
	});

	it('should use bionic rendering when enabled', async () => {
		const { isBionicEnabled, renderBionicText } = await import('./bionic');
		(isBionicEnabled as ReturnType<typeof vi.fn>).mockReturnValue(true);

		const state = createSegmentEditorState();
		state.transcriptionSegments = makeSegments(1);
		state.cachedSegmentElements = [];
		const video = { currentTime: 2 } as unknown as HTMLVideoElement;
		const subtitle = { textContent: '', innerHTML: '' } as unknown as HTMLElement;
		const feedback = {} as unknown as HTMLElement;
		const container = {} as unknown as HTMLElement;

		const updater = createSubtitleUpdater(state, video, subtitle, feedback, container);
		updater();

		expect(renderBionicText).toHaveBeenCalled();
		expect(subtitle.innerHTML).toContain('<b>');
	});

	it('should highlight active segment and remove previous highlight', () => {
		const state = createSegmentEditorState();
		state.transcriptionSegments = makeSegments(3);
		state.lastActiveSegmentIndex = 0;
		const prevElement = { classList: { add: vi.fn(), remove: vi.fn() } };
		const currentElement = { classList: { add: vi.fn(), remove: vi.fn() } };
		state.cachedSegmentElements = [
			prevElement as unknown as HTMLElement,
			currentElement as unknown as HTMLElement,
			{ classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		];

		const video = { currentTime: 7 } as unknown as HTMLVideoElement;
		const subtitle = { textContent: '', innerHTML: '' } as unknown as HTMLElement;
		const feedback = {} as unknown as HTMLElement;
		const container = {} as unknown as HTMLElement;

		const updater = createSubtitleUpdater(state, video, subtitle, feedback, container);
		updater();

		expect(prevElement.classList.remove).toHaveBeenCalledWith('active');
		expect(currentElement.classList.add).toHaveBeenCalledWith('active');
		expect(state.lastActiveSegmentIndex).toBe(1);
	});

	it('should not update highlight when segment index unchanged', async () => {
		const segEditorMock = await import('./segment-editor');
		const state = createSegmentEditorState();
		state.transcriptionSegments = makeSegments(2);
		state.lastActiveSegmentIndex = 0;
		const element0 = { classList: { add: vi.fn(), remove: vi.fn() } };
		state.cachedSegmentElements = [
			element0 as unknown as HTMLElement,
			{ classList: { add: vi.fn(), remove: vi.fn() } } as unknown as HTMLElement,
		];

		const video = { currentTime: 2 } as unknown as HTMLVideoElement;
		const subtitle = { textContent: '', innerHTML: '' } as unknown as HTMLElement;
		const feedback = {} as unknown as HTMLElement;
		const container = {} as unknown as HTMLElement;

		const updater = createSubtitleUpdater(state, video, subtitle, feedback, container);
		updater();

		// Same segment as lastActiveSegmentIndex, so no classList updates
		expect(element0.classList.add).not.toHaveBeenCalled();
		expect(segEditorMock.updateFeedbackButtons).not.toHaveBeenCalled();
	});
});

describe('createSpeedUIState', () => {
	it('should return default state', () => {
		const state = createSpeedUIState();
		expect(state.speedMenuOpen).toBe(false);
		expect(state.speedIndicatorTimeout).toBeNull();
		expect(state.abortController).toBeNull();
	});
});

describe('updateSpeedUI', () => {
	it('should set speed button text', () => {
		const speedBtn = { textContent: '' } as unknown as HTMLButtonElement;
		const activeOption = {
			dataset: { speed: '0.9' },
			classList: { add: vi.fn(), remove: vi.fn() },
		};
		const inactiveOption = {
			dataset: { speed: '1' },
			classList: { add: vi.fn(), remove: vi.fn() },
		};
		const speedOptions = {
			querySelectorAll: vi.fn(() => [activeOption, inactiveOption]),
		} as unknown as HTMLElement;

		updateSpeedUI(speedBtn, speedOptions, 0.9);

		expect(speedBtn.textContent).toBe('0.9x');
		expect(activeOption.classList.add).toHaveBeenCalledWith('active');
		expect(inactiveOption.classList.remove).toHaveBeenCalledWith('active');
	});
});

describe('showSpeedIndicator', () => {
	it('should show indicator and hide after timeout', () => {
		const indicator = {
			textContent: '',
			classList: { add: vi.fn(), remove: vi.fn() },
		} as unknown as HTMLElement;
		const speedState = createSpeedUIState();

		showSpeedIndicator(indicator, speedState, 0.9);

		expect(indicator.textContent).toBe('0.9x');
		expect(indicator.classList.add).toHaveBeenCalledWith('visible');
		expect(speedState.speedIndicatorTimeout).not.toBeNull();

		vi.advanceTimersByTime(800);

		expect(indicator.classList.remove).toHaveBeenCalledWith('visible');
		expect(speedState.speedIndicatorTimeout).toBeNull();
	});

	it('should clear previous timeout before setting new one', () => {
		const indicator = {
			textContent: '',
			classList: { add: vi.fn(), remove: vi.fn() },
		} as unknown as HTMLElement;
		const speedState = createSpeedUIState();

		showSpeedIndicator(indicator, speedState, 0.8);
		const firstTimeout = speedState.speedIndicatorTimeout;

		showSpeedIndicator(indicator, speedState, 0.9);

		expect(speedState.speedIndicatorTimeout).not.toBe(firstTimeout);
	});
});

describe('createBurnState', () => {
	it('should return default state', () => {
		const state = createBurnState();
		expect(state.isBurning).toBe(false);
		expect(state.burnPollTimeout).toBeNull();
		expect(state.fetchAbortController).toBeNull();
	});
});

describe('startBurn', () => {
	it('should do nothing when already burning', async () => {
		const { csrfFetch } = await import('./csrf');
		const burnState = createBurnState();
		burnState.isBurning = true;
		const els = makeMockBurnElements();

		await startBurn('upload-1', burnState, els);

		expect(csrfFetch).not.toHaveBeenCalled();
	});

	it('should set burning state and disable button', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({ status: 'ok' }),
		});
		// Mock global fetch for pollBurnStatus (it uses raw fetch, not csrfFetch)
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ status: 'complete', message: 'Done', progress: 100 }),
		}));

		const burnState = createBurnState();
		const els = makeMockBurnElements();

		await startBurn('upload-1', burnState, els);

		expect(mockCsrfFetch).toHaveBeenCalledWith('/api/videos/upload-1/burn', { method: 'POST' });
		expect(els.burnBtn.textContent).not.toBe('');
	});

	it('should show error on failed burn start', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: false,
			json: async () => ({ error: 'Video not found' }),
		});

		const burnState = createBurnState();
		const els = makeMockBurnElements();

		await startBurn('upload-1', burnState, els);

		expect(els.burnStatus.classList.add).toHaveBeenCalledWith('error');
		expect(els.burnStatusText.textContent).toContain('Video not found');
		expect(els.burnBtn.disabled).toBe(false);
		expect(burnState.isBurning).toBe(false);
	});

	it('should handle fetch rejection', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockRejectedValueOnce(new Error('Network error'));

		const burnState = createBurnState();
		const els = makeMockBurnElements();

		await startBurn('upload-1', burnState, els);

		expect(els.burnStatus.classList.add).toHaveBeenCalledWith('error');
		expect(els.burnStatusText.textContent).toContain('Network error');
		expect(burnState.isBurning).toBe(false);
	});

	it('should update UI elements at burn start', async () => {
		const { csrfFetch } = await import('./csrf');
		const mockCsrfFetch = csrfFetch as ReturnType<typeof vi.fn>;
		mockCsrfFetch.mockResolvedValueOnce({
			ok: true,
			json: async () => ({ status: 'ok' }),
		});
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ status: 'complete', message: 'Done', progress: 100 }),
		}));

		const burnState = createBurnState();
		const els = makeMockBurnElements();

		await startBurn('upload-1', burnState, els);

		expect(els.burnStatus.classList.add).toHaveBeenCalledWith('visible');
		expect(els.burnStatus.classList.remove).toHaveBeenCalledWith('complete', 'error');
		expect(els.burnProgressFill.style.width).not.toBe('');
		expect(els.downloadBurnedBtn.style.display).toBe('none');
	});
});
