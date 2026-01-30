import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	formatBytes,
	formatDate,
	formatRetention,
	getStatusBadge,
	getEmbeddedSubtitlesBadge,
	renderVideo,
	reprocessVideo,
	deleteVideo,
	type Video,
} from './videos-list';

// --- Mocks ---

vi.mock('./csrf', () => ({
	csrfFetch: vi.fn(),
}));

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
		created_at: '2026-01-29T12:00:00Z',
		...overrides,
	};
}

beforeEach(() => {
	vi.clearAllMocks();
});

afterEach(() => {
	vi.restoreAllMocks();
});

// --- Tests ---

describe('formatBytes', () => {
	it('should format bytes', () => {
		expect(formatBytes(500)).toBe('500 B');
	});

	it('should format kilobytes', () => {
		expect(formatBytes(2048)).toBe('2.0 KB');
	});

	it('should format megabytes', () => {
		expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MB');
	});

	it('should handle zero bytes', () => {
		expect(formatBytes(0)).toBe('0 B');
	});

	it('should format fractional KB', () => {
		expect(formatBytes(1536)).toBe('1.5 KB');
	});

	it('should format fractional MB', () => {
		expect(formatBytes(1.5 * 1024 * 1024)).toBe('1.5 MB');
	});

	it('should show bytes below 1024', () => {
		expect(formatBytes(1023)).toBe('1023 B');
	});

	it('should show KB at exactly 1024', () => {
		expect(formatBytes(1024)).toBe('1.0 KB');
	});
});

describe('formatDate', () => {
	it('should format ISO date string', () => {
		const result = formatDate('2026-01-29T12:00:00Z');
		// The exact output depends on locale, but should contain date parts
		expect(result).toContain('2026');
		expect(result).toContain('Jan');
		expect(result).toContain('29');
	});

	it('should include time', () => {
		const result = formatDate('2026-06-15T15:30:00Z');
		expect(result).toContain('2026');
		expect(result).toContain('Jun');
	});
});

describe('formatRetention', () => {
	it('should return empty string when no expiration', () => {
		const video = makeVideo({ expires_at: null });
		expect(formatRetention(video)).toBe('');
	});

	it('should return empty string when expires_at is undefined', () => {
		const video = makeVideo({ expires_at: undefined });
		// expires_at is optional, so !video.expires_at is true
		expect(formatRetention(video)).toBe('');
	});

	it('should show Expired for past dates', () => {
		const video = makeVideo({ expires_at: '2020-01-01T00:00:00Z' });
		const result = formatRetention(video);
		expect(result).toContain('Expired');
		expect(result).toContain('retention-expired');
	});

	it('should show hours for anonymous users', () => {
		const futureDate = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString();
		const video = makeVideo({ expires_at: futureDate, user_id: undefined });
		const result = formatRetention(video);
		expect(result).toContain('h');
		expect(result).toContain('retention-info');
	});

	it('should show warning for anonymous users with less than 6h', () => {
		const futureDate = new Date(Date.now() + 3 * 60 * 60 * 1000).toISOString();
		const video = makeVideo({ expires_at: futureDate, user_id: undefined });
		const result = formatRetention(video);
		expect(result).toContain('retention-warning');
	});

	it('should show days for registered users', () => {
		const futureDate = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString();
		const video = makeVideo({ expires_at: futureDate, user_id: 'user-1' });
		const result = formatRetention(video);
		expect(result).toContain('d remaining');
		expect(result).toContain('retention-info');
	});

	it('should show warning for registered users with less than 7 days', () => {
		const futureDate = new Date(Date.now() + 3 * 24 * 60 * 60 * 1000).toISOString();
		const video = makeVideo({ expires_at: futureDate, user_id: 'user-1' });
		const result = formatRetention(video);
		expect(result).toContain('retention-warning');
		expect(result).toContain('d');
	});
});

describe('getStatusBadge', () => {
	it('should return Complete badge', () => {
		const result = getStatusBadge('complete');
		expect(result).toContain('Complete');
		expect(result).toContain('status-complete');
	});

	it('should return Processing badge', () => {
		const result = getStatusBadge('processing');
		expect(result).toContain('Processing');
		expect(result).toContain('status-processing');
	});

	it('should return Pending badge', () => {
		const result = getStatusBadge('pending');
		expect(result).toContain('Pending');
	});

	it('should return Error badge', () => {
		const result = getStatusBadge('error');
		expect(result).toContain('Error');
		expect(result).toContain('status-error');
	});

	it('should return Not Started for undefined status', () => {
		const result = getStatusBadge(undefined);
		expect(result).toContain('Not Started');
		expect(result).toContain('status-none');
	});

	it('should show raw status for unknown values', () => {
		const result = getStatusBadge('custom');
		expect(result).toContain('custom');
		expect(result).toContain('status-custom');
	});
});

