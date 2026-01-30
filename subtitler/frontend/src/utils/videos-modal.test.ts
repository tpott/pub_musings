import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	createModalState,
	formatTime,
	setupModalListeners,
	type ModalElements,
	type ModalState,
} from './videos-modal';

// --- Mocks ---

vi.mock('./bionic', () => ({
	renderBionicText: vi.fn((text: string) => `<b>${text}</b>`),
	isBionicEnabled: vi.fn(() => false),
}));

vi.mock('./html', () => ({
	escapeHtml: vi.fn((text: string) => text),
}));

vi.mock('./playback-speed', () => ({
	PLAYBACK_SPEEDS: [0.8, 0.9, 1.0],
	getSavedSpeed: vi.fn(() => 1.0),
	setPlaybackSpeed: vi.fn(),
	increaseSpeed: vi.fn((s: number) => Math.min(s + 0.1, 1.0)),
	decreaseSpeed: vi.fn((s: number) => Math.max(s - 0.1, 0.8)),
	formatSpeed: vi.fn((s: number) => `${s}x`),
}));

vi.mock('./subtitles', () => ({
	openSRT: vi.fn(),
	openVTT: vi.fn(),
	openJSON: vi.fn(),
}));

vi.mock('./dialog', () => ({
	showAlert: vi.fn(() => Promise.resolve()),
}));

vi.mock('./api-schemas', () => ({
	TranscriptionStatusResponseSchema: {},
	safeParse: vi.fn((schema: unknown, data: unknown) => data),
}));

function makeMockModalElements(): ModalElements {
	return {
		videoModal: {
			classList: { add: vi.fn(), remove: vi.fn(), contains: vi.fn(() => false) },
			querySelectorAll: vi.fn(() => []),
			addEventListener: vi.fn(),
		} as unknown as HTMLElement,
		modalClose: {
			focus: vi.fn(),
			addEventListener: vi.fn(),
		} as unknown as HTMLButtonElement,
		modalTitle: { textContent: '' } as unknown as HTMLElement,
		modalLoading: { style: { display: '' } } as unknown as HTMLElement,
		modalError: { style: { display: '' }, textContent: '' } as unknown as HTMLElement,
		modalVideoContainer: { style: { display: '' } } as unknown as HTMLElement,
		modalVideo: {
			currentTime: 0,
			duration: 100,
			playbackRate: 1.0,
			paused: true,
			src: '',
			play: vi.fn(() => Promise.resolve()),
			pause: vi.fn(),
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
		} as unknown as HTMLVideoElement,
		modalCurrentSubtitle: { textContent: '', innerHTML: '' } as unknown as HTMLElement,
		modalSegments: {
			innerHTML: '',
			querySelectorAll: vi.fn(() => []),
			addEventListener: vi.fn(),
			clientHeight: 400,
			scrollTop: 0,
		} as unknown as HTMLElement,
		modalDownloadSrt: { addEventListener: vi.fn(), href: '' } as unknown as HTMLAnchorElement,
		modalDownloadVtt: { addEventListener: vi.fn(), href: '' } as unknown as HTMLAnchorElement,
		modalDownloadJson: { addEventListener: vi.fn(), href: '' } as unknown as HTMLAnchorElement,
		modalEditLink: { addEventListener: vi.fn(), href: '' } as unknown as HTMLAnchorElement,
		modalSpeedBtn: {
			textContent: '',
			setAttribute: vi.fn(),
			addEventListener: vi.fn(),
		} as unknown as HTMLButtonElement,
		modalSpeedOptions: {
			classList: { add: vi.fn(), remove: vi.fn(), toggle: vi.fn() },
			querySelectorAll: vi.fn(() => []),
			addEventListener: vi.fn(),
		} as unknown as HTMLElement,
		modalSpeedDropdown: {
			contains: vi.fn(() => false),
		} as unknown as HTMLElement,
		modalSpeedIndicator: {
			textContent: '',
			classList: { add: vi.fn(), remove: vi.fn() },
		} as unknown as HTMLElement,
	};
}

let mockDocument: Record<string, unknown>;

beforeEach(() => {
	vi.clearAllMocks();
	mockDocument = {
		body: { style: { overflow: '' } },
		activeElement: null,
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		contains: vi.fn(() => true),
	};
	vi.stubGlobal('document', mockDocument);
	vi.stubGlobal('window', {});
	vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
	vi.restoreAllMocks();
});

// --- Tests ---

describe('createModalState', () => {
	it('should return default state', () => {
		const state = createModalState();
		expect(state.currentVideoId).toBeNull();
		expect(state.subtitleSegments).toEqual([]);
		expect(state.subtitleSyncActive).toBe(false);
		expect(state.triggerElement).toBeNull();
		expect(state.keydownController).toBeNull();
		expect(state.cachedSegmentElements).toEqual([]);
		expect(state.lastActiveSegmentIndex).toBe(-1);
		expect(state.speedMenuOpen).toBe(false);
		expect(state.speedIndicatorTimeout).toBeNull();
	});

	it('should return independent state objects', () => {
		const state1 = createModalState();
		const state2 = createModalState();
		state1.currentVideoId = 'video-1';
		expect(state2.currentVideoId).toBeNull();
	});
});

