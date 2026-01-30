import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { startProgressPolling } from './videos-progress';
import type { Video } from './videos-list';

vi.mock('./session', () => ({
	getOrCreateSessionId: vi.fn(() => 'session-abc-123'),
}));

vi.mock('./html', () => ({
	escapeHtml: vi.fn((s: string) => s),
}));

vi.mock('./dialog', () => ({
	showAlert: vi.fn(() => Promise.resolve()),
	showConfirm: vi.fn(() => Promise.resolve(true)),
}));

function makeVideo(overrides: Partial<Video> = {}): Video {
	return {
		id: 'v1',
		filename: 'test.mp4',
		size: 1024 * 1024 * 10,
		created_at: '2026-01-30T12:00:00Z',
		transcription_status: 'processing',
		...overrides,
	};
}

/** Flush microtask queue so async poll() resolves (multiple rounds for chained promises) */
async function flushPromises(): Promise<void> {
	// Multiple rounds to flush chained .then() calls
	for (let i = 0; i < 5; i++) {
		await new Promise(resolve => {
			setTimeout(resolve, 0);
			vi.advanceTimersByTime(0);
		});
	}
}

/**
 * Create a minimal mock HTMLElement that supports the DOM operations
 * used by videos-progress.ts (querySelector, insertBefore, replaceWith).
 */
function createMockVideoList(videoIds: string[]): HTMLElement {
	const progressElements = new Map<string, { innerHTML: string; className: string }>();

	const cards = new Map<string, {
		innerHTML: string;
		className: string;
		querySelector: (sel: string) => unknown;
		insertBefore: (child: unknown, ref: unknown) => void;
		appendChild: (child: unknown) => void;
		replaceWith: (newEl: unknown) => void;
	}>();

	for (const id of videoIds) {
		const card = {
			innerHTML: '',
			className: 'video-card',
			querySelector(sel: string): unknown {
				if (sel === '.video-progress') {
					return progressElements.get(id) || null;
				}
				if (sel === '.video-actions') {
					return { className: 'video-actions' };
				}
				return null;
			},
			insertBefore(child: unknown) {
				const el = child as { className: string; innerHTML: string };
				progressElements.set(id, el);
			},
			appendChild(child: unknown) {
				const el = child as { className: string; innerHTML: string };
				progressElements.set(id, el);
			},
			replaceWith(newEl: unknown) {
				const el = newEl as { innerHTML: string };
				card.innerHTML = el.innerHTML || '';
			},
		};
		cards.set(id, card);
	}

	return {
		querySelector(sel: string): unknown {
			const match = sel.match(/data-video-id="([^"]+)"/);
			if (match) {
				return cards.get(match[1]) || null;
			}
			return null;
		},
	} as unknown as HTMLElement;
}

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
	vi.useFakeTimers();
	fetchMock = vi.fn();
	vi.stubGlobal('fetch', fetchMock);

	vi.stubGlobal('document', {
		createElement: vi.fn(() => {
			let storedHtml = '';
			const div = {
				get innerHTML() { return storedHtml; },
				set innerHTML(value: string) {
					storedHtml = value;
					div.firstElementChild = { innerHTML: value, className: 'video-card' };
				},
				firstElementChild: null as unknown,
			};
			return div;
		}),
	});
});

afterEach(() => {
	vi.useRealTimers();
	vi.restoreAllMocks();
});

