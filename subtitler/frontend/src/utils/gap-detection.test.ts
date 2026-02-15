import { describe, it, expect } from 'vitest';
import type { TranscriptionSegment } from '../types/transcription';
import { detectGaps, type GapInfo } from './gap-detection';

function seg(id: number, start: number, end: number, text = ''): TranscriptionSegment {
	return { id, start, end, text };
}

describe('detectGaps', () => {
	it('returns empty array for empty segments', () => {
		expect(detectGaps([], 10)).toEqual([]);
	});

	it('finds gap before first segment', () => {
		const segments = [seg(0, 2.0, 4.0, 'hello')];
		const gaps = detectGaps(segments, 10);

		expect(gaps).toContainEqual({
			start: 0,
			end: 2.0,
			afterIndex: -1,
		});
	});

	it('does not find gap before first segment when start is small', () => {
		const segments = [seg(0, 0.3, 4.0, 'hello')];
		const gaps = detectGaps(segments, 10);

		const beforeGap = gaps.find(g => g.afterIndex === -1);
		expect(beforeGap).toBeUndefined();
	});

	it('finds gap between two segments', () => {
		const segments = [
			seg(0, 0.0, 2.0, 'first'),
			seg(1, 5.0, 8.0, 'second'),
		];
		const gaps = detectGaps(segments, 10);

		expect(gaps).toContainEqual({
			start: 2.0,
			end: 5.0,
			afterIndex: 0,
		});
	});

	it('finds gap after last segment', () => {
		const segments = [seg(0, 0.0, 4.0, 'hello')];
		const gaps = detectGaps(segments, 10);

		expect(gaps).toContainEqual({
			start: 4.0,
			end: 10,
			afterIndex: 0,
		});
	});

	it('does not find gap after last segment when close to duration', () => {
		const segments = [seg(0, 0.0, 9.8, 'hello')];
		const gaps = detectGaps(segments, 10);

		const afterGap = gaps.find(g => g.afterIndex === 0);
		expect(afterGap).toBeUndefined();
	});

	it('ignores gaps smaller than threshold', () => {
		const segments = [
			seg(0, 0.0, 2.0, 'first'),
			seg(1, 2.3, 4.0, 'second'),
		];
		const gaps = detectGaps(segments, 4.2);

		// Gap of 0.3s between segments — below default 0.5 threshold
		const betweenGap = gaps.find(g => g.afterIndex === 0);
		expect(betweenGap).toBeUndefined();
	});

	it('uses custom minGap threshold', () => {
		const segments = [
			seg(0, 0.0, 2.0, 'first'),
			seg(1, 2.3, 4.0, 'second'),
		];
		const gaps = detectGaps(segments, 4.0, 0.2);

		const betweenGap = gaps.find(g => g.afterIndex === 0);
		expect(betweenGap).toBeDefined();
		expect(betweenGap!.start).toBe(2.0);
		expect(betweenGap!.end).toBe(2.3);
	});

	it('returns correct afterIndex for insertion', () => {
		const segments = [
			seg(0, 0.0, 1.0, 'a'),
			seg(1, 3.0, 4.0, 'b'),
			seg(2, 6.0, 7.0, 'c'),
		];
		const gaps = detectGaps(segments, 10);

		// Gap between seg 0 and seg 1 → afterIndex = 0
		expect(gaps).toContainEqual({ start: 1.0, end: 3.0, afterIndex: 0 });
		// Gap between seg 1 and seg 2 → afterIndex = 1
		expect(gaps).toContainEqual({ start: 4.0, end: 6.0, afterIndex: 1 });
		// Gap after seg 2 → afterIndex = 2
		expect(gaps).toContainEqual({ start: 7.0, end: 10, afterIndex: 2 });
	});

	it('finds all three gap types simultaneously', () => {
		const segments = [
			seg(0, 2.0, 3.0, 'first'),
			seg(1, 6.0, 7.0, 'second'),
		];
		const gaps = detectGaps(segments, 10);

		expect(gaps).toHaveLength(3);
		expect(gaps[0]).toEqual({ start: 0, end: 2.0, afterIndex: -1 });
		expect(gaps[1]).toEqual({ start: 3.0, end: 6.0, afterIndex: 0 });
		expect(gaps[2]).toEqual({ start: 7.0, end: 10, afterIndex: 1 });
	});

	it('returns empty array when no gaps exist', () => {
		const segments = [
			seg(0, 0.0, 2.0, 'first'),
			seg(1, 2.0, 4.0, 'second'),
			seg(2, 4.0, 5.0, 'third'),
		];
		const gaps = detectGaps(segments, 5.0);

		expect(gaps).toHaveLength(0);
	});

	it('handles single segment covering entire duration', () => {
		const segments = [seg(0, 0.0, 10.0, 'all')];
		const gaps = detectGaps(segments, 10.0);

		expect(gaps).toHaveLength(0);
	});

	it('handles video duration of zero', () => {
		const gaps = detectGaps([], 0);
		expect(gaps).toHaveLength(0);
	});

	it('handles overlapping segments without false gaps', () => {
		const segments = [
			seg(0, 0.0, 3.0, 'first'),
			seg(1, 2.5, 5.0, 'second'),
		];
		const gaps = detectGaps(segments, 5.0);

		// Segments overlap, no gap between them
		const betweenGap = gaps.find(g => g.afterIndex === 0);
		expect(betweenGap).toBeUndefined();
	});
});
