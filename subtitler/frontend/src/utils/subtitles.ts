/**
 * Subtitle generation utilities for client-side SRT/VTT/JSON file creation
 *
 * These functions generate subtitle files entirely in the browser from
 * loaded segment data, avoiding unnecessary server round-trips.
 */

import type { TranscriptionSegment } from '../types/transcription';

/**
 * Format seconds as SRT timestamp (HH:MM:SS,mmm)
 * Note: SRT uses comma as decimal separator
 */
export function formatSRTTimestamp(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = Math.floor(seconds % 60);
  const millis = Math.round((seconds - Math.floor(seconds)) * 1000);

  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')},${String(millis).padStart(3, '0')}`;
}

/**
 * Format seconds as VTT timestamp (HH:MM:SS.mmm)
 * Note: VTT uses period as decimal separator
 */
export function formatVTTTimestamp(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = Math.floor(seconds % 60);
  const millis = Math.round((seconds - Math.floor(seconds)) * 1000);

  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}.${String(millis).padStart(3, '0')}`;
}

/**
 * Generate SRT subtitle content from segments
 */
export function generateSRT(segments: TranscriptionSegment[]): string {
  const lines: string[] = [];

  for (let i = 0; i < segments.length; i++) {
    const segment = segments[i];
    // SRT sequence numbers are 1-indexed
    lines.push(String(i + 1));
    lines.push(`${formatSRTTimestamp(segment.start)} --> ${formatSRTTimestamp(segment.end)}`);
    lines.push(segment.text.trim());
    lines.push(''); // Blank line between entries
  }

  return lines.join('\n');
}

/**
 * Generate WebVTT subtitle content from segments
 */
export function generateVTT(segments: TranscriptionSegment[]): string {
  const lines: string[] = ['WEBVTT'];

  if (segments.length === 0) {
    lines.push('');
    return lines.join('\n');
  }

  lines.push('');

  for (let i = 0; i < segments.length; i++) {
    const segment = segments[i];
    // VTT cue identifiers are optional but helpful
    lines.push(String(i + 1));
    lines.push(`${formatVTTTimestamp(segment.start)} --> ${formatVTTTimestamp(segment.end)}`);
    lines.push(segment.text.trim());
    lines.push(''); // Blank line between entries
  }

  return lines.join('\n');
}

/**
 * Generate JSON subtitle content from segments
 */
export function generateJSON(segments: TranscriptionSegment[]): string {
  const output = {
    segments: segments.map(s => ({
      id: s.id,
      start: s.start,
      end: s.end,
      text: s.text
    }))
  };
  return JSON.stringify(output, null, 2);
}

/**
 * Create a downloadable blob URL from content
 * Returns the URL and a cleanup function
 */
export function createDownloadURL(content: string, mimeType: string): { url: string; revoke: () => void } {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  return {
    url,
    revoke: () => URL.revokeObjectURL(url)
  };
}

/**
 * Trigger a file download in the browser
 */
export function triggerDownload(content: string, filename: string, mimeType: string): void {
  const { url, revoke } = createDownloadURL(content, mimeType);

  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.style.display = 'none';

  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);

  // Clean up the URL after a short delay to ensure download starts
  setTimeout(revoke, 100);
}

/**
 * Download segments as SRT file
 */
export function downloadSRT(segments: TranscriptionSegment[], baseFilename: string): void {
  const content = generateSRT(segments);
  triggerDownload(content, `${baseFilename}.srt`, 'text/plain; charset=utf-8');
}

/**
 * Download segments as VTT file
 */
export function downloadVTT(segments: TranscriptionSegment[], baseFilename: string): void {
  const content = generateVTT(segments);
  triggerDownload(content, `${baseFilename}.vtt`, 'text/vtt; charset=utf-8');
}

/**
 * Download segments as JSON file
 */
export function downloadJSON(segments: TranscriptionSegment[], baseFilename: string): void {
  const content = generateJSON(segments);
  triggerDownload(content, `${baseFilename}.json`, 'application/json; charset=utf-8');
}
