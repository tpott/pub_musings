/**
 * Transcription polling and display logic for the upload page.
 */

import { csrfFetch } from './csrf';
import {
	estimateTimeRemaining,
	formatTimeRemaining,
	hasHistoricalData,
	recordProcessingTime
} from './processing-speed';
import type { TranscriptionResult, TranscriptionSegment } from '../types/transcription';
import { LANGUAGE_DISPLAY_NAMES } from './upload-constants';

/** Subtitle track information from embedded subtitles */
export interface SubtitleTrack {
	index: number;
	language: string;
	title: string;
	codec: string;
	default: boolean;
	forced: boolean;
	text_based: boolean;
}

/** Mutable state for transcription polling */
export interface TranscriptionState {
	transcriptionPollTimeout: ReturnType<typeof setTimeout> | null;
	fetchAbortController: AbortController | null;
	currentVideoDuration: number;
	transcriptionStartTime: number | null;
	embeddedSubtitleTracks: SubtitleTrack[];
}

export function createTranscriptionState(): TranscriptionState {
	return {
		transcriptionPollTimeout: null,
		fetchAbortController: null,
		currentVideoDuration: 0,
		transcriptionStartTime: null,
		embeddedSubtitleTracks: []
	};
}

/** Callbacks for transcription polling to interact with the page */
export interface TranscriptionCallbacks {
	setTranscriptionStatus: (text: string) => void;
	displayTranscription: (result: TranscriptionResult) => void;
}

/** Start transcription for an upload */
export async function startTranscription(
	uploadId: string,
	state: TranscriptionState,
	callbacks: TranscriptionCallbacks,
	language?: string,
	force?: boolean
): Promise<void> {
	callbacks.setTranscriptionStatus('Starting transcription...');

	// Record start time for ETA calculation
	state.transcriptionStartTime = Date.now();

	try {
		let transcribeUrl = `/api/transcribe/${uploadId}`;
		const params: string[] = [];
		if (language && language !== 'auto') {
			params.push(`language=${encodeURIComponent(language)}`);
		}
		if (force) {
			params.push('force=true');
		}
		if (params.length > 0) {
			transcribeUrl += '?' + params.join('&');
		}

		await csrfFetch(transcribeUrl, { method: 'POST' });

		// Start polling for status
		pollTranscriptionStatus(uploadId, state, callbacks);
	} catch (error) {
		callbacks.setTranscriptionStatus(`Failed to start transcription: ${error}`);
	}
}

/** Poll for transcription status updates */
export function pollTranscriptionStatus(
	uploadId: string,
	state: TranscriptionState,
	callbacks: TranscriptionCallbacks
): void {
	// Clear any existing poll timeout
	if (state.transcriptionPollTimeout !== null) {
		clearTimeout(state.transcriptionPollTimeout);
		state.transcriptionPollTimeout = null;
	}

	const poll = async () => {
		try {
			state.fetchAbortController = new AbortController();
			const response = await fetch(`/api/transcribe/${uploadId}`, {
				signal: state.fetchAbortController.signal
			});
			const result = await response.json();

			if (result.status === 'processing') {
				let statusMsg = result.message || 'Processing...';
				if (result.progress) {
					statusMsg += ` (${result.progress}%)`;
				}

				if (state.currentVideoDuration > 0 && state.transcriptionStartTime !== null) {
					const elapsedSeconds = (Date.now() - state.transcriptionStartTime) / 1000;
					const progress = result.progress || 0;
					const remaining = estimateTimeRemaining(state.currentVideoDuration, progress, elapsedSeconds);
					const etaText = formatTimeRemaining(remaining);
					statusMsg += ` - ${etaText}`;
				} else if (!hasHistoricalData()) {
					statusMsg += ' - Estimating...';
				}

				callbacks.setTranscriptionStatus(statusMsg);
				state.transcriptionPollTimeout = setTimeout(poll, 2000);
			} else if (result.status === 'complete') {
				state.transcriptionPollTimeout = null;

				if (state.currentVideoDuration > 0 && state.transcriptionStartTime !== null) {
					const processingTime = (Date.now() - state.transcriptionStartTime) / 1000;
					recordProcessingTime(state.currentVideoDuration, processingTime);
				}
				state.transcriptionStartTime = null;

				// Store embedded subtitle tracks if present
				state.embeddedSubtitleTracks = result.embedded_subtitles || [];

				callbacks.setTranscriptionStatus('Transcription complete!');
				callbacks.displayTranscription(result.result);
			} else if (result.status === 'error') {
				state.transcriptionPollTimeout = null;
				callbacks.setTranscriptionStatus(`Error: ${result.message}`);
			} else if (result.status === 'pending') {
				callbacks.setTranscriptionStatus('Waiting to start...');
				state.transcriptionPollTimeout = setTimeout(poll, 2000);
			}
		} catch (error) {
			if (error instanceof Error && error.name === 'AbortError') {
				return;
			}
			state.transcriptionPollTimeout = null;
			callbacks.setTranscriptionStatus(`Polling error: ${error}`);
		}
	};

	poll();
}

/** Format seconds into timestamp (H:MM:SS.mmm or M:SS.mmm) */
export function formatTime(seconds: number): string {
	const hrs = Math.floor(seconds / 3600);
	const mins = Math.floor((seconds % 3600) / 60);
	const secs = Math.floor(seconds % 60);
	const ms = Math.floor((seconds % 1) * 1000);

	if (hrs > 0) {
		return `${hrs}:${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}.${ms.toString().padStart(3, '0')}`;
	}
	return `${mins}:${secs.toString().padStart(2, '0')}.${ms.toString().padStart(3, '0')}`;
}

/** Format duration for human-readable display */
export function formatDuration(seconds: number): string {
	if (seconds < 0) seconds = 0;
	const mins = Math.floor(seconds / 60);
	const secs = Math.floor(seconds % 60);
	if (mins > 0) {
		return `${mins}m ${secs}s`;
	}
	return `${secs}s`;
}

/** Parse SRT content into TranscriptionSegment array */
export function parseSRT(srtContent: string): TranscriptionSegment[] {
	const segments: TranscriptionSegment[] = [];
	const blocks = srtContent.trim().split(/\n\n+/);

	for (const block of blocks) {
		const lines = block.split('\n');
		if (lines.length < 3) continue;

		const timeLine = lines[1];
		const timeMatch = timeLine.match(/(\d{2}):(\d{2}):(\d{2}),(\d{3})\s*-->\s*(\d{2}):(\d{2}):(\d{2}),(\d{3})/);
		if (!timeMatch) continue;

		const startHrs = parseInt(timeMatch[1], 10);
		const startMins = parseInt(timeMatch[2], 10);
		const startSecs = parseInt(timeMatch[3], 10);
		const startMs = parseInt(timeMatch[4], 10);
		const start = startHrs * 3600 + startMins * 60 + startSecs + startMs / 1000;

		const endHrs = parseInt(timeMatch[5], 10);
		const endMins = parseInt(timeMatch[6], 10);
		const endSecs = parseInt(timeMatch[7], 10);
		const endMs = parseInt(timeMatch[8], 10);
		const end = endHrs * 3600 + endMins * 60 + endSecs + endMs / 1000;

		const text = lines.slice(2).join('\n').trim();

		segments.push({ id: segments.length, start, end, text });
	}

	return segments;
}

/** Get display name for a language code */
export function getLanguageDisplayName(code: string, fallbackName?: string): string {
	return LANGUAGE_DISPLAY_NAMES[code] || fallbackName || code.toUpperCase();
}
