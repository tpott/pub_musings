import { describe, it, expect } from 'vitest';
import {
  validateEmail,
  validatePassword,
  checkPasswordComplexity,
  getPasswordErrors,
  validateTotpCode,
  validateVideoFile,
  validateSegmentTiming,
  parseTimeString,
  validateSubtitleTime,
  ALLOWED_VIDEO_MIME_TYPES,
  MAX_TIME_SECONDS
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
  it('should accept valid passwords meeting all requirements', () => {
    expect(validatePassword('Password1!')).toBe(null);
    expect(validatePassword('MySecureP@ss1')).toBe(null);
    expect(validatePassword('Abcdefg1@')).toBe(null);
    expect(validatePassword('Test@2024')).toBe(null);
    expect(validatePassword('Complex$Pass123')).toBe(null);
  });

  it('should reject empty/missing password', () => {
    expect(validatePassword('')).toBe('Password is required');
    expect(validatePassword(null as unknown as string)).toBe('Password is required');
    expect(validatePassword(undefined as unknown as string)).toBe('Password is required');
  });

  it('should reject short passwords', () => {
    expect(validatePassword('short')).toBe('Password must be at least 8 characters');
    expect(validatePassword('1234567')).toBe('Password must be at least 8 characters');
    expect(validatePassword('Aa1!')).toBe('Password must be at least 8 characters');
  });

  it('should reject excessively long passwords', () => {
    const longPassword = 'Aa1!' + 'a'.repeat(69); // 73 chars
    expect(validatePassword(longPassword)).toBe('Password is too long (max 72 characters)');
  });

  it('should reject passwords without uppercase', () => {
    expect(validatePassword('password1!')).toBe('Password must contain at least one uppercase letter');
    expect(validatePassword('abcdefg1@')).toBe('Password must contain at least one uppercase letter');
  });

  it('should reject passwords without lowercase', () => {
    expect(validatePassword('PASSWORD1!')).toBe('Password must contain at least one lowercase letter');
    expect(validatePassword('ABCDEFG1@')).toBe('Password must contain at least one lowercase letter');
  });

  it('should reject passwords without numbers', () => {
    expect(validatePassword('Password!')).toBe('Password must contain at least one number');
    expect(validatePassword('Abcdefgh@')).toBe('Password must contain at least one number');
  });

  it('should reject passwords without special characters', () => {
    expect(validatePassword('Password1')).toBe('Password must contain at least one special character');
    expect(validatePassword('Abcdefg12')).toBe('Password must contain at least one special character');
  });
});

describe('checkPasswordComplexity', () => {
  it('should correctly identify all requirements met', () => {
    const result = checkPasswordComplexity('Password1!');
    expect(result.hasMinLength).toBe(true);
    expect(result.hasMaxLength).toBe(true);
    expect(result.hasUppercase).toBe(true);
    expect(result.hasLowercase).toBe(true);
    expect(result.hasNumber).toBe(true);
    expect(result.hasSpecial).toBe(true);
  });

  it('should correctly identify missing requirements', () => {
    const result = checkPasswordComplexity('password');
    expect(result.hasMinLength).toBe(true);
    expect(result.hasMaxLength).toBe(true);
    expect(result.hasUppercase).toBe(false);
    expect(result.hasLowercase).toBe(true);
    expect(result.hasNumber).toBe(false);
    expect(result.hasSpecial).toBe(false);
  });

  it('should detect special characters', () => {
    expect(checkPasswordComplexity('test!')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test@')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test#')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test$')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test%')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test^')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test&')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test*')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test(')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test)')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test_')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test-')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test=')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test[')).toHaveProperty('hasSpecial', true);
    expect(checkPasswordComplexity('test]')).toHaveProperty('hasSpecial', true);
  });
});

