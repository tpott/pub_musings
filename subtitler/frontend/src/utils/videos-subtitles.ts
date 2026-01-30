/**
 * Subtitle caching and download helpers for the videos page.
 * LRU cache for transcription data and download handlers.
 */

import { downloadSRT, downloadVTT, downloadJSON } from './subtitles';
import { showAlert } from './dialog';
import { fetchWithTimeout } from './fetch-timeout';
import type { TranscriptionSegment } from '../types/transcription';
import { TranscriptionStatusResponseSchema, safeParse } from './api-schemas';

const CACHE_MAX_SIZE = 10;
const transcriptionCache: Map<string, TranscriptionSegment[]> = new Map();

function addToCache(videoId: string, segments: TranscriptionSegment[]) {
	if (transcriptionCache.size >= CACHE_MAX_SIZE) {
		const oldestKey = transcriptionCache.keys().next().value;
		if (oldestKey) {
			transcriptionCache.delete(oldestKey);
		}
	}
	transcriptionCache.set(videoId, segments);
}

export async function getSubtitleSegments(videoId: string): Promise<TranscriptionSegment[]> {
	// Check cache first
	if (transcriptionCache.has(videoId)) {
		// Move to end (most recently used) by re-inserting
		const cached = transcriptionCache.get(videoId)!;
		transcriptionCache.delete(videoId);
		transcriptionCache.set(videoId, cached);
		return cached;
	}

	// Fetch from server
	const response = await fetchWithTimeout(`/api/transcribe/${videoId}`);
	let rawData;
	try {
		rawData = await response.json();
	} catch (e) {
		if (e instanceof SyntaxError) {
			throw new Error('Server returned invalid data');
		}
		throw e;
	}

	if (!response.ok) {
		throw new Error(rawData.error || 'Failed to load transcription');
	}

	const data = safeParse(TranscriptionStatusResponseSchema, rawData);
	if (!data) {
		throw new Error('Invalid transcription response format');
	}

	if (data.status !== 'complete' || !data.result) {
		throw new Error('Transcription not available');
	}

	const segments = data.result.segments;
	addToCache(videoId, segments as TranscriptionSegment[]);
	return segments as TranscriptionSegment[];
}

export async function handleDownloadClick(
	button: HTMLButtonElement,
	format: 'srt' | 'vtt' | 'json'
): Promise<void> {
	const videoId = button.dataset.videoId;
	const filename = button.dataset.filename || videoId || 'subtitles';

	if (!videoId) return;

	button.disabled = true;
	button.setAttribute('aria-busy', 'true');
	const originalText = button.textContent;
	button.textContent = 'Loading...';

	try {
		const segments = await getSubtitleSegments(videoId);
		if (segments.length === 0) {
			await showAlert('No subtitles available', 'info');
			return;
		}

		if (format === 'srt') {
			downloadSRT(segments, filename);
		} else if (format === 'vtt') {
			downloadVTT(segments, filename);
		} else {
			downloadJSON(segments, filename);
		}
	} catch (err) {
		await showAlert(`Error: ${err instanceof Error ? err.message : 'Failed to download'}`, 'error');
	} finally {
		button.disabled = false;
		button.removeAttribute('aria-busy');
		button.textContent = originalText;
	}
}