describe('getEmbeddedSubtitlesBadge', () => {
	it('should return empty string when no embedded subtitles', () => {
		const video = makeVideo({ embedded_subtitles: [] });
		expect(getEmbeddedSubtitlesBadge(video)).toBe('');
	});

	it('should return empty string when embedded_subtitles is undefined', () => {
		const video = makeVideo({ embedded_subtitles: undefined });
		expect(getEmbeddedSubtitlesBadge(video)).toBe('');
	});

	it('should show language for single track', () => {
		const video = makeVideo({
			embedded_subtitles: [
				{ index: 0, language: 'en', codec: 'subrip', text_based: true },
			],
		});
		const result = getEmbeddedSubtitlesBadge(video);
		expect(result).toContain('EN');
		expect(result).toContain('embedded-subtitles-badge');
	});

	it('should show multiple languages', () => {
		const video = makeVideo({
			embedded_subtitles: [
				{ index: 0, language: 'en', codec: 'subrip', text_based: true },
				{ index: 1, language: 'fr', codec: 'subrip', text_based: true },
			],
		});
		const result = getEmbeddedSubtitlesBadge(video);
		expect(result).toContain('EN');
		expect(result).toContain('FR');
	});

	it('should use title as fallback when no language', () => {
		const video = makeVideo({
			embedded_subtitles: [
				{ index: 0, title: 'English', codec: 'subrip', text_based: true },
			],
		});
		const result = getEmbeddedSubtitlesBadge(video);
		expect(result).toContain('ENGLISH');
	});

	it('should show Unknown when no language or title', () => {
		const video = makeVideo({
			embedded_subtitles: [
				{ index: 0, codec: 'subrip', text_based: true },
				{ index: 1, codec: 'subrip', text_based: true },
			],
		});
		const result = getEmbeddedSubtitlesBadge(video);
		// Falls back to 'Unknown' for each track, deduplicated to single "UNKNOWN"
		expect(result).toContain('UNKNOWN');
	});

	it('should deduplicate languages', () => {
		const video = makeVideo({
			embedded_subtitles: [
				{ index: 0, language: 'en', codec: 'subrip', text_based: true },
				{ index: 1, language: 'en', codec: 'ass', text_based: true },
			],
		});
		const result = getEmbeddedSubtitlesBadge(video);
		// Should have EN only once, not "EN, EN"
		expect(result).toContain('EN');
		expect(result).not.toContain('EN, EN');
	});
});

describe('renderVideo', () => {
	it('should render video card with filename', () => {
		const video = makeVideo({ transcription_status: 'complete' });
		const html = renderVideo(video);
		expect(html).toContain('test.mp4');
		expect(html).toContain('video-card');
		expect(html).toContain('data-video-id="v1"');
	});

	it('should show View button for complete transcription', () => {
		const video = makeVideo({ transcription_status: 'complete' });
		const html = renderVideo(video);
		expect(html).toContain('btn-view');
		expect(html).toContain('View');
	});

	it('should show download buttons for complete transcription', () => {
		const video = makeVideo({ transcription_status: 'complete' });
		const html = renderVideo(video);
		expect(html).toContain('btn-download-srt');
		expect(html).toContain('btn-download-vtt');
		expect(html).toContain('btn-download-json');
	});

	it('should show Retry button for error status', () => {
		const video = makeVideo({ transcription_status: 'error' });
		const html = renderVideo(video);
		expect(html).toContain('btn-retry');
		expect(html).toContain('Retry');
	});

	it('should show Transcribe link for pending status', () => {
		const video = makeVideo({ transcription_status: 'pending' });
		const html = renderVideo(video);
		expect(html).toContain('Transcribe');
		expect(html).toContain('/upload?id=v1');
	});

	it('should always show Delete button', () => {
		const video = makeVideo({ transcription_status: 'processing' });
		const html = renderVideo(video);
		expect(html).toContain('btn-delete');
		expect(html).toContain('Delete');
	});

	it('should show thumbnail when available', () => {
		const video = makeVideo({ thumbnail_path: '/path/to/thumb.jpg', transcription_status: 'complete' });
		const html = renderVideo(video);
		expect(html).toContain('video-thumbnail');
		expect(html).toContain(`/api/videos/${video.id}/thumbnail`);
	});

	it('should show placeholder when no thumbnail', () => {
		const video = makeVideo({ thumbnail_path: '', transcription_status: 'complete' });
		const html = renderVideo(video);
		expect(html).toContain('video-thumbnail-placeholder');
	});

	it('should not show View/download buttons for processing status', () => {
		const video = makeVideo({ transcription_status: 'processing' });
		const html = renderVideo(video);
		expect(html).not.toContain('btn-view');
		expect(html).not.toContain('btn-download-srt');
	});
});

