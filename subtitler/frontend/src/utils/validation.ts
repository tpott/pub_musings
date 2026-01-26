/**
 * Validation utilities for the subtitler application
 */

/**
 * Validate email format
 * @param email - Email to validate
 * @returns Error message or null if valid
 */
export function validateEmail(email: string): string | null {
  if (!email || typeof email !== 'string') {
    return 'Email is required';
  }

  const trimmed = email.trim();
  if (trimmed === '') {
    return 'Email is required';
  }

  // Basic email format check
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  if (!emailRegex.test(trimmed)) {
    return 'Invalid email format';
  }

  if (trimmed.length > 255) {
    return 'Email is too long';
  }

  return null;
}

/**
 * Validate password strength
 * @param password - Password to validate
 * @returns Error message or null if valid
 */
export function validatePassword(password: string): string | null {
  if (!password || typeof password !== 'string') {
    return 'Password is required';
  }

  if (password.length < 8) {
    return 'Password must be at least 8 characters';
  }

  if (password.length > 72) {
    return 'Password is too long (max 72 characters)';
  }

  return null;
}

/**
 * Validate TOTP code format (6 digits)
 * @param code - TOTP code to validate
 * @returns Error message or null if valid
 */
export function validateTotpCode(code: string): string | null {
  if (!code || typeof code !== 'string') {
    return '2FA code is required';
  }

  const trimmed = code.trim();
  if (!/^\d{6}$/.test(trimmed)) {
    return '2FA code must be exactly 6 digits';
  }

  return null;
}

/**
 * Allowed video MIME types that the backend will accept
 */
export const ALLOWED_VIDEO_MIME_TYPES = [
  'video/mp4',
  'video/webm',
  'video/quicktime', // .mov files
  'video/x-m4v', // .m4v files
  'video/mpeg', // .mpeg, .mpg files
  'video/x-msvideo', // .avi files
  'video/x-matroska', // .mkv files
  'video/ogg', // .ogv files
];

/**
 * Validate video file for upload
 * @param file - File to validate
 * @param maxSizeMB - Maximum file size in MB (default 500)
 * @returns Error message or null if valid
 */
export function validateVideoFile(file: File | null, maxSizeMB: number = 500): string | null {
  if (!file) {
    return 'No file selected';
  }

  // Check file type against whitelist
  if (!ALLOWED_VIDEO_MIME_TYPES.includes(file.type)) {
    return 'Unsupported video format. Allowed formats: MP4, WebM, MOV, M4V, MPEG, AVI, MKV, OGV';
  }

  // Check file size
  const maxBytes = maxSizeMB * 1024 * 1024;
  if (file.size > maxBytes) {
    return `File is too large (max ${maxSizeMB}MB)`;
  }

  return null;
}

/**
 * Maximum allowed time value in seconds (24 hours)
 */
export const MAX_TIME_SECONDS = 24 * 60 * 60; // 86400 seconds

/**
 * Parse a time string into seconds
 * Supports formats: "HH:MM:SS.mmm", "MM:SS.mmm", "MM:SS", "HH:MM:SS"
 * @param timeStr - Time string to parse
 * @returns Object with either seconds value or error message
 */
export function parseTimeString(timeStr: string): { seconds: number } | { error: string } {
  if (!timeStr || typeof timeStr !== 'string') {
    return { error: 'Time is required' };
  }

  const trimmed = timeStr.trim();
  if (trimmed === '') {
    return { error: 'Time is required' };
  }

  const parts = trimmed.split(':');
  if (parts.length < 2 || parts.length > 3) {
    return { error: 'Invalid time format. Use HH:MM:SS.mmm or MM:SS.mmm' };
  }

  // Helper to strictly parse integer (no trailing non-digits allowed)
  const strictParseInt = (str: string): number => {
    if (!/^\d+$/.test(str)) return NaN;
    return parseInt(str, 10);
  };

  // Helper to strictly parse the seconds part (integer or decimal)
  const strictParseSeconds = (str: string): number => {
    const secParts = str.split('.');
    if (secParts.length > 2) return NaN;

    const wholePart = strictParseInt(secParts[0]);
    if (isNaN(wholePart)) return NaN;

    if (secParts[1]) {
      if (!/^\d+$/.test(secParts[1])) return NaN;
      const msStr = secParts[1].padEnd(3, '0').slice(0, 3);
      return wholePart + parseInt(msStr, 10) / 1000;
    }
    return wholePart;
  };

  let hours = 0;
  let mins = 0;
  let secs = 0;

  if (parts.length === 3) {
    // HH:MM:SS.mmm format
    hours = strictParseInt(parts[0]);
    mins = strictParseInt(parts[1]);
    secs = strictParseSeconds(parts[2]);
  } else {
    // MM:SS.mmm format
    mins = strictParseInt(parts[0]);
    secs = strictParseSeconds(parts[1]);
  }

  // Check for NaN values (invalid number parsing)
  if (isNaN(hours) || isNaN(mins) || isNaN(secs)) {
    return { error: 'Invalid time format. Numbers only' };
  }

  // Validate ranges
  if (hours < 0 || mins < 0 || secs < 0) {
    return { error: 'Time values cannot be negative' };
  }

  if (mins >= 60) {
    return { error: 'Minutes must be 0-59' };
  }

  if (secs >= 60) {
    return { error: 'Seconds must be 0-59' };
  }

  const totalSeconds = hours * 3600 + mins * 60 + secs;

  if (totalSeconds > MAX_TIME_SECONDS) {
    return { error: 'Time exceeds maximum (24 hours)' };
  }

  return { seconds: totalSeconds };
}

/**
 * Validate a subtitle time value (already in seconds)
 * @param seconds - Time in seconds to validate
 * @returns Error message or null if valid
 */
export function validateSubtitleTime(seconds: number): string | null {
  if (typeof seconds !== 'number' || isNaN(seconds)) {
    return 'Invalid time value';
  }

  if (seconds < 0) {
    return 'Time cannot be negative';
  }

  if (seconds > MAX_TIME_SECONDS) {
    return 'Time exceeds maximum (24 hours)';
  }

  return null;
}

/**
 * Validate subtitle segment timing
 * @param start - Start time in seconds
 * @param end - End time in seconds
 * @returns Error message or null if valid
 */
export function validateSegmentTiming(start: number, end: number): string | null {
  if (typeof start !== 'number' || isNaN(start)) {
    return 'Invalid start time';
  }

  if (typeof end !== 'number' || isNaN(end)) {
    return 'Invalid end time';
  }

  if (start < 0) {
    return 'Start time cannot be negative';
  }

  if (end < 0) {
    return 'End time cannot be negative';
  }

  if (start > MAX_TIME_SECONDS) {
    return 'Start time exceeds maximum (24 hours)';
  }

  if (end > MAX_TIME_SECONDS) {
    return 'End time exceeds maximum (24 hours)';
  }

  if (start >= end) {
    return 'End time must be greater than start time';
  }

  return null;
}