describe('formatTime', () => {
	it('should format seconds without hours', () => {
		expect(formatTime(65.5)).toBe('1:05.500');
	});

	it('should format zero', () => {
		expect(formatTime(0)).toBe('0:00.000');
	});

	it('should format with hours when >= 3600', () => {
		expect(formatTime(3661.123)).toBe('1:01:01.123');
	});

	it('should pad minutes and seconds', () => {
		// 5.1 in IEEE 754 is 5.09999..., so ms = floor(0.09999 * 1000) = 99
		expect(formatTime(5.1)).toBe('0:05.099');
	});

	it('should handle exact minute boundaries', () => {
		expect(formatTime(60)).toBe('1:00.000');
	});

	it('should handle large values with hours', () => {
		expect(formatTime(7200)).toBe('2:00:00.000');
	});

	it('should truncate milliseconds (floor)', () => {
		// 1.9999 seconds -> ms = floor(0.9999 * 1000) = 999
		expect(formatTime(1.9999)).toBe('0:01.999');
	});
});

describe('setupModalListeners', () => {
	it('should return openVideoModal, closeVideoModal, and setTriggerElement', () => {
		const els = makeMockModalElements();
		const state = createModalState();

		const result = setupModalListeners(els, state);

		expect(typeof result.openVideoModal).toBe('function');
		expect(typeof result.closeVideoModal).toBe('function');
		expect(typeof result.setTriggerElement).toBe('function');
	});

	it('should register event listeners on modal elements', () => {
		const els = makeMockModalElements();
		const state = createModalState();

		setupModalListeners(els, state);

		expect(els.modalSegments.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.modalSegments.addEventListener).toHaveBeenCalledWith('keydown', expect.any(Function));
		expect(els.modalSpeedBtn.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.modalSpeedOptions.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.modalClose.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
	});

	it('should register subtitle view button listeners', () => {
		const els = makeMockModalElements();
		const state = createModalState();

		setupModalListeners(els, state);

		expect(els.modalDownloadSrt.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.modalDownloadVtt.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
		expect(els.modalDownloadJson.addEventListener).toHaveBeenCalledWith('click', expect.any(Function));
	});

	it('setTriggerElement should set state.triggerElement', () => {
		const els = makeMockModalElements();
		const state = createModalState();
		const { setTriggerElement } = setupModalListeners(els, state);

		const el = {} as HTMLElement;
		setTriggerElement(el);

		expect(state.triggerElement).toBe(el);
	});
});

describe('closeVideoModal', () => {
	it('should reset modal visibility and state', () => {
		const els = makeMockModalElements();
		const state = createModalState();
		state.currentVideoId = 'video-1';
		state.subtitleSegments = [{ id: 0, start: 0, end: 5, text: 'test' }];

		const { closeVideoModal } = setupModalListeners(els, state);
		closeVideoModal();

		expect(els.videoModal.classList.remove).toHaveBeenCalledWith('visible');
		expect(els.modalVideo.pause).toHaveBeenCalled();
		expect(els.modalVideo.src).toBe('');
		expect(state.currentVideoId).toBeNull();
		expect(state.subtitleSegments).toEqual([]);
	});

	it('should remove timeupdate listener when sync active after open', async () => {
		// Need to open first so updateSubtitleFn is set
		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({
				status: 'complete',
				result: { segments: [{ id: 0, start: 0, end: 5, text: 'test' }] },
			}),
		});
		vi.stubGlobal('fetch', mockFetch);

		const els = makeMockModalElements();
		const state = createModalState();
		const { openVideoModal, closeVideoModal } = setupModalListeners(els, state);

		await openVideoModal('video-1', 'test.mp4');
		expect(state.subtitleSyncActive).toBe(true);

		closeVideoModal();

		expect(els.modalVideo.removeEventListener).toHaveBeenCalledWith('timeupdate', expect.any(Function));
		expect(state.subtitleSyncActive).toBe(false);
	});

	it('should abort keydown controller', () => {
		const els = makeMockModalElements();
		const state = createModalState();
		const abortFn = vi.fn();
		state.keydownController = { abort: abortFn } as unknown as AbortController;

		const { closeVideoModal } = setupModalListeners(els, state);
		closeVideoModal();

		expect(abortFn).toHaveBeenCalled();
		expect(state.keydownController).toBeNull();
	});

	it('should focus trigger element if it exists in document', () => {
		const triggerEl = { focus: vi.fn() } as unknown as HTMLElement;
		const els = makeMockModalElements();
		const state = createModalState();
		state.triggerElement = triggerEl;
		(mockDocument.contains as ReturnType<typeof vi.fn>).mockReturnValue(true);

		const { closeVideoModal } = setupModalListeners(els, state);
		closeVideoModal();

		expect(triggerEl.focus).toHaveBeenCalled();
		expect(state.triggerElement).toBeNull();
	});
});

