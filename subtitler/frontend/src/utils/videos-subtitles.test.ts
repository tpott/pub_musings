import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { getSubtitleSegments, handleDownloadClick } from './videos-subtitles';

// --- Mocks ---

vi.mock('./subtitles', () => ({
	downloadSRT: vi.fn(),
	downloadVTT: vi.fn(),
	downloadJSON: vi.fn(),
}));

vi.mock('./dialog', () => ({
	showAlert: vi.fn(() => Promise.resolve()),
}));

vi.mock('./api-schemas', () => ({
	TranscriptionStatusResponseSchema: {},
	safeParse: vi.fn((_, data: Record<string, unknown>) => data),
}));

function makeSegments(count: number) {
	return Array.from({ length: count }, (_, i) => ({
		id: i,
		start: i * 2,
		end: i * 2 + 1.5,
		text: `Segment ${i}`,
	}));
}

beforeEach(() => {
	vi.clearAllMocks();
	// Reset the module-level cache by re-importing
	// Since we can't directly clear the cache, we use unique videoIds per test
});

afterEach(() => {
	vi.restoreAllMocks();
});

// --- Tests ---

describe('getSubtitleSegments', () => {
	it('should fetch segments from server', async () => {
		const segments = makeSegments(3);
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () =>
						Promise.resolve({
							status: 'complete',
							result: { segments },
						}),
				})
			)
		);

		const result = await getSubtitleSegments('fetch-test-1');

		expect(fetch).toHaveBeenCalledWith('/api/transcribe/fetch-test-1');
		expect(result).toEqual(segments);
	});

	it('should return cached segments on second call', async () => {
		const segments = makeSegments(2);
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () =>
						Promise.resolve({
							status: 'complete',
							result: { segments },
						}),
				})
			)
		);

		// First call - fetches
		await getSubtitleSegments('cache-test-1');
		expect(fetch).toHaveBeenCalledTimes(1);

		// Second call - cached
		const result = await getSubtitleSegments('cache-test-1');
		expect(fetch).toHaveBeenCalledTimes(1); // not called again
		expect(result).toEqual(segments);
	});

	it('should throw on non-ok response', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: false,
					json: () => Promise.resolve({ error: 'Not found' }),
				})
			)
		);

		await expect(getSubtitleSegments('error-test-1')).rejects.toThrow('Not found');
	});

	it('should throw on invalid JSON', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () => Promise.reject(new SyntaxError('Unexpected token')),
				})
			)
		);

		await expect(getSubtitleSegments('json-error-test-1')).rejects.toThrow(
			'Server returned invalid data'
		);
	});

	it('should re-throw non-SyntaxError from json parsing', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () => Promise.reject(new TypeError('something else')),
				})
			)
		);

		await expect(getSubtitleSegments('rethrow-test-1')).rejects.toThrow('something else');
	});

	it('should throw when safeParse returns null', async () => {
		const { safeParse } = await import('./api-schemas');
		(safeParse as ReturnType<typeof vi.fn>).mockReturnValueOnce(null);

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () => Promise.resolve({ status: 'complete', result: { segments: [] } }),
				})
			)
		);

		await expect(getSubtitleSegments('parse-fail-1')).rejects.toThrow(
			'Invalid transcription response format'
		);
	});

	it('should throw when status is not complete', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () => Promise.resolve({ status: 'processing' }),
				})
			)
		);

		await expect(getSubtitleSegments('not-complete-1')).rejects.toThrow(
			'Transcription not available'
		);
	});

	it('should throw when result is missing', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () => Promise.resolve({ status: 'complete', result: null }),
				})
			)
		);

		await expect(getSubtitleSegments('no-result-1')).rejects.toThrow(
			'Transcription not available'
		);
	});
});

