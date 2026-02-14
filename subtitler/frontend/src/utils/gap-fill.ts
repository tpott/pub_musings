/**
 * Gap filling for subtitle segments.
 * Creates empty segments for detected gaps and triggers async transcription.
 */

import { csrfFetch } from './csrf';
import type { TranscriptionSegment } from '../types/transcription';

/** Result from gap transcription API */
export interface GapTranscriptionResult {
	text: string;
	segments: TranscriptionSegment[];
}

/** Create a new empty segment spanning a gap */
export function createGapSegment(start: number, end: number): TranscriptionSegment {
	return { id: 0, start, end, text: '' };
}

/**
 * Insert a gap segment into the segments array at the correct position.
 * afterIndex is the index of the segment after which to insert (-1 for before first).
 * Returns a new array with renumbered IDs.
 */
export function insertGapSegment(
	segments: TranscriptionSegment[],
	gap: TranscriptionSegment,
	afterIndex: number
): TranscriptionSegment[] {
	const result = [...segments];
	const insertAt = afterIndex + 1;
	result.splice(insertAt, 0, { ...gap });

	// Renumber IDs
	for (let i = 0; i < result.length; i++) {
		result[i] = { ...result[i], id: i };
	}
	return result;
}

/**
 * Call the gap transcription API to transcribe a time range.
 * Returns the transcribed text and segments with absolute timestamps.
 */
export async function transcribeGap(
	uploadId: string,
	start: number,
	end: number,
	language: string = ''
): Promise<GapTranscriptionResult> {
	const response = await csrfFetch(`/api/transcribe/${uploadId}/gap`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ start, end, language }),
	});

	const result = await response.json();
	if (!response.ok) {
		throw new Error(result.error || 'Gap transcription failed');
	}

	return result as GapTranscriptionResult;
}