describe('openVideoModal', () => {
	it('should set state and show modal', async () => {
		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({
				status: 'complete',
				result: { segments: [{ id: 0, start: 0, end: 5, text: 'Hello' }] },
			}),
		});
		vi.stubGlobal('fetch', mockFetch);

		const els = makeMockModalElements();
		const state = createModalState();
		const { openVideoModal } = setupModalListeners(els, state);

		await openVideoModal('video-123', 'test.mp4');

		expect(state.currentVideoId).toBe('video-123');
		expect(els.modalTitle.textContent).toBe('test.mp4');
		expect(els.videoModal.classList.add).toHaveBeenCalledWith('visible');
		expect(els.modalClose.focus).toHaveBeenCalled();
	});

	it('should set video source and edit link', async () => {
		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({
				status: 'complete',
				result: { segments: [] },
			}),
		});
		vi.stubGlobal('fetch', mockFetch);

		const els = makeMockModalElements();
		const state = createModalState();
		const { openVideoModal } = setupModalListeners(els, state);

		await openVideoModal('vid-abc', 'movie.mp4');

		expect(els.modalVideo.src).toBe('/api/videos/vid-abc/video');
		expect(els.modalEditLink.href).toBe('/upload?id=vid-abc');
	});

	it('should show error when fetch fails', async () => {
		const mockFetch = vi.fn().mockRejectedValue(new Error('Network error'));
		vi.stubGlobal('fetch', mockFetch);

		const els = makeMockModalElements();
		const state = createModalState();
		const { openVideoModal } = setupModalListeners(els, state);

		await openVideoModal('video-1', 'test.mp4');

		expect(els.modalLoading.style.display).toBe('none');
		expect(els.modalError.style.display).toBe('block');
		expect(els.modalError.textContent).toContain('Network error');
	});

	it('should show error when response is not ok', async () => {
		const mockFetch = vi.fn().mockResolvedValue({
			ok: false,
			json: async () => ({ error: 'Not found' }),
		});
		vi.stubGlobal('fetch', mockFetch);

		const els = makeMockModalElements();
		const state = createModalState();
		const { openVideoModal } = setupModalListeners(els, state);

		await openVideoModal('video-1', 'test.mp4');

		expect(els.modalError.style.display).toBe('block');
		expect(els.modalError.textContent).toContain('Not found');
	});

	it('should show error when transcription not complete', async () => {
		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ status: 'pending' }),
		});
		vi.stubGlobal('fetch', mockFetch);

		const els = makeMockModalElements();
		const state = createModalState();
		const { openVideoModal } = setupModalListeners(els, state);

		await openVideoModal('video-1', 'test.mp4');

		expect(els.modalError.style.display).toBe('block');
		expect(els.modalError.textContent).toContain('not available');
	});

	it('should abort previous keydown controller', async () => {
		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({
				status: 'complete',
				result: { segments: [] },
			}),
		});
		vi.stubGlobal('fetch', mockFetch);

		const abortFn = vi.fn();
		const els = makeMockModalElements();
		const state = createModalState();
		state.keydownController = { abort: abortFn } as unknown as AbortController;

		const { openVideoModal } = setupModalListeners(els, state);
		await openVideoModal('video-1', 'test.mp4');

		expect(abortFn).toHaveBeenCalled();
	});

	it('should store subtitle segments from response', async () => {
		const segments = [
			{ id: 0, start: 0, end: 5, text: 'One' },
			{ id: 1, start: 5, end: 10, text: 'Two' },
		];
		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({
				status: 'complete',
				result: { segments },
			}),
		});
		vi.stubGlobal('fetch', mockFetch);

		const els = makeMockModalElements();
		const state = createModalState();
		const { openVideoModal } = setupModalListeners(els, state);

		await openVideoModal('video-1', 'test.mp4');

		expect(state.subtitleSegments).toEqual(segments);
		expect(state.subtitleSyncActive).toBe(true);
	});

	it('should show video container and hide loading on success', async () => {
		const mockFetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({
				status: 'complete',
				result: { segments: [] },
			}),
		});
		vi.stubGlobal('fetch', mockFetch);

		const els = makeMockModalElements();
		const state = createModalState();
		const { openVideoModal } = setupModalListeners(els, state);

		await openVideoModal('video-1', 'test.mp4');

		expect(els.modalLoading.style.display).toBe('none');
		expect(els.modalVideoContainer.style.display).toBe('block');
	});
});
