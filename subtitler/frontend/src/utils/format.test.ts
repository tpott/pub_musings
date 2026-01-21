import { describe, it, expect } from 'vitest';
import {
  formatBytes,
  formatTime,
  formatTimeForInput,
  parseTimeInput,
  formatDate,
  escapeHtml,
  getStatusInfo
} from './format';

describe('formatBytes', () => {
  it('should format bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(500)).toBe('500 B');
    expect(formatBytes(1023)).toBe('1023 B');
  });

  it('should format kilobytes', () => {
    expect(formatBytes(1024)).toBe('1.0 KB');
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(1024 * 100)).toBe('100.0 KB');
  });

  it('should format megabytes', () => {
    expect(formatBytes(1024 * 1024)).toBe('1.0 MB');
    expect(formatBytes(1024 * 1024 * 1.5)).toBe('1.5 MB');
    expect(formatBytes(1024 * 1024 * 500)).toBe('500.0 MB');
  });
});

describe('formatTime', () => {
  it('should format zero seconds', () => {
    expect(formatTime(0)).toBe('00:00.000');
  });

  it('should format seconds only', () => {
    expect(formatTime(5)).toBe('00:05.000');
    expect(formatTime(30.5)).toBe('00:30.500');
    expect(formatTime(59.999)).toBe('00:59.999');
  });

  it('should format minutes and seconds', () => {
    expect(formatTime(60)).toBe('01:00.000');
    expect(formatTime(61.5)).toBe('01:01.500');
    expect(formatTime(125.123)).toBe('02:05.123');
    expect(formatTime(3599)).toBe('59:59.000');
  });

  it('should include hours when needed', () => {
    expect(formatTime(3600)).toBe('01:00:00.000');
    expect(formatTime(3661.5)).toBe('01:01:01.500');
    expect(formatTime(7325.999)).toBe('02:02:05.999');
  });
});

describe('formatTimeForInput', () => {
  it('should always include hours', () => {
    expect(formatTimeForInput(0)).toBe('00:00:00.000');
    expect(formatTimeForInput(30.5)).toBe('00:00:30.500');
    expect(formatTimeForInput(61.5)).toBe('00:01:01.500');
    expect(formatTimeForInput(3661.5)).toBe('01:01:01.500');
  });
});

describe('parseTimeInput', () => {
  it('should return null for invalid input', () => {
    expect(parseTimeInput('')).toBe(null);
    expect(parseTimeInput('   ')).toBe(null);
    expect(parseTimeInput('abc')).toBe(null);
    expect(parseTimeInput(null as unknown as string)).toBe(null);
    expect(parseTimeInput(undefined as unknown as string)).toBe(null);
  });

  it('should parse HH:MM:SS.mmm format', () => {
    expect(parseTimeInput('00:00:00.000')).toBe(0);
    expect(parseTimeInput('00:00:30.500')).toBe(30.5);
    expect(parseTimeInput('00:01:01.500')).toBe(61.5);
    expect(parseTimeInput('01:01:01.500')).toBe(3661.5);
    expect(parseTimeInput('01:02:03.456')).toBe(3723.456);
  });

  it('should parse MM:SS.mmm format', () => {
    expect(parseTimeInput('00:00.000')).toBe(0);
    expect(parseTimeInput('00:30.500')).toBe(30.5);
    expect(parseTimeInput('01:01.500')).toBe(61.5);
    expect(parseTimeInput('59:59.999')).toBe(3599.999);
  });

  it('should parse MM:SS format without milliseconds', () => {
    expect(parseTimeInput('00:00')).toBe(0);
    expect(parseTimeInput('01:30')).toBe(90);
    expect(parseTimeInput('10:00')).toBe(600);
  });

  it('should parse seconds only', () => {
    expect(parseTimeInput('30')).toBe(30);
    expect(parseTimeInput('30.5')).toBe(30.5);
    expect(parseTimeInput('120')).toBe(120);
  });

  it('should reject invalid time values', () => {
    // Seconds >= 60 should be rejected in MM:SS format
    expect(parseTimeInput('00:60')).toBe(null);
    expect(parseTimeInput('00:00:60.000')).toBe(null);
    // Minutes >= 60 should be rejected
    expect(parseTimeInput('00:60:00.000')).toBe(null);
  });

  it('should handle partial milliseconds', () => {
    expect(parseTimeInput('00:30.5')).toBe(30.5);
    expect(parseTimeInput('00:30.50')).toBe(30.5);
  });
});

describe('formatDate', () => {
  it('should format valid ISO date strings', () => {
    // Note: This test depends on locale, so we just check it returns something reasonable
    const result = formatDate('2024-01-15T10:30:00Z');
    expect(result).toContain('2024');
    expect(result).toContain('Jan');
  });

  it('should return "Invalid date" for invalid input', () => {
    expect(formatDate('')).toBe('Invalid date');
    expect(formatDate('not a date')).toBe('Invalid date');
    expect(formatDate('2024-99-99')).toBe('Invalid date');
  });
});

describe('escapeHtml', () => {
  it('should return empty string for falsy input', () => {
    expect(escapeHtml('')).toBe('');
    expect(escapeHtml(null as unknown as string)).toBe('');
    expect(escapeHtml(undefined as unknown as string)).toBe('');
  });

  it('should escape HTML special characters', () => {
    expect(escapeHtml('<')).toBe('&lt;');
    expect(escapeHtml('>')).toBe('&gt;');
    expect(escapeHtml('&')).toBe('&amp;');
    expect(escapeHtml('"')).toBe('&quot;');
    expect(escapeHtml("'")).toBe('&#039;');
  });

  it('should escape multiple characters', () => {
    expect(escapeHtml('<script>alert("xss")</script>')).toBe(
      '&lt;script&gt;alert(&quot;xss&quot;)&lt;/script&gt;'
    );
  });

  it('should leave safe characters unchanged', () => {
    expect(escapeHtml('Hello World')).toBe('Hello World');
    expect(escapeHtml('123')).toBe('123');
    expect(escapeHtml('abc.def@example.com')).toBe('abc.def@example.com');
  });
});

describe('getStatusInfo', () => {
  it('should return correct info for known statuses', () => {
    expect(getStatusInfo('none')).toEqual({ className: 'badge-gray', label: 'Not Started' });
    expect(getStatusInfo('pending')).toEqual({ className: 'badge-yellow', label: 'Pending' });
    expect(getStatusInfo('processing')).toEqual({ className: 'badge-blue', label: 'Processing' });
    expect(getStatusInfo('complete')).toEqual({ className: 'badge-green', label: 'Complete' });
    expect(getStatusInfo('error')).toEqual({ className: 'badge-red', label: 'Error' });
  });

  it('should return default for unknown statuses', () => {
    expect(getStatusInfo('unknown')).toEqual({ className: 'badge-gray', label: 'unknown' });
    expect(getStatusInfo('')).toEqual({ className: 'badge-gray', label: 'Unknown' });
  });
});
