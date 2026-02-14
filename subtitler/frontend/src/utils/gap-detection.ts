/**
 * Gap detection for subtitle segments.
 * Identifies gaps in the timeline where subtitles may be missing.
 */

import type { TranscriptionSegment } from '../types/transcription';

/** Information about a detected gap between segments */
export interface GapInfo {
	start: number;     // gap start time in seconds
	end: number;       // gap end time in seconds
	afterIndex: number; // insert after this segment index (-1 for before first)
}

/**
 * Detect gaps in subtitle segment coverage.
 * Returns gaps larger than minGap seconds between:
 * - Video start (0) and first segment
 * - Adjacent segments
 * - Last segment and video end
 */
export function detectGaps(
	segments: TranscriptionSegment[],
	videoDuration: number,
	minGap: number = 0.5
): GapInfo[] {
	const gaps: GapInfo[] = [];

	if (segments.length === 0) return gaps;

	// Gap before first segment
	if (segments[0].start > minGap) {
		gaps.push({ start: 0, end: segments[0].start, afterIndex: -1 });
	}

	// Gaps between segments
	for (let i = 0; i < segments.length - 1; i++) {
		const gap = segments[i + 1].start - segments[i].end;
		if (gap > minGap) {
			gaps.push({
				start: segments[i].end,
				end: segments[i + 1].start,
				afterIndex: i,
			});
		}
	}

	// Gap after last segment
	const lastEnd = segments[segments.length - 1].end;
	if (videoDuration - lastEnd > minGap) {
		gaps.push({
			start: lastEnd,
			end: videoDuration,
			afterIndex: segments.length - 1,
		});
	}

	return gaps;
}
