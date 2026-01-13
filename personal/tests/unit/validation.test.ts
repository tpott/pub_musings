import { describe, it, expect } from 'vitest';
import { validateEmail, validateRequired, validateMinLength } from '../../src/utils/validation';

describe('validateEmail', () => {
  it('returns true for valid emails', () => {
    expect(validateEmail('test@example.com')).toBe(true);
    expect(validateEmail('user.name@domain.org')).toBe(true);
    expect(validateEmail('user+tag@example.co.uk')).toBe(true);
  });

  it('returns false for invalid emails', () => {
    expect(validateEmail('invalid-email')).toBe(false);
    expect(validateEmail('missing@domain')).toBe(false);
    expect(validateEmail('@nodomain.com')).toBe(false);
    expect(validateEmail('spaces in@email.com')).toBe(false);
    expect(validateEmail('')).toBe(false);
  });
});

describe('validateRequired', () => {
  it('returns true for non-empty strings', () => {
    expect(validateRequired('hello')).toBe(true);
    expect(validateRequired('a')).toBe(true);
  });

  it('returns false for empty or whitespace-only strings', () => {
    expect(validateRequired('')).toBe(false);
    expect(validateRequired('   ')).toBe(false);
    expect(validateRequired('\t\n')).toBe(false);
  });
});

describe('validateMinLength', () => {
  it('returns true when string meets minimum length', () => {
    expect(validateMinLength('hello world', 10)).toBe(true);
    expect(validateMinLength('exactly 10', 10)).toBe(true);
    expect(validateMinLength('short', 5)).toBe(true);
  });

  it('returns false when string is too short', () => {
    expect(validateMinLength('short', 10)).toBe(false);
    expect(validateMinLength('', 1)).toBe(false);
  });

  it('trims whitespace before checking length', () => {
    expect(validateMinLength('  hello  ', 5)).toBe(true);
    expect(validateMinLength('  hi  ', 5)).toBe(false);
  });
});
