import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	createTranscriptionState,
	formatTime,
	formatDuration,
	parseSRT,
	getLanguageDisplayName,
	startTranscription,
	pollTranscriptionStatus,
	type TranscriptionCallbacks,
	type TranscriptionState,
} from './transcription-polling';

// --- Mocks ---

vi.mock('./csrf', () => ({
	csrfFetch: vi.fn(() => Promise.resolve({ ok: true })),
}));

vi.mock('./processing-speed', () => ({
	estimateTimeRemaining: vi.fn(() => 30),
	formatTimeRemaining: vi.fn(() => 'About 30 seconds remaining'),
	hasHistoricalData: vi.fn(() => false),
	recordProcessingTime: vi.fn(),
}));

vi.mock('./upload-constants', () => ({
	LANGUAGE_DISPLAY_NAMES: {
		auto: 'Auto-detect',
		en: 'English',
		es: 'Spanish',
		fr: 'French',
	},
}));

function makeCallbacks(overrides: Partial<TranscriptionCallbacks> = {}): TranscriptionCallbacks {
	return {
		setTranscriptionStatus: vi.fn(),
		displayTranscription: vi.fn(),
		...overrides,
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

describe('createTranscriptionState', () => {
	it('should return default state', () => {
		const state = createTranscriptionState();
		expect(state.transcriptionPollTimeout).toBeNull();
		expect(state.fetchAbortController).toBeNull();
		expect(state.currentVideoDuration).toBe(0);
		expect(state.transcriptionStartTime).toBeNull();
		expect(state.embeddedSubtitleTracks).toEqual([]);
	});

	it('should return independent state objects', () => {
		const state1 = createTranscriptionState();
		const state2 = createTranscriptionState();
		state1.currentVideoDuration = 120;
		expect(state2.currentVideoDuration).toBe(0);
	});
});

describe('formatTime', () => {
	it('should format zero seconds', () => {
		expect(formatTime(0)).toBe('0:00.000');
	});

	it('should format seconds with milliseconds', () => {
		expect(formatTime(5)).toBe('0:05.000');
	});

	it('should format minutes and seconds', () => {
		expect(formatTime(65)).toBe('1:05.000');
	});

	it('should format with hours', () => {
		expect(formatTime(3661)).toBe('1:01:01.000');
	});

	it('should format fractional seconds', () => {
		expect(formatTime(1.5)).toBe('0:01.500');
	});

	it('should format sub-second values', () => {
		expect(formatTime(0.123)).toBe('0:00.123');
	});

	it('should pad minutes/seconds when hours present', () => {
		expect(formatTime(3600 + 5 * 60 + 3)).toBe('1:05:03.000');
	});

	it('should pad milliseconds', () => {
		expect(formatTime(0.01)).toBe('0:00.010');
	});

	it('should handle large values', () => {
		// Note: 45.678 in IEEE 754 is 45.67799..., so Math.floor gives 677
		expect(formatTime(7200 + 30 * 60 + 45.678)).toBe('2:30:45.677');
	});
});

describe('formatDuration', () => {
	it('should format zero seconds', () => {
		expect(formatDuration(0)).toBe('0s');
	});

	it('should format seconds only', () => {
		expect(formatDuration(30)).toBe('30s');
	});

	it('should format minutes and seconds', () => {
		expect(formatDuration(90)).toBe('1m 30s');
	});

	it('should floor fractional seconds', () => {
		expect(formatDuration(5.7)).toBe('5s');
	});

	it('should clamp negative values to 0', () => {
		expect(formatDuration(-5)).toBe('0s');
	});

	it('should handle exact minutes', () => {
		expect(formatDuration(120)).toBe('2m 0s');
	});
});

describe('parseSRT', () => {
	it('should parse a valid SRT block', () => {
		const srt = `1
00:00:01,000 --> 00:00:04,000
Hello world

2
00:00:05,000 --> 00:00:08,500
Second subtitle`;

		const segments = parseSRT(srt);
		expect(segments).toHaveLength(2);
		expect(segments[0]).toEqual({ id: 0, start: 1, end: 4, text: 'Hello world' });
		expect(segments[1]).toEqual({ id: 1, start: 5, end: 8.5, text: 'Second subtitle' });
	});

	it('should handle multiline subtitle text', () => {
		const srt = `1
00:00:01,000 --> 00:00:04,000
Line one
Line two`;

		const segments = parseSRT(srt);
		expect(segments).toHaveLength(1);
		expect(segments[0].text).toBe('Line one\nLine two');
	});

	it('should handle empty input', () => {
		expect(parseSRT('')).toEqual([]);
	});

	it('should skip blocks with fewer than 3 lines', () => {
		const srt = `1
00:00:01,000 --> 00:00:04,000`;

		expect(parseSRT(srt)).toEqual([]);
	});

	it('should skip blocks with invalid time format', () => {
		const srt = `1
invalid time line
Hello world`;

		expect(parseSRT(srt)).toEqual([]);
	});

	it('should handle hours in timestamps', () => {
		const srt = `1
01:30:00,500 --> 02:00:30,000
Long video subtitle`;

		const segments = parseSRT(srt);
		expect(segments[0].start).toBe(5400.5); // 1*3600 + 30*60 + 0 + 0.5
		expect(segments[0].end).toBe(7230); // 2*3600 + 0*60 + 30 + 0
	});

	it('should handle multiple blank lines between blocks', () => {
		const srt = `1
00:00:01,000 --> 00:00:02,000
First


2
00:00:03,000 --> 00:00:04,000
Second`;

		const segments = parseSRT(srt);
		expect(segments).toHaveLength(2);
	});

	it('should assign sequential IDs starting from 0', () => {
		const srt = `1
00:00:00,000 --> 00:00:01,000
A

2
00:00:01,000 --> 00:00:02,000
B

3
00:00:02,000 --> 00:00:03,000
C`;

		const segments = parseSRT(srt);
		expect(segments.map((s) => s.id)).toEqual([0, 1, 2]);
	});
});

describe('getLanguageDisplayName', () => {
	it('should return display name for known language', () => {
		expect(getLanguageDisplayName('en')).toBe('English');
	});

	it('should return display name for another known language', () => {
		expect(getLanguageDisplayName('fr')).toBe('French');
	});

	it('should return fallback name when code not in map', () => {
		expect(getLanguageDisplayName('xx', 'Unknown')).toBe('Unknown');
	});

	it('should return uppercase code when no fallback provided', () => {
		expect(getLanguageDisplayName('xx')).toBe('XX');
	});

	it('should prefer map entry over fallback', () => {
		expect(getLanguageDisplayName('en', 'Fallback')).toBe('English');
	});
});

describe('startTranscription', () => {
	it('should set status and call csrfFetch', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		// Mock fetch for the poll call
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () => Promise.resolve({ status: 'processing', message: 'Working...' }),
				})
			)
		);

		await startTranscription('upload-123', state, callbacks);

		expect(callbacks.setTranscriptionStatus).toHaveBeenCalledWith('Starting transcription...');

		const { csrfFetch } = await import('./csrf');
		expect(csrfFetch).toHaveBeenCalledWith('/api/transcribe/upload-123', { method: 'POST' });
	});

	it('should include language parameter when not auto', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () => Promise.resolve({ status: 'processing' }),
				})
			)
		);

		await startTranscription('upload-123', state, callbacks, 'es');

		const { csrfFetch } = await import('./csrf');
		expect(csrfFetch).toHaveBeenCalledWith('/api/transcribe/upload-123?language=es', {
			method: 'POST',
		});
	});

	it('should not include language parameter for auto', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () => Promise.resolve({ status: 'processing' }),
				})
			)
		);

		await startTranscription('upload-123', state, callbacks, 'auto');

		const { csrfFetch } = await import('./csrf');
		expect(csrfFetch).toHaveBeenCalledWith('/api/transcribe/upload-123', { method: 'POST' });
	});

	it('should include force parameter', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () => Promise.resolve({ status: 'processing' }),
				})
			)
		);

		await startTranscription('upload-123', state, callbacks, undefined, true);

		const { csrfFetch } = await import('./csrf');
		expect(csrfFetch).toHaveBeenCalledWith('/api/transcribe/upload-123?force=true', {
			method: 'POST',
		});
	});

	it('should include both language and force parameters', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () => Promise.resolve({ status: 'processing' }),
				})
			)
		);

		await startTranscription('upload-123', state, callbacks, 'fr', true);

		const { csrfFetch } = await import('./csrf');
		expect(csrfFetch).toHaveBeenCalledWith(
			'/api/transcribe/upload-123?language=fr&force=true',
			{ method: 'POST' }
		);
	});

	it('should handle csrfFetch error', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		const { csrfFetch } = await import('./csrf');
		(csrfFetch as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('Network error'));

		await startTranscription('upload-123', state, callbacks);

		expect(callbacks.setTranscriptionStatus).toHaveBeenCalledWith(
			'Failed to start transcription: Error: Network error'
		);
	});

	it('should record start time', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () => Promise.resolve({ status: 'processing' }),
				})
			)
		);

		vi.setSystemTime(new Date('2026-01-29T12:00:00Z'));

		await startTranscription('upload-123', state, callbacks);

		expect(state.transcriptionStartTime).toBe(Date.now());
	});
});