describe('getPasswordErrors', () => {
  it('should return empty array for valid password', () => {
    const errors = getPasswordErrors('Password1!');
    expect(errors).toHaveLength(0);
  });

  it('should return all errors for password missing all requirements', () => {
    const errors = getPasswordErrors('abc');
    expect(errors).toContain('At least 8 characters');
    expect(errors).toContain('At least one uppercase letter');
    expect(errors).toContain('At least one number');
    expect(errors).toContain('At least one special character');
    expect(errors).not.toContain('At least one lowercase letter'); // Has lowercase
  });

  it('should handle empty password', () => {
    const errors = getPasswordErrors('');
    expect(errors).toHaveLength(1);
    expect(errors[0]).toBe('Password is required');
  });

  it('should return specific errors for missing requirements', () => {
    const errors = getPasswordErrors('password');
    expect(errors).toContain('At least one uppercase letter');
    expect(errors).toContain('At least one number');
    expect(errors).toContain('At least one special character');
    expect(errors).not.toContain('At least 8 characters');
    expect(errors).not.toContain('At least one lowercase letter');
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

  it('should accept all allowed video MIME types', () => {
    for (const mimeType of ALLOWED_VIDEO_MIME_TYPES) {
      const file = createMockFile('test.video', mimeType, 1024 * 1024);
      expect(validateVideoFile(file)).toBe(null);
    }
  });

  it('should reject non-video files', () => {
    const imageFile = createMockFile('test.jpg', 'image/jpeg', 1024);
    expect(validateVideoFile(imageFile)).toContain('Unsupported video format');

    const textFile = createMockFile('test.txt', 'text/plain', 100);
    expect(validateVideoFile(textFile)).toContain('Unsupported video format');
  });

  it('should reject unsupported video formats', () => {
    // video/3gpp is a video format but not in our whitelist
    const unsupportedFile = createMockFile('test.3gp', 'video/3gpp', 1024);
    expect(validateVideoFile(unsupportedFile)).toContain('Unsupported video format');
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

describe('parseTimeString', () => {
  it('should parse HH:MM:SS.mmm format', () => {
    const result = parseTimeString('01:23:45.678');
    expect(result).toEqual({ seconds: 5025.678 }); // 1*3600 + 23*60 + 45.678
  });

  it('should parse HH:MM:SS format (no milliseconds)', () => {
    const result = parseTimeString('02:30:00');
    expect(result).toEqual({ seconds: 9000 }); // 2*3600 + 30*60
  });

  it('should parse MM:SS.mmm format', () => {
    const result = parseTimeString('05:30.500');
    expect(result).toEqual({ seconds: 330.5 }); // 5*60 + 30.5
  });

  it('should parse MM:SS format', () => {
    const result = parseTimeString('10:15');
    expect(result).toEqual({ seconds: 615 }); // 10*60 + 15
  });

  it('should parse 00:00:00.000 as zero', () => {
    const result = parseTimeString('00:00:00.000');
    expect(result).toEqual({ seconds: 0 });
  });

  it('should handle whitespace', () => {
    const result = parseTimeString('  01:30  ');
    expect(result).toEqual({ seconds: 90 });
  });

  it('should pad milliseconds correctly', () => {
    // ".5" should be interpreted as ".500" (500ms)
    const result = parseTimeString('00:01.5');
    expect(result).toEqual({ seconds: 1.5 });
  });

  it('should reject empty/missing input', () => {
    expect(parseTimeString('')).toEqual({ error: 'Time is required' });
    expect(parseTimeString('   ')).toEqual({ error: 'Time is required' });
    expect(parseTimeString(null as unknown as string)).toEqual({ error: 'Time is required' });
    expect(parseTimeString(undefined as unknown as string)).toEqual({ error: 'Time is required' });
  });

  it('should reject invalid formats', () => {
    expect(parseTimeString('123')).toEqual({ error: 'Invalid time format. Use HH:MM:SS.mmm or MM:SS.mmm' });
    expect(parseTimeString('1:2:3:4')).toEqual({ error: 'Invalid time format. Use HH:MM:SS.mmm or MM:SS.mmm' });
  });

  it('should reject non-numeric values', () => {
    expect(parseTimeString('ab:cd')).toEqual({ error: 'Invalid time format. Numbers only' });
    expect(parseTimeString('1:2x')).toEqual({ error: 'Invalid time format. Numbers only' });
  });

  it('should reject minutes >= 60', () => {
    expect(parseTimeString('60:00')).toEqual({ error: 'Minutes must be 0-59' });
    expect(parseTimeString('01:70:00')).toEqual({ error: 'Minutes must be 0-59' });
  });

  it('should reject seconds >= 60', () => {
    expect(parseTimeString('01:60')).toEqual({ error: 'Seconds must be 0-59' });
    expect(parseTimeString('00:01:60')).toEqual({ error: 'Seconds must be 0-59' });
  });

  it('should reject times exceeding 24 hours', () => {
    const result = parseTimeString('25:00:00');
    expect(result).toEqual({ error: 'Time exceeds maximum (24 hours)' });
  });

  it('should accept times up to exactly 24 hours', () => {
    const result = parseTimeString('24:00:00');
    expect(result).toEqual({ seconds: 86400 }); // MAX_TIME_SECONDS
  });
});

describe('validateSubtitleTime', () => {
  it('should accept valid times', () => {
    expect(validateSubtitleTime(0)).toBe(null);
    expect(validateSubtitleTime(3600)).toBe(null);
    expect(validateSubtitleTime(86400)).toBe(null); // exactly 24 hours
  });

  it('should reject NaN', () => {
    expect(validateSubtitleTime(NaN)).toBe('Invalid time value');
  });

  it('should reject non-numbers', () => {
    expect(validateSubtitleTime('abc' as unknown as number)).toBe('Invalid time value');
  });

  it('should reject negative times', () => {
    expect(validateSubtitleTime(-1)).toBe('Time cannot be negative');
    expect(validateSubtitleTime(-0.001)).toBe('Time cannot be negative');
  });

  it('should reject times exceeding 24 hours', () => {
    expect(validateSubtitleTime(86401)).toBe('Time exceeds maximum (24 hours)');
    expect(validateSubtitleTime(100000)).toBe('Time exceeds maximum (24 hours)');
  });
});

describe('MAX_TIME_SECONDS', () => {
  it('should equal 24 hours in seconds', () => {
    expect(MAX_TIME_SECONDS).toBe(86400);
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

  it('should reject times exceeding 24 hours', () => {
    expect(validateSegmentTiming(86401, 86402)).toBe('Start time exceeds maximum (24 hours)');
    expect(validateSegmentTiming(0, 86401)).toBe('End time exceeds maximum (24 hours)');
  });

  it('should reject start >= end', () => {
    expect(validateSegmentTiming(5, 5)).toBe('End time must be greater than start time');
    expect(validateSegmentTiming(10, 5)).toBe('End time must be greater than start time');
  });
});
