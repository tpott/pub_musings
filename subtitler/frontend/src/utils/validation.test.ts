import { describe, it, expect } from 'vitest';
import {
  validateEmail,
  validatePassword,
  validateTotpCode,
  validateVideoFile,
  validateSegmentTiming
} from './validation';

describe('validateEmail', () => {
  it('should accept valid emails', () => {
    expect(validateEmail('test@example.com')).toBe(null);
    expect(validateEmail('user.name@domain.org')).toBe(null);
    expect(validateEmail('user+tag@example.co.uk')).toBe(null);
  });

  it('should reject empty/missing email', () => {
    expect(validateEmail('')).toBe('Email is required');
    expect(validateEmail('   ')).toBe('Email is required');
    expect(validateEmail(null as unknown as string)).toBe('Email is required');
    expect(validateEmail(undefined as unknown as string)).toBe('Email is required');
  });

  it('should reject invalid email format', () => {
    expect(validateEmail('notanemail')).toBe('Invalid email format');
    expect(validateEmail('@example.com')).toBe('Invalid email format');
    expect(validateEmail('user@')).toBe('Invalid email format');
    expect(validateEmail('user@domain')).toBe('Invalid email format');
    expect(validateEmail('user name@domain.com')).toBe('Invalid email format');
  });

  it('should reject excessively long emails', () => {
    const longEmail = 'a'.repeat(250) + '@example.com';
    expect(validateEmail(longEmail)).toBe('Email is too long');
  });
});

describe('validatePassword', () => {
  it('should accept valid passwords', () => {
    expect(validatePassword('password123')).toBe(null);
    expect(validatePassword('MySecureP@ss!')).toBe(null);
    expect(validatePassword('12345678')).toBe(null);
  });

  it('should reject empty/missing password', () => {
    expect(validatePassword('')).toBe('Password is required');
    expect(validatePassword(null as unknown as string)).toBe('Password is required');
    expect(validatePassword(undefined as unknown as string)).toBe('Password is required');
  });

  it('should reject short passwords', () => {
    expect(validatePassword('short')).toBe('Password must be at least 8 characters');
    expect(validatePassword('1234567')).toBe('Password must be at least 8 characters');
  });

  it('should reject excessively long passwords', () => {
    const longPassword = 'a'.repeat(73);
    expect(validatePassword(longPassword)).toBe('Password is too long (max 72 characters)');
  });
});

describe('validateTotpCode', () => {
  it('should accept valid 6-digit codes', () => {
    expect(validateTotpCode('123456')).toBe(null);
    expect(validateTotpCode('000000')).toBe(null);
    expect(validateTotpCode('999999')).toBe(null);
    expect(validateTotpCode('  123456  ')).toBe(null); // With whitespace
  });

  it('should reject empty/missing code', () => {
    expect(validateTotpCode('')).toBe('2FA code is required');
    expect(validateTotpCode(null as unknown as string)).toBe('2FA code is required');
    expect(validateTotpCode(undefined as unknown as string)).toBe('2FA code is required');
  });

  it('should reject non-6-digit codes', () => {
    expect(validateTotpCode('12345')).toBe('2FA code must be exactly 6 digits');
    expect(validateTotpCode('1234567')).toBe('2FA code must be exactly 6 digits');
    expect(validateTotpCode('abcdef')).toBe('2FA code must be exactly 6 digits');
    expect(validateTotpCode('12-345')).toBe('2FA code must be exactly 6 digits');
  });
});

describe('validateVideoFile', () => {
  // Helper to create mock File objects
  const createMockFile = (name: string, type: string, size: number): File => {
    const file = new File([''], name, { type });
    Object.defineProperty(file, 'size', { value: size });
    return file;
  };

  it('should accept valid video files', () => {
    const mp4File = createMockFile('test.mp4', 'video/mp4', 1024 * 1024);
    expect(validateVideoFile(mp4File)).toBe(null);

    const webmFile = createMockFile('test.webm', 'video/webm', 1024 * 1024);
    expect(validateVideoFile(webmFile)).toBe(null);

    const movFile = createMockFile('test.mov', 'video/quicktime', 1024 * 1024);
    expect(validateVideoFile(movFile)).toBe(null);
  });

  it('should reject null file', () => {
    expect(validateVideoFile(null)).toBe('No file selected');
  });

  it('should reject non-video files', () => {
    const imageFile = createMockFile('test.jpg', 'image/jpeg', 1024);
    expect(validateVideoFile(imageFile)).toBe('File must be a video');

    const textFile = createMockFile('test.txt', 'text/plain', 100);
    expect(validateVideoFile(textFile)).toBe('File must be a video');
  });

  it('should reject files exceeding max size', () => {
    const largeFile = createMockFile('large.mp4', 'video/mp4', 501 * 1024 * 1024);
    expect(validateVideoFile(largeFile)).toBe('File is too large (max 500MB)');
  });

  it('should respect custom max size', () => {
    const file = createMockFile('test.mp4', 'video/mp4', 101 * 1024 * 1024);
    expect(validateVideoFile(file, 100)).toBe('File is too large (max 100MB)');
    expect(validateVideoFile(file, 200)).toBe(null);
  });
});

describe('validateSegmentTiming', () => {
  it('should accept valid timing', () => {
    expect(validateSegmentTiming(0, 5)).toBe(null);
    expect(validateSegmentTiming(1.5, 3.5)).toBe(null);
    expect(validateSegmentTiming(0.001, 0.002)).toBe(null);
  });

  it('should reject invalid start time', () => {
    expect(validateSegmentTiming(NaN, 5)).toBe('Invalid start time');
    expect(validateSegmentTiming('abc' as unknown as number, 5)).toBe('Invalid start time');
  });

  it('should reject invalid end time', () => {
    expect(validateSegmentTiming(0, NaN)).toBe('Invalid end time');
    expect(validateSegmentTiming(0, 'abc' as unknown as number)).toBe('Invalid end time');
  });

  it('should reject negative times', () => {
    expect(validateSegmentTiming(-1, 5)).toBe('Start time cannot be negative');
    expect(validateSegmentTiming(0, -1)).toBe('End time cannot be negative');
  });

  it('should reject start >= end', () => {
    expect(validateSegmentTiming(5, 5)).toBe('End time must be greater than start time');
    expect(validateSegmentTiming(10, 5)).toBe('End time must be greater than start time');
  });
});
