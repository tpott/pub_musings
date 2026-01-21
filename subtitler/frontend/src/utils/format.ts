/**
 * Format utilities for the subtitler application
 */

/**
 * Format bytes into human-readable string
 * @param bytes - Number of bytes
 * @returns Formatted string like "1.5 MB"
 */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

/**
 * Format seconds into SRT-style timestamp (HH:MM:SS.mmm or MM:SS.mmm)
 * @param totalSeconds - Total seconds (can be fractional)
 * @returns Formatted time string
 */
export function formatTime(totalSeconds: number): string {
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  const pad = (n: number): string => n.toString().padStart(2, '0');
  const formatSeconds = (s: number): string => {
    const wholeSecs = Math.floor(s);
    const ms = Math.round((s - wholeSecs) * 1000);
    return `${pad(wholeSecs)}.${ms.toString().padStart(3, '0')}`;
  };

  if (hours > 0) {
    return `${pad(hours)}:${pad(minutes)}:${formatSeconds(seconds)}`;
  }
  return `${pad(minutes)}:${formatSeconds(seconds)}`;
}

/**
 * Format seconds for input fields (always includes hours)
 * @param totalSeconds - Total seconds (can be fractional)
 * @returns Formatted time string HH:MM:SS.mmm
 */
export function formatTimeForInput(totalSeconds: number): string {
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  const pad = (n: number): string => n.toString().padStart(2, '0');
  return `${pad(hours)}:${pad(minutes)}:${pad(Math.floor(seconds))}.${Math.round((seconds % 1) * 1000).toString().padStart(3, '0')}`;
}

/**
 * Parse time input string into seconds
 * Supports formats: HH:MM:SS.mmm, MM:SS.mmm, MM:SS, SS.mmm, SS
 * @param timeStr - Time string to parse
 * @returns Total seconds, or null if invalid
 */
export function parseTimeInput(timeStr: string): number | null {
  if (!timeStr || typeof timeStr !== 'string') {
    return null;
  }

  const trimmed = timeStr.trim();
  if (trimmed === '') {
    return null;
  }

  // Try HH:MM:SS.mmm or MM:SS.mmm format
  const timeRegex = /^(\d+):(\d{1,2}):(\d{1,2})(?:\.(\d{1,3}))?$/;
  const shortRegex = /^(\d+):(\d{1,2})(?:\.(\d{1,3}))?$/;
  const secsRegex = /^(\d+)(?:\.(\d{1,3}))?$/;

  let match = trimmed.match(timeRegex);
  if (match) {
    const hours = parseInt(match[1], 10);
    const minutes = parseInt(match[2], 10);
    const secs = parseInt(match[3], 10);
    const ms = match[4] ? parseInt(match[4].padEnd(3, '0'), 10) : 0;

    if (minutes >= 60 || secs >= 60) return null;

    return hours * 3600 + minutes * 60 + secs + ms / 1000;
  }

  match = trimmed.match(shortRegex);
  if (match) {
    const minutes = parseInt(match[1], 10);
    const secs = parseInt(match[2], 10);
    const ms = match[3] ? parseInt(match[3].padEnd(3, '0'), 10) : 0;

    if (secs >= 60) return null;

    return minutes * 60 + secs + ms / 1000;
  }

  match = trimmed.match(secsRegex);
  if (match) {
    const secs = parseInt(match[1], 10);
    const ms = match[2] ? parseInt(match[2].padEnd(3, '0'), 10) : 0;
    return secs + ms / 1000;
  }

  return null;
}

/**
 * Format ISO date string for display
 * @param dateStr - ISO date string
 * @returns Formatted date string or 'Invalid date'
 */
export function formatDate(dateStr: string): string {
  try {
    const date = new Date(dateStr);
    if (isNaN(date.getTime())) {
      return 'Invalid date';
    }
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  } catch {
    return 'Invalid date';
  }
}

/**
 * Escape HTML special characters to prevent XSS
 * @param text - Text to escape
 * @returns Escaped HTML string
 */
export function escapeHtml(text: string): string {
  if (!text) return '';

  const escapeMap: Record<string, string> = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#039;'
  };

  return text.replace(/[&<>"']/g, char => escapeMap[char] || char);
}

/**
 * Get status badge HTML for transcription status
 * @param status - Status string (none, pending, processing, complete, error)
 * @returns Object with class and label
 */
export function getStatusInfo(status: string): { className: string; label: string } {
  switch (status) {
    case 'none':
      return { className: 'badge-gray', label: 'Not Started' };
    case 'pending':
      return { className: 'badge-yellow', label: 'Pending' };
    case 'processing':
      return { className: 'badge-blue', label: 'Processing' };
    case 'complete':
      return { className: 'badge-green', label: 'Complete' };
    case 'error':
      return { className: 'badge-red', label: 'Error' };
    default:
      return { className: 'badge-gray', label: status || 'Unknown' };
  }
}
