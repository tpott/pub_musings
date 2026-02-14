import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { TranscriptionSegment } from '../types/transcription';
import {
	createGapSegment,
	insertGapSegment,
	transcribeGap,
} from './gap-fill';

// Mock csrf module
vi.mock('./csrf', () => ({
	csrfFetch: vi.fn(),
}));

import { csrfFetch } from './csrf';
const mockCsrfFetch = vi.mocked(csrfFetch);

function seg(id: number, start: number, end: number, text: string): TranscriptionSegment {
	return { id, start, end, text };
}

describe('createGapSegment', () => {
	it('creates a segment spanning the gap', () => {
		const result = createGapSegment(2.0, 5.0);
		expect(result.start).toBe(2.0);
		expect(result.end).toBe(5.0);
		expect(result.text).toBe('');
		expect(result.id).toBe(0);
	});
});

describe('insertGapSegment', () => {
	it('inserts before first segment (afterIndex = -1)', () => {
		const segments = [seg(0, 2.0, 4.0, 'first')];
		const gap = createGapSegment(0, 2.0);
		const result = insertGapSegment(segments, gap, -1);

		expect(result).toHaveLength(2);
		expect(result[0].start).toBe(0);
		expect(result[0].end).toBe(2.0);
		expect(result[1].text).toBe('first');
		// IDs should be renumbered
		expect(result[0].id).toBe(0);
		expect(result[1].id).toBe(1);
	});

	it('inserts between segments', () => {
		const segments = [
			seg(0, 0.0, 2.0, 'first'),
			seg(1, 5.0, 8.0, 'second'),
		];
		const gap = createGapSegment(2.0, 5.0);
		const result = insertGapSegment(segments, gap, 0);

		expect(result).toHaveLength(3);
		expect(result[0].text).toBe('first');
		expect(result[1].start).toBe(2.0);
		expect(result[1].end).toBe(5.0);
		expect(result[1].text).toBe('');
		expect(result[2].text).toBe('second');
	});

	it('inserts after last segment', () => {
		const segments = [seg(0, 0.0, 4.0, 'only')];
		const gap = createGapSegment(4.0, 10.0);
		const result = insertGapSegment(segments, gap, 0);

		expect(result).toHaveLength(2);
		expect(result[0].text).toBe('only');
		expect(result[1].start).toBe(4.0);
		expect(result[1].end).toBe(10.0);
	});

	it('renumbers all segment IDs after insertion', () => {
		const segments = [
			seg(0, 0.0, 1.0, 'a'),
			seg(1, 3.0, 4.0, 'b'),
			seg(2, 6.0, 7.0, 'c'),
		];
		const gap = createGapSegment(1.0, 3.0);
		const result = insertGapSegment(segments, gap, 0);

		expect(result.map(s => s.id)).toEqual([0, 1, 2, 3]);
	});
});

describe('transcribeGap', () => {
	beforeEach(() => {
		mockCsrfFetch.mockReset();
	});

	it('calls the gap transcription API with correct params', async () => {
		mockCsrfFetch.mockResolvedValue({
			ok: true,
			json: () => Promise.resolve({
				text: 'transcribed text',
				segments: [{ id: 0, start: 2.1, end: 4.8, text: 'transcribed text' }],
			}),
		} as Response);

		const result = await transcribeGap('upload-123', 2.0, 5.0, 'en');

		expect(mockCsrfFetch).toHaveBeenCalledWith('/api/transcribe/upload-123/gap', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ start: 2.0, end: 5.0, language: 'en' }),
		});
		expect(result.text).toBe('transcribed text');
		expect(result.segments).toHaveLength(1);
	});

	it('throws on non-ok response', async () => {
		mockCsrfFetch.mockResolvedValue({
			ok: false,
			json: () => Promise.resolve({ error: 'Audio extraction failed' }),
		} as Response);

		await expect(transcribeGap('upload-123', 2.0, 5.0)).rejects.toThrow('Audio extraction failed');
	});

	it('handles network errors', async () => {
		mockCsrfFetch.mockRejectedValue(new Error('Network error'));

		await expect(transcribeGap('upload-123', 2.0, 5.0)).rejects.toThrow('Network error');
	});

	it('uses empty language when not provided', async () => {
		mockCsrfFetch.mockResolvedValue({
			ok: true,
			json: () => Promise.resolve({ text: 'result', segments: [] }),
		} as Response);

		await transcribeGap('upload-123', 0, 3);

		const body = JSON.parse(mockCsrfFetch.mock.calls[0][1]!.body as string);
		expect(body.language).toBe('');
	});
});
