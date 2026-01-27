import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  formatSRTTimestamp,
  formatVTTTimestamp,
  generateSRT,
  generateVTT,
  generateJSON,
  createDownloadURL,
} from './subtitles';
import type { TranscriptionSegment } from '../types/transcription';

// Note: triggerDownload, downloadSRT, downloadVTT, downloadJSON tests
// are skipped in unit tests as they require browser DOM.
// These functions are tested via E2E tests instead.

describe('Subtitle utilities', () => {
  describe('formatSRTTimestamp', () => {
    it('formats zero correctly', () => {
      expect(formatSRTTimestamp(0)).toBe('00:00:00,000');
    });

    it('formats milliseconds correctly', () => {
      expect(formatSRTTimestamp(0.5)).toBe('00:00:00,500');
      expect(formatSRTTimestamp(0.123)).toBe('00:00:00,123');
      expect(formatSRTTimestamp(0.999)).toBe('00:00:00,999');
    });

    it('formats seconds correctly', () => {
      expect(formatSRTTimestamp(5)).toBe('00:00:05,000');
      expect(formatSRTTimestamp(59)).toBe('00:00:59,000');
    });

    it('formats minutes correctly', () => {
      expect(formatSRTTimestamp(60)).toBe('00:01:00,000');
      expect(formatSRTTimestamp(125)).toBe('00:02:05,000');
    });

    it('formats hours correctly', () => {
      expect(formatSRTTimestamp(3600)).toBe('01:00:00,000');
      expect(formatSRTTimestamp(7325.456)).toBe('02:02:05,456');
    });

    it('uses comma as decimal separator (SRT standard)', () => {
      const result = formatSRTTimestamp(1.5);
      expect(result).toContain(',');
      expect(result).not.toContain('.');
    });
  });

  describe('formatVTTTimestamp', () => {
    it('formats zero correctly', () => {
      expect(formatVTTTimestamp(0)).toBe('00:00:00.000');
    });

    it('formats milliseconds correctly', () => {
      expect(formatVTTTimestamp(0.5)).toBe('00:00:00.500');
      expect(formatVTTTimestamp(0.123)).toBe('00:00:00.123');
    });

    it('formats hours correctly', () => {
      expect(formatVTTTimestamp(7325.456)).toBe('02:02:05.456');
    });

    it('uses period as decimal separator (VTT standard)', () => {
      const result = formatVTTTimestamp(1.5);
      expect(result).toContain('.');
      expect(result.match(/\./g)?.length).toBe(1);
    });
  });

  describe('generateSRT', () => {
    const testSegments: TranscriptionSegment[] = [
      { id: 0, start: 0, end: 2.5, text: 'Hello world' },
      { id: 1, start: 2.5, end: 5, text: 'This is a test' },
    ];

    it('generates valid SRT format', () => {
      const srt = generateSRT(testSegments);
      const lines = srt.split('\n');

      // First entry
      expect(lines[0]).toBe('1'); // 1-indexed sequence number
      expect(lines[1]).toBe('00:00:00,000 --> 00:00:02,500');
      expect(lines[2]).toBe('Hello world');
      expect(lines[3]).toBe('');

      // Second entry
      expect(lines[4]).toBe('2');
      expect(lines[5]).toBe('00:00:02,500 --> 00:00:05,000');
      expect(lines[6]).toBe('This is a test');
    });

    it('trims whitespace from segment text', () => {
      const segments: TranscriptionSegment[] = [
        { id: 0, start: 0, end: 1, text: '  spaced text  ' },
      ];
      const srt = generateSRT(segments);
      expect(srt).toContain('spaced text');
      expect(srt).not.toContain('  spaced text  ');
    });

    it('handles empty segments array', () => {
      const srt = generateSRT([]);
      expect(srt).toBe('');
    });

    it('handles single segment', () => {
      const segments: TranscriptionSegment[] = [
        { id: 0, start: 0, end: 3.5, text: 'Single line' },
      ];
      const srt = generateSRT(segments);
      expect(srt).toContain('1\n');
      expect(srt).toContain('00:00:00,000 --> 00:00:03,500');
      expect(srt).toContain('Single line');
    });
  });

  describe('generateVTT', () => {
    const testSegments: TranscriptionSegment[] = [
      { id: 0, start: 0, end: 2.5, text: 'Hello world' },
      { id: 1, start: 2.5, end: 5, text: 'This is a test' },
    ];

    it('starts with WEBVTT header', () => {
      const vtt = generateVTT(testSegments);
      expect(vtt.startsWith('WEBVTT\n')).toBe(true);
    });

    it('generates valid VTT format', () => {
      const vtt = generateVTT(testSegments);
      const lines = vtt.split('\n');

      expect(lines[0]).toBe('WEBVTT');
      expect(lines[1]).toBe('');

      // First entry
      expect(lines[2]).toBe('1');
      expect(lines[3]).toBe('00:00:00.000 --> 00:00:02.500');
      expect(lines[4]).toBe('Hello world');
    });

    it('uses period as decimal separator', () => {
      const vtt = generateVTT(testSegments);
      expect(vtt).toContain('00:00:00.000');
      expect(vtt).not.toContain('00:00:00,000');
    });

    it('handles empty segments array', () => {
      const vtt = generateVTT([]);
      // Empty VTT just has header and one blank line
      expect(vtt).toBe('WEBVTT\n');
    });
  });

  describe('generateJSON', () => {
    const testSegments: TranscriptionSegment[] = [
      { id: 0, start: 0, end: 2.5, text: 'Hello world' },
      { id: 1, start: 2.5, end: 5, text: 'This is a test' },
    ];

    it('generates valid JSON', () => {
      const json = generateJSON(testSegments);
      const parsed = JSON.parse(json);

      expect(parsed.segments).toBeInstanceOf(Array);
      expect(parsed.segments.length).toBe(2);
    });

    it('includes all segment properties', () => {
      const json = generateJSON(testSegments);
      const parsed = JSON.parse(json);

      expect(parsed.segments[0]).toEqual({
        id: 0,
        start: 0,
        end: 2.5,
        text: 'Hello world',
      });
    });

    it('handles empty segments array', () => {
      const json = generateJSON([]);
      const parsed = JSON.parse(json);
      expect(parsed.segments).toEqual([]);
    });

    it('produces pretty-printed JSON', () => {
      const json = generateJSON(testSegments);
      expect(json).toContain('\n');
      expect(json).toContain('  '); // 2-space indentation
    });
  });

  describe('createDownloadURL', () => {
    it('creates a blob URL', () => {
      const { url, revoke } = createDownloadURL('test content', 'text/plain');
      expect(url).toMatch(/^blob:/);
      revoke(); // Clean up
    });

    it('returns a revoke function', () => {
      const { url, revoke } = createDownloadURL('test content', 'text/plain');
      expect(typeof revoke).toBe('function');
      revoke();
    });
  });

  // Note: triggerDownload, downloadSRT, downloadVTT, downloadJSON tests are skipped
  // because they require browser DOM (document.createElement, document.body.appendChild).
  // These functions are tested via E2E tests in e2e/subtitle-download.spec.ts;

  describe('SRT/VTT format compatibility', () => {
    const segments: TranscriptionSegment[] = [
      { id: 0, start: 3661.234, end: 3665.789, text: 'Test at 1 hour' },
    ];

    it('SRT format matches backend output format', () => {
      const srt = generateSRT(segments);
      // Backend generates: "01:01:01,234 --> 01:01:05,789"
      expect(srt).toContain('01:01:01,234 --> 01:01:05,789');
    });

    it('VTT format matches backend output format', () => {
      const vtt = generateVTT(segments);
      // Backend generates: "01:01:01.234 --> 01:01:05.789"
      expect(vtt).toContain('01:01:01.234 --> 01:01:05.789');
    });
  });

  describe('Edge cases', () => {
    it('handles special characters in text', () => {
      const segments: TranscriptionSegment[] = [
        { id: 0, start: 0, end: 1, text: '< Hello > & "World"' },
      ];

      const srt = generateSRT(segments);
      expect(srt).toContain('< Hello > & "World"');

      const json = generateJSON(segments);
      const parsed = JSON.parse(json);
      expect(parsed.segments[0].text).toBe('< Hello > & "World"');
    });

    it('handles unicode characters', () => {
      const segments: TranscriptionSegment[] = [
        { id: 0, start: 0, end: 1, text: 'नमस्ते 你好 🎵' },
      ];

      const srt = generateSRT(segments);
      expect(srt).toContain('नमस्ते 你好 🎵');

      const json = generateJSON(segments);
      const parsed = JSON.parse(json);
      expect(parsed.segments[0].text).toBe('नमस्ते 你好 🎵');
    });

    it('handles very long timestamps (multi-day videos)', () => {
      const segments: TranscriptionSegment[] = [
        { id: 0, start: 100000, end: 100001, text: 'Long video' }, // ~27.7 hours
      ];

      const srt = generateSRT(segments);
      expect(srt).toContain('27:46:40,000');
    });

    it('handles fractional milliseconds by rounding', () => {
      const segments: TranscriptionSegment[] = [
        { id: 0, start: 1.1234, end: 1.5678, text: 'Precision' },
      ];

      const srt = generateSRT(segments);
      expect(srt).toContain('00:00:01,123'); // Rounded
      expect(srt).toContain('00:00:01,568');
    });
  });
});
