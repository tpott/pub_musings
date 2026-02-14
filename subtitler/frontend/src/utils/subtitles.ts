/**
 * Subtitle generation utilities for client-side SRT/VTT/JSON file creation
 *
 * These functions generate subtitle files entirely in the browser from
 * loaded segment data, avoiding unnecessary server round-trips.
 */

import type { TranscriptionSegment, TranscriptionWord } from '../types/transcription';
import type { SubtitleLayer } from './subtitle-layers';
import { escapeHtml } from './html';

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
 * Flatten segments into individual word entries for word-level export.
 * Falls back to segment-level entries if a segment has no word data.
 */
function flattenToWords(segments: TranscriptionSegment[]): TranscriptionWord[] {
  const words: TranscriptionWord[] = [];
  for (const seg of segments) {
    if (seg.words && seg.words.length > 0) {
      words.push(...seg.words);
    } else {
      // Fallback: treat the whole segment as a single "word"
      words.push({ text: seg.text.trim(), start: seg.start, end: seg.end });
    }
  }
  return words;
}

/**
 * Generate SRT subtitle content from segments.
 * In word mode, each word becomes a separate subtitle entry.
 */
export function generateSRT(segments: TranscriptionSegment[], layer: SubtitleLayer = 'sentences'): string {
  const lines: string[] = [];

  if (layer === 'words') {
    const words = flattenToWords(segments);
    for (let i = 0; i < words.length; i++) {
      lines.push(String(i + 1));
      lines.push(`${formatSRTTimestamp(words[i].start)} --> ${formatSRTTimestamp(words[i].end)}`);
      lines.push(words[i].text.trim());
      lines.push('');
    }
  } else {
    for (let i = 0; i < segments.length; i++) {
      const segment = segments[i];
      lines.push(String(i + 1));
      lines.push(`${formatSRTTimestamp(segment.start)} --> ${formatSRTTimestamp(segment.end)}`);
      lines.push(segment.text.trim());
      lines.push('');
    }
  }

  return lines.join('\n');
}

/**
 * Generate WebVTT subtitle content from segments.
 * In word mode, each word becomes a separate subtitle entry.
 */
export function generateVTT(segments: TranscriptionSegment[], layer: SubtitleLayer = 'sentences'): string {
  const lines: string[] = ['WEBVTT'];

  if (segments.length === 0) {
    lines.push('');
    return lines.join('\n');
  }

  lines.push('');

  if (layer === 'words') {
    const words = flattenToWords(segments);
    for (let i = 0; i < words.length; i++) {
      lines.push(String(i + 1));
      lines.push(`${formatVTTTimestamp(words[i].start)} --> ${formatVTTTimestamp(words[i].end)}`);
      lines.push(words[i].text.trim());
      lines.push('');
    }
  } else {
    for (let i = 0; i < segments.length; i++) {
      const segment = segments[i];
      lines.push(String(i + 1));
      lines.push(`${formatVTTTimestamp(segment.start)} --> ${formatVTTTimestamp(segment.end)}`);
      lines.push(segment.text.trim());
      lines.push('');
    }
  }

  return lines.join('\n');
}

/**
 * Generate JSON subtitle content from segments.
 * Always includes word data when available.
 */
export function generateJSON(segments: TranscriptionSegment[]): string {
  const output = {
    segments: segments.map(s => {
      const entry: Record<string, unknown> = {
        id: s.id,
        start: s.start,
        end: s.end,
        text: s.text,
      };
      if (s.words && s.words.length > 0) {
        entry.words = s.words.map(w => ({
          text: w.text,
          start: w.start,
          end: w.end,
        }));
      }
      return entry;
    })
  };
  return JSON.stringify(output, null, 2);
}

/** Default auto-revoke timeout for blob URLs (5 minutes) */
const DEFAULT_AUTO_REVOKE_MS = 5 * 60 * 1000;

/**
 * Create a downloadable blob URL from content.
 * Returns the URL and a cleanup function.
 *
 * The URL will be automatically revoked after the specified timeout (default 5 min)
 * as a safety net to prevent memory leaks from orphaned blobs. Calling revoke()
 * manually is recommended for immediate cleanup.
 *
 * @param content The content to create a blob URL for
 * @param mimeType The MIME type for the blob
 * @param autoRevokeMs Auto-revoke timeout in ms (default 5 min, 0 to disable)
 */