describe('handleDownloadClick', () => {
	// Use unique videoIds per test to avoid module-level cache interference
	let testCounter = 0;

	function makeButton(overrides: Record<string, string> = {}): HTMLButtonElement {
		testCounter++;
		const dataset: Record<string, string | undefined> = {
			videoId: `dl-v${testCounter}`,
			filename: 'test.mp4',
			...overrides,
		};
		return {
			dataset,
			disabled: false,
			textContent: 'SRT',
			setAttribute: vi.fn(),
			removeAttribute: vi.fn(),
		} as unknown as HTMLButtonElement;
	}

	it('should download SRT format', async () => {
		const segments = makeSegments(2);
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () =>
						Promise.resolve({ status: 'complete', result: { segments } }),
				})
			)
		);

		const button = makeButton();
		await handleDownloadClick(button, 'srt');

		const { downloadSRT } = await import('./subtitles');
		expect(downloadSRT).toHaveBeenCalledWith(segments, 'test.mp4');
	});

	it('should download VTT format', async () => {
		const segments = makeSegments(2);
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () =>
						Promise.resolve({ status: 'complete', result: { segments } }),
				})
			)
		);

		const button = makeButton();
		await handleDownloadClick(button, 'vtt');

		const { downloadVTT } = await import('./subtitles');
		expect(downloadVTT).toHaveBeenCalledWith(segments, 'test.mp4');
	});

	it('should download JSON format', async () => {
		const segments = makeSegments(2);
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () =>
						Promise.resolve({ status: 'complete', result: { segments } }),
				})
			)
		);

		const button = makeButton();
		await handleDownloadClick(button, 'json');

		const { downloadJSON } = await import('./subtitles');
		expect(downloadJSON).toHaveBeenCalledWith(segments, 'test.mp4');
	});

	it('should do nothing if videoId is missing', async () => {
		const button = makeButton();
		delete button.dataset.videoId;
		await handleDownloadClick(button, 'srt');

		const { downloadSRT } = await import('./subtitles');
		expect(downloadSRT).not.toHaveBeenCalled();
	});

	it('should show info alert when no segments', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () =>
						Promise.resolve({ status: 'complete', result: { segments: [] } }),
				})
			)
		);

		const button = makeButton();
		await handleDownloadClick(button, 'srt');

		const { showAlert } = await import('./dialog');
		expect(showAlert).toHaveBeenCalledWith('No subtitles available', 'info');
	});

	it('should show error alert on fetch failure', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: false,
					json: () => Promise.resolve({ error: 'Server error' }),
				})
			)
		);

		const button = makeButton();
		await handleDownloadClick(button, 'srt');

		const { showAlert } = await import('./dialog');
		expect(showAlert).toHaveBeenCalledWith(
			expect.stringContaining('Server error'),
			'error'
		);
	});

	it('should restore button state after download', async () => {
		const segments = makeSegments(1);
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () =>
						Promise.resolve({ status: 'complete', result: { segments } }),
				})
			)
		);

		const button = makeButton();
		button.textContent = 'SRT';
		await handleDownloadClick(button, 'srt');

		expect(button.disabled).toBe(false);
		expect(button.removeAttribute).toHaveBeenCalledWith('aria-busy');
		expect(button.textContent).toBe('SRT');
	});

	it('should restore button state after error', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(() => Promise.reject(new Error('Network error')))
		);

		const button = makeButton();
		button.textContent = 'VTT';
		await handleDownloadClick(button, 'vtt');

		expect(button.disabled).toBe(false);
		expect(button.textContent).toBe('VTT');
	});

	it('should use videoId as fallback filename', async () => {
		const segments = makeSegments(1);
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					json: () =>
						Promise.resolve({ status: 'complete', result: { segments } }),
				})
			)
		);

		const button = makeButton();
		const videoId = button.dataset.videoId!;
		delete button.dataset.filename;
		await handleDownloadClick(button, 'srt');

		const { downloadSRT } = await import('./subtitles');
		expect(downloadSRT).toHaveBeenCalledWith(segments, videoId);
	});
});