describe('startProgressPolling', () => {
	it('should not start polling for completed videos', async () => {
		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'complete' })];

		const poller = startProgressPolling(videoList, videos, true);
		await flushPromises();
		poller.stop();

		expect(fetchMock).not.toHaveBeenCalled();
	});

	it('should not start polling for error videos', async () => {
		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'error' })];

		const poller = startProgressPolling(videoList, videos, true);
		await flushPromises();
		poller.stop();

		expect(fetchMock).not.toHaveBeenCalled();
	});

	it('should start polling for processing videos', async () => {
		fetchMock.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ status: 'complete' }),
		});

		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'processing' })];

		const poller = startProgressPolling(videoList, videos, true);
		await flushPromises();

		expect(fetchMock).toHaveBeenCalledWith('/api/transcribe/v1');
		poller.stop();
	});

	it('should start polling for pending videos', async () => {
		fetchMock.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ status: 'complete' }),
		});

		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'pending' })];

		const poller = startProgressPolling(videoList, videos, true);
		await flushPromises();

		expect(fetchMock).toHaveBeenCalledWith('/api/transcribe/v1');
		poller.stop();
	});

	it('should include session_id for anonymous users', async () => {
		fetchMock.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ status: 'complete' }),
		});

		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'processing' })];

		const poller = startProgressPolling(videoList, videos, false);
		await flushPromises();

		expect(fetchMock).toHaveBeenCalledWith(
			expect.stringContaining('session_id=session-abc-123')
		);
		poller.stop();
	});

	it('should update video status to complete when transcription finishes', async () => {
		fetchMock.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ status: 'complete' }),
		});

		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'processing' })];

		const poller = startProgressPolling(videoList, videos, true);
		await flushPromises();

		expect(videos[0].transcription_status).toBe('complete');
		poller.stop();
	});

	it('should update video status to error when transcription fails', async () => {
		fetchMock.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ status: 'error', message: 'Failed' }),
		});

		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'processing' })];

		const poller = startProgressPolling(videoList, videos, true);
		await flushPromises();

		expect(videos[0].transcription_status).toBe('error');
		poller.stop();
	});

	it('should stop polling when stop() is called', async () => {
		// First call: processing (will schedule retry), but we stop before retry
		fetchMock.mockResolvedValueOnce({
			ok: true,
			json: () => Promise.resolve({ status: 'processing', progress: 30 }),
		});

		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'processing' })];

		const poller = startProgressPolling(videoList, videos, true);
		await flushPromises();

		expect(fetchMock).toHaveBeenCalledTimes(1);

		poller.stop();

		// Advance timers — should NOT trigger another fetch
		vi.advanceTimersByTime(5000);
		await flushPromises();
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it('should stop polling on 403 error', async () => {
		fetchMock.mockResolvedValueOnce({
			ok: false,
			status: 403,
		});

		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'processing' })];

		const poller = startProgressPolling(videoList, videos, true);
		await flushPromises();

		vi.advanceTimersByTime(5000);
		await flushPromises();
		expect(fetchMock).toHaveBeenCalledTimes(1);

		poller.stop();
	});

	it('should stop polling on 404 error', async () => {
		fetchMock.mockResolvedValueOnce({
			ok: false,
			status: 404,
		});

		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'processing' })];

		const poller = startProgressPolling(videoList, videos, true);
		await flushPromises();

		vi.advanceTimersByTime(5000);
		await flushPromises();
		expect(fetchMock).toHaveBeenCalledTimes(1);

		poller.stop();
	});

	it('should retry on transient server errors', async () => {
		fetchMock
			.mockResolvedValueOnce({ ok: false, status: 500 })
			.mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({ status: 'complete' }),
			});

		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'processing' })];

		const poller = startProgressPolling(videoList, videos, true);

		// First poll (500 error) — schedules retry
		await flushPromises();
		expect(fetchMock).toHaveBeenCalledTimes(1);

		// Advance timer to trigger retry
		vi.advanceTimersByTime(3000);
		await flushPromises();
		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(videos[0].transcription_status).toBe('complete');

		poller.stop();
	});

	it('should retry on network errors', async () => {
		fetchMock
			.mockRejectedValueOnce(new Error('Network error'))
			.mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({ status: 'complete' }),
			});

		const videoList = createMockVideoList(['v1']);
		const videos = [makeVideo({ transcription_status: 'processing' })];

		const poller = startProgressPolling(videoList, videos, true);

		// First poll (network error)
		await flushPromises();
		expect(fetchMock).toHaveBeenCalledTimes(1);

		// Retry
		vi.advanceTimersByTime(3000);
		await flushPromises();
		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(videos[0].transcription_status).toBe('complete');

		poller.stop();
	});

	it('should poll multiple processing videos independently', async () => {
		fetchMock
			.mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({ status: 'complete' }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({ status: 'complete' }),
			});

		const videoList = createMockVideoList(['v1', 'v2']);
		const videos = [
			makeVideo({ id: 'v1', transcription_status: 'processing' }),
			makeVideo({ id: 'v2', transcription_status: 'processing' }),
		];

		const poller = startProgressPolling(videoList, videos, true);
		await flushPromises();

		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(fetchMock).toHaveBeenCalledWith('/api/transcribe/v1');
		expect(fetchMock).toHaveBeenCalledWith('/api/transcribe/v2');
		expect(videos[0].transcription_status).toBe('complete');
		expect(videos[1].transcription_status).toBe('complete');

		poller.stop();
	});

	it('should continue polling processing while stopping complete', async () => {
		fetchMock
			.mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({ status: 'processing', progress: 30 }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({ status: 'complete' }),
			})
			// Third call: v1 completes on second poll
			.mockResolvedValueOnce({
				ok: true,
				json: () => Promise.resolve({ status: 'complete' }),
			});

		const videoList = createMockVideoList(['v1', 'v2']);
		const videos = [
			makeVideo({ id: 'v1', transcription_status: 'processing' }),
			makeVideo({ id: 'v2', transcription_status: 'processing' }),
		];

		const poller = startProgressPolling(videoList, videos, true);

		// First round: both poll; v1=processing, v2=complete
		await flushPromises();
		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(videos[1].transcription_status).toBe('complete');

		// Second round: only v1 polls
		vi.advanceTimersByTime(3000);
		await flushPromises();
		expect(fetchMock).toHaveBeenCalledTimes(3);
		expect(videos[0].transcription_status).toBe('complete');

		poller.stop();
	});
});
