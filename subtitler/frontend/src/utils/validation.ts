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

  if (start >= end) {
    return 'End time must be greater than start time';
  }

  return null;
}
