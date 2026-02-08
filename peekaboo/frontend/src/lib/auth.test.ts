import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { validateEmail, validatePassword, login, register } from './auth';

describe('auth', () => {
  describe('validateEmail', () => {
    it('returns null for valid email', () => {
      expect(validateEmail('user@example.com')).toBeNull();
    });

    it('returns error for empty email', () => {
      expect(validateEmail('')).toBe('Email is required');
    });

    it('returns error for whitespace-only email', () => {
      expect(validateEmail('   ')).toBe('Email is required');
    });

    it('returns error for email without @', () => {
      expect(validateEmail('userexample.com')).toBe('Please enter a valid email address');
    });

    it('returns error for email without domain', () => {
      expect(validateEmail('user@')).toBe('Please enter a valid email address');
    });

    it('returns error for email without TLD', () => {
      expect(validateEmail('user@example')).toBe('Please enter a valid email address');
    });

    it('accepts email with subdomain', () => {
      expect(validateEmail('user@mail.example.com')).toBeNull();
    });
  });

  describe('validatePassword', () => {
    it('returns null for valid password', () => {
      expect(validatePassword('password123')).toBeNull();
    });

    it('returns error for empty password', () => {
      expect(validatePassword('')).toBe('Password is required');
    });

    it('returns error for short password', () => {
      expect(validatePassword('1234567')).toBe('Password must be at least 8 characters');
    });

    it('accepts exactly 8 characters', () => {
      expect(validatePassword('12345678')).toBeNull();
    });
  });

  describe('login', () => {
    let fetchSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      fetchSpy = vi.spyOn(globalThis, 'fetch');
    });

    afterEach(() => {
      fetchSpy.mockRestore();
    });

    it('sends login request and returns user on success', async () => {
      const mockResponse = {
        user: { id: 'user-1', email: 'test@example.com', totp_enabled: false },
        token: 'session-token',
      };
      fetchSpy.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      } as Response);

      const result = await login({ email: 'test@example.com', password: 'password123' });

      expect(fetchSpy).toHaveBeenCalledWith('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'test@example.com', password: 'password123' }),
      });
      expect(result.user?.email).toBe('test@example.com');
    });

    it('includes totp_code when provided', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ user: { id: '1', email: 'a@b.c', totp_enabled: true } }),
      } as Response);

      await login({ email: 'a@b.c', password: 'pw', totp_code: '123456' });

      const body = JSON.parse((fetchSpy.mock.calls[0][1] as RequestInit).body as string);
      expect(body.totp_code).toBe('123456');
    });

    it('returns totp_required when 2FA is needed', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({ totp_required: true }),
      } as Response);

      const result = await login({ email: 'a@b.c', password: 'pw' });
      expect(result.totp_required).toBe(true);
    });

    it('returns email_not_verified when unverified', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({ email_not_verified: true, can_resend_verification: true }),
      } as Response);

      const result = await login({ email: 'a@b.c', password: 'pw' });
      expect(result.email_not_verified).toBe(true);
    });

    it('throws on account lockout', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({ error: 'account locked', retry_after_min: 10 }),
      } as Response);

      await expect(login({ email: 'a@b.c', password: 'pw' }))
        .rejects.toThrow('Account locked. Try again in 10 minutes.');
    });

    it('throws on generic error', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({ error: 'invalid credentials' }),
      } as Response);

      await expect(login({ email: 'a@b.c', password: 'pw' }))
        .rejects.toThrow('invalid credentials');
    });

    it('throws on non-JSON response', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.reject(new Error('not JSON')),
      } as Response);

      await expect(login({ email: 'a@b.c', password: 'pw' }))
        .rejects.toThrow('Unexpected server response');
    });
  });

  describe('register', () => {
    let fetchSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      fetchSpy = vi.spyOn(globalThis, 'fetch');
    });

    afterEach(() => {
      fetchSpy.mockRestore();
    });

    it('sends register request and returns message on success', async () => {
      const mockResponse = {
        message: 'Check your email to verify your account',
        email_verification: true,
        user: { id: 'user-1', email: 'test@example.com' },
      };
      fetchSpy.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      } as Response);

      const result = await register({ email: 'test@example.com', password: 'password123' });

      expect(fetchSpy).toHaveBeenCalledWith('/api/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'test@example.com', password: 'password123' }),
      });
      expect(result.message).toBe('Check your email to verify your account');
      expect(result.email_verification).toBe(true);
    });

    it('throws on duplicate email', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({ error: 'email already registered' }),
      } as Response);

      await expect(register({ email: 'a@b.c', password: 'password123' }))
        .rejects.toThrow('email already registered');
    });

    it('throws on server error with default message', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({}),
      } as Response);

      await expect(register({ email: 'a@b.c', password: 'password123' }))
        .rejects.toThrow('Registration failed');
    });

    it('throws on non-JSON response', async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.reject(new Error('not JSON')),
      } as Response);

      await expect(register({ email: 'a@b.c', password: 'password123' }))
        .rejects.toThrow('Unexpected server response');
    });
  });
});