describe('reprocessVideo', () => {
	it('should disable button and show retrying state', async () => {
		const { csrfFetch } = await import('./csrf');
		(csrfFetch as ReturnType<typeof vi.fn>).mockResolvedValue({
			ok: true,
			json: () => Promise.resolve({ message: 'ok' }),
		});

		const mockLocation = { href: '' };
		vi.stubGlobal('window', { location: mockLocation });

		const button = {
			disabled: false,
			textContent: 'Retry',
			setAttribute: vi.fn(),
			removeAttribute: vi.fn(),
		} as unknown as HTMLButtonElement;

		await reprocessVideo('v1', button, true);

		expect(button.setAttribute).toHaveBeenCalledWith('aria-busy', 'true');
		expect(mockLocation.href).toBe('/upload?id=v1');
	});

	it('should include session_id for anonymous users', async () => {
		const { csrfFetch } = await import('./csrf');
		(csrfFetch as ReturnType<typeof vi.fn>).mockResolvedValue({
			ok: true,
			json: () => Promise.resolve({ message: 'ok' }),
		});

		const mockLocation = { href: '' };
		vi.stubGlobal('window', { location: mockLocation });

		const button = {
			disabled: false,
			textContent: 'Retry',
			setAttribute: vi.fn(),
			removeAttribute: vi.fn(),
		} as unknown as HTMLButtonElement;

		await reprocessVideo('v1', button, false);

		expect(csrfFetch).toHaveBeenCalledWith(
			expect.stringContaining('session_id=session-abc-123'),
			{ method: 'POST' }
		);
	});

	it('should show error on failure response', async () => {
		const { csrfFetch } = await import('./csrf');
		(csrfFetch as ReturnType<typeof vi.fn>).mockResolvedValue({
			ok: false,
			json: () => Promise.resolve({ error: 'Something went wrong' }),
		});

		const button = {
			disabled: false,
			textContent: 'Retry',
			setAttribute: vi.fn(),
			removeAttribute: vi.fn(),
		} as unknown as HTMLButtonElement;

		await reprocessVideo('v1', button, true);

		const { showAlert } = await import('./dialog');
		expect(showAlert).toHaveBeenCalledWith(
			expect.stringContaining('Something went wrong'),
			'error'
		);
		expect(button.disabled).toBe(false);
		expect(button.textContent).toBe('Retry');
	});

	it('should show error on network failure', async () => {
		const { csrfFetch } = await import('./csrf');
		(csrfFetch as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

		const button = {
			disabled: false,
			textContent: 'Retry',
			setAttribute: vi.fn(),
			removeAttribute: vi.fn(),
		} as unknown as HTMLButtonElement;

		await reprocessVideo('v1', button, true);

		const { showAlert } = await import('./dialog');
		expect(showAlert).toHaveBeenCalledWith(
			expect.stringContaining('Network error'),
			'error'
		);
	});
});

describe('deleteVideo', () => {
	it('should not proceed if user cancels confirm', async () => {
		const { showConfirm } = await import('./dialog');
		(showConfirm as ReturnType<typeof vi.fn>).mockResolvedValue(false);

		const button = { disabled: false, textContent: 'Delete', setAttribute: vi.fn() } as unknown as HTMLButtonElement;
		const videoList = { querySelectorAll: vi.fn() } as unknown as HTMLElement;

		await deleteVideo('v1', 'test.mp4', button, videoList, true);

		const { csrfFetch } = await import('./csrf');
		expect(csrfFetch).not.toHaveBeenCalled();
	});

	it('should delete and remove card from DOM', async () => {
		const { showConfirm } = await import('./dialog');
		(showConfirm as ReturnType<typeof vi.fn>).mockResolvedValue(true);

		const { csrfFetch } = await import('./csrf');
		(csrfFetch as ReturnType<typeof vi.fn>).mockResolvedValue({
			ok: true,
			json: () => Promise.resolve({ message: 'deleted' }),
		});

		const removeFn = vi.fn();
		const videoCard = { remove: removeFn };
		const button = {
			disabled: false,
			textContent: 'Delete',
			setAttribute: vi.fn(),
			removeAttribute: vi.fn(),
			closest: vi.fn(() => videoCard),
		} as unknown as HTMLButtonElement;
		const videoList = {
			querySelectorAll: vi.fn(() => [{}]), // 1 remaining
			innerHTML: '',
		} as unknown as HTMLElement;

		await deleteVideo('v1', 'test.mp4', button, videoList, true);

		expect(csrfFetch).toHaveBeenCalledWith('/api/videos/v1', { method: 'DELETE' });
		expect(removeFn).toHaveBeenCalled();
	});

	it('should show empty state when no videos remain', async () => {
		const { showConfirm } = await import('./dialog');
		(showConfirm as ReturnType<typeof vi.fn>).mockResolvedValue(true);

		const { csrfFetch } = await import('./csrf');
		(csrfFetch as ReturnType<typeof vi.fn>).mockResolvedValue({
			ok: true,
			json: () => Promise.resolve({ message: 'deleted' }),
		});

		const removeFn = vi.fn();
		const videoCard = { remove: removeFn };
		const button = {
			disabled: false,
			textContent: 'Delete',
			setAttribute: vi.fn(),
			removeAttribute: vi.fn(),
			closest: vi.fn(() => videoCard),
		} as unknown as HTMLButtonElement;
		const videoList = {
			querySelectorAll: vi.fn(() => []), // 0 remaining
			innerHTML: '',
		} as unknown as HTMLElement;

		await deleteVideo('v1', 'test.mp4', button, videoList, true);

		expect(videoList.innerHTML).toContain('No videos found');
		expect(videoList.innerHTML).toContain('Upload Your First Video');
	});

	it('should show error on failed delete', async () => {
		const { showConfirm } = await import('./dialog');
		(showConfirm as ReturnType<typeof vi.fn>).mockResolvedValue(true);

		const { csrfFetch } = await import('./csrf');
		(csrfFetch as ReturnType<typeof vi.fn>).mockResolvedValue({
			ok: false,
			json: () => Promise.resolve({ error: 'Not found' }),
		});

		const button = {
			disabled: false,
			textContent: 'Delete',
			setAttribute: vi.fn(),
			removeAttribute: vi.fn(),
			closest: vi.fn(() => null),
		} as unknown as HTMLButtonElement;
		const videoList = { querySelectorAll: vi.fn() } as unknown as HTMLElement;

		await deleteVideo('v1', 'test.mp4', button, videoList, true);

		const { showAlert } = await import('./dialog');
		expect(showAlert).toHaveBeenCalledWith(
			expect.stringContaining('Not found'),
			'error'
		);
		expect(button.disabled).toBe(false);
		expect(button.textContent).toBe('Delete');
	});

	it('should include session_id for anonymous delete', async () => {
		const { showConfirm } = await import('./dialog');
		(showConfirm as ReturnType<typeof vi.fn>).mockResolvedValue(true);

		const { csrfFetch } = await import('./csrf');
		(csrfFetch as ReturnType<typeof vi.fn>).mockResolvedValue({
			ok: true,
			json: () => Promise.resolve({ message: 'deleted' }),
		});

		const button = {
			disabled: false,
			textContent: 'Delete',
			setAttribute: vi.fn(),
			removeAttribute: vi.fn(),
			closest: vi.fn(() => ({ remove: vi.fn() })),
		} as unknown as HTMLButtonElement;
		const videoList = {
			querySelectorAll: vi.fn(() => [{}]),
			innerHTML: '',
		} as unknown as HTMLElement;

		await deleteVideo('v1', 'test.mp4', button, videoList, false);

		expect(csrfFetch).toHaveBeenCalledWith(
			expect.stringContaining('session_id=session-abc-123'),
			{ method: 'DELETE' }
		);
	});
});