export function createDownloadURL(
  content: string,
  mimeType: string,
  autoRevokeMs: number = DEFAULT_AUTO_REVOKE_MS
): { url: string; revoke: () => void } {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);

  let isRevoked = false;
  let timeoutId: ReturnType<typeof setTimeout> | undefined;

  // Safety net: auto-revoke after timeout if not manually revoked
  if (autoRevokeMs > 0) {
    timeoutId = setTimeout(() => {
      if (!isRevoked) {
        URL.revokeObjectURL(url);
        isRevoked = true;
      }
    }, autoRevokeMs);
  }

  return {
    url,
    revoke: () => {
      if (!isRevoked) {
        URL.revokeObjectURL(url);
        isRevoked = true;
        if (timeoutId !== undefined) {
          clearTimeout(timeoutId);
        }
      }
    }
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
export function downloadSRT(segments: TranscriptionSegment[], baseFilename: string, layer: SubtitleLayer = 'sentences'): void {
  const content = generateSRT(segments, layer);
  triggerDownload(content, `${baseFilename}.srt`, 'text/plain; charset=utf-8');
}

/**
 * Download segments as VTT file
 */
export function downloadVTT(segments: TranscriptionSegment[], baseFilename: string, layer: SubtitleLayer = 'sentences'): void {
  const content = generateVTT(segments, layer);
  triggerDownload(content, `${baseFilename}.vtt`, 'text/vtt; charset=utf-8');
}

/**
 * Download segments as JSON file
 */
export function downloadJSON(segments: TranscriptionSegment[], baseFilename: string): void {
  const content = generateJSON(segments);
  triggerDownload(content, `${baseFilename}.json`, 'application/json; charset=utf-8');
}

/**
 * Create an HTML viewer page for subtitle content
 */
function createViewerHTML(content: string, title: string, language: string = ''): string {
  const escapedContent = escapeHtml(content);
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>${title}</title>
  <style>
    body {
      font-family: monospace;
      margin: 0;
      padding: 1rem;
      background: #1a1a1f;
      color: #e8e8ec;
      line-height: 1.5;
    }
    pre {
      white-space: pre-wrap;
      word-wrap: break-word;
      margin: 0;
    }
    .header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 1rem;
      padding-bottom: 0.5rem;
      border-bottom: 1px solid #3a3a42;
    }
    h1 {
      font-size: 1rem;
      font-weight: normal;
      margin: 0;
      color: #9ca3af;
    }
    .copy-btn {
      padding: 0.5rem 1rem;
      background: #60a5fa;
      color: white;
      border: none;
      border-radius: 4px;
      cursor: pointer;
      font-size: 0.875rem;
    }
    .copy-btn:hover { background: #3b82f6; }
    .copy-btn.copied { background: #4ade80; }
  </style>
</head>
<body>
  <div class="header">
    <h1>${title}</h1>
    <button class="copy-btn" onclick="copyContent()">Copy to Clipboard</button>
  </div>
  <pre><code${language ? ` class="language-${language}"` : ''}>${escapedContent}</code></pre>
  <script>
    const content = ${JSON.stringify(content)};
    function copyContent() {
      navigator.clipboard.writeText(content).then(() => {
        const btn = document.querySelector('.copy-btn');
        if (!btn) return;
        btn.textContent = 'Copied!';
        btn.classList.add('copied');
        setTimeout(() => {
          btn.textContent = 'Copy to Clipboard';
          btn.classList.remove('copied');
        }, 2000);
      }).catch(() => {
        const btn = document.querySelector('.copy-btn');
        if (!btn) return;
        btn.textContent = 'Copy failed';
        setTimeout(() => { btn.textContent = 'Copy to Clipboard'; }, 2000);
      });
    }
  </script>
</body>
</html>`;
}

/**
 * Open content as an HTML viewer page in a new tab
 */
function openAsViewerPage(content: string, title: string, language?: string): void {
  const html = createViewerHTML(content, title, language);
  const { url, revoke } = createDownloadURL(html, 'text/html; charset=utf-8');

  const newTab = window.open(url, '_blank');

  // Revoke after a delay to ensure tab has loaded
  setTimeout(revoke, 2000);

  // If popup was blocked, fall back to navigating current window
  if (!newTab) {
    window.location.href = url;
  }
}

/**
 * Open SRT content in new tab as formatted viewer
 */
export function openSRT(segments: TranscriptionSegment[], layer: SubtitleLayer = 'sentences'): void {
  const content = generateSRT(segments, layer);
  const label = layer === 'words' ? 'Subtitles - Words (SRT)' : 'Subtitles (SRT)';
  openAsViewerPage(content, label);
}

/**
 * Open VTT content in new tab as formatted viewer
 */
export function openVTT(segments: TranscriptionSegment[], layer: SubtitleLayer = 'sentences'): void {
  const content = generateVTT(segments, layer);
  const label = layer === 'words' ? 'Subtitles - Words (WebVTT)' : 'Subtitles (WebVTT)';
  openAsViewerPage(content, label);
}

/**
 * Open JSON content in new tab as formatted viewer
 */
export function openJSON(segments: TranscriptionSegment[]): void {
  const content = generateJSON(segments);
  openAsViewerPage(content, 'Subtitles (JSON)', 'json');
}