describe('pollTranscriptionStatus', () => {
	it('should show processing status', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () => Promise.resolve({ status: 'processing', message: 'Working...' }),
				})
			)
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		expect(callbacks.setTranscriptionStatus).toHaveBeenCalledWith(
			expect.stringContaining('Working...')
		);
	});

	it('should show progress percentage', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () =>
						Promise.resolve({ status: 'processing', message: 'Working', progress: 50 }),
				})
			)
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		expect(callbacks.setTranscriptionStatus).toHaveBeenCalledWith(
			expect.stringContaining('50%')
		);
	});

	it('should show ETA when duration and start time available', async () => {
		const state = createTranscriptionState();
		state.currentVideoDuration = 120;
		state.transcriptionStartTime = Date.now() - 10000;
		const callbacks = makeCallbacks();

		const { hasHistoricalData } = await import('./processing-speed');
		(hasHistoricalData as ReturnType<typeof vi.fn>).mockReturnValue(true);

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () =>
						Promise.resolve({ status: 'processing', message: 'Working', progress: 30 }),
				})
			)
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		expect(callbacks.setTranscriptionStatus).toHaveBeenCalledWith(
			expect.stringContaining('About 30 seconds remaining')
		);
	});

	it('should show Estimating when no historical data and no duration', async () => {
		const state = createTranscriptionState();
		state.currentVideoDuration = 0;
		state.transcriptionStartTime = null;
		const callbacks = makeCallbacks();

		const { hasHistoricalData } = await import('./processing-speed');
		(hasHistoricalData as ReturnType<typeof vi.fn>).mockReturnValue(false);

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () =>
						Promise.resolve({ status: 'processing', message: 'Working' }),
				})
			)
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		expect(callbacks.setTranscriptionStatus).toHaveBeenCalledWith(
			expect.stringContaining('Estimating...')
		);
	});

	it('should handle complete status', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();
		const mockResult = { segments: [{ id: 0, start: 0, end: 1, text: 'Hello' }] };

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () =>
						Promise.resolve({
							status: 'complete',
							result: mockResult,
							embedded_subtitles: [{ index: 0, language: 'en' }],
						}),
				})
			)
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		expect(callbacks.setTranscriptionStatus).toHaveBeenCalledWith('Transcription complete!');
		expect(callbacks.displayTranscription).toHaveBeenCalledWith(mockResult);
		expect(state.embeddedSubtitleTracks).toEqual([{ index: 0, language: 'en' }]);
	});

	it('should record processing time on completion', async () => {
		const state = createTranscriptionState();
		state.currentVideoDuration = 60;
		state.transcriptionStartTime = Date.now() - 5000;
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () =>
						Promise.resolve({ status: 'complete', result: { segments: [] } }),
				})
			)
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		const { recordProcessingTime } = await import('./processing-speed');
		expect(recordProcessingTime).toHaveBeenCalledWith(60, expect.any(Number));
		expect(state.transcriptionStartTime).toBeNull();
	});

	it('should handle error status', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () =>
						Promise.resolve({ status: 'error', message: 'Something went wrong' }),
				})
			)
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		expect(callbacks.setTranscriptionStatus).toHaveBeenCalledWith(
			'Error: Something went wrong'
		);
		expect(state.transcriptionPollTimeout).toBeNull();
	});

	it('should handle pending status and continue polling', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () => Promise.resolve({ status: 'pending' }),
				})
			)
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		expect(callbacks.setTranscriptionStatus).toHaveBeenCalledWith('Waiting to start...');
		expect(state.transcriptionPollTimeout).not.toBeNull();
	});

	it('should clear existing poll timeout before starting new poll', async () => {
		const state = createTranscriptionState();
		state.transcriptionPollTimeout = setTimeout(() => {}, 10000) as ReturnType<typeof setTimeout>;
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					json: () => Promise.resolve({ status: 'processing', message: 'Working' }),
				})
			)
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		// The function should have cleared the old timeout and set a new one
		expect(callbacks.setTranscriptionStatus).toHaveBeenCalled();
	});

	it('should silently ignore abort errors', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		const abortError = new Error('Aborted');
		abortError.name = 'AbortError';
		vi.stubGlobal(
			'fetch',
			vi.fn(() => Promise.reject(abortError))
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		// Should not set an error status for abort errors
		expect(callbacks.setTranscriptionStatus).not.toHaveBeenCalledWith(
			expect.stringContaining('Polling error')
		);
	});

	it('should show polling error for non-abort errors', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		vi.stubGlobal(
			'fetch',
			vi.fn(() => Promise.reject(new Error('Network failure')))
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		expect(callbacks.setTranscriptionStatus).toHaveBeenCalledWith(
			expect.stringContaining('Polling error')
		);
	});

	it('should set fetchAbortController', async () => {
		const state = createTranscriptionState();
		const callbacks = makeCallbacks();

		// We need to capture the state during the fetch call
		let controllerSet = false;
		vi.stubGlobal(
			'fetch',
			vi.fn(() => {
				controllerSet = state.fetchAbortController !== null;
				return Promise.resolve({
					json: () => Promise.resolve({ status: 'complete', result: { segments: [] } }),
				});
			})
		);

		pollTranscriptionStatus('upload-123', state, callbacks);
		await vi.advanceTimersByTimeAsync(0);

		expect(controllerSet).toBe(true);
	});
});
