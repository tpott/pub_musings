/**
 * Auth API client for login, register, and session management.
 */

import { getCSRFHeaders, clearCSRFToken } from './csrf';

export interface LoginRequest {
  email: string;
  password: string;
  totp_code?: string;
}

export interface LoginResponse {
  user?: { id: string; email: string; totp_enabled: boolean };
  token?: string;
  error?: string;
  totp_required?: boolean;
  email_not_verified?: boolean;
  can_resend_verification?: boolean;
  retry_after_min?: number;
}

export interface RegisterRequest {
  email: string;
  password: string;
}

export interface RegisterResponse {
  message?: string;
  email_verification?: boolean;
  user?: { id: string; email: string };
  error?: string;
}

export interface VerifyResponse {
  message?: string;
  error?: string;
}

export interface MagicLinkVerifyResponse {
  message?: string;
  user?: { id: string; email: string; totp_enabled: boolean };
  token?: string;
  error?: string;
}

// Cache the in-flight checkAuth promise so multiple concurrent callers
// (LoginButton, LogoutButton, SettingsButton) share one /api/auth/me request.
let authPromise: Promise<boolean> | null = null;

/**
 * Checks if the current user is authenticated by calling /api/auth/me.
 * Returns true if authenticated, false otherwise.
 * Caches the result for the lifetime of the page to avoid redundant requests.
 */
export function checkAuth(): Promise<boolean> {
  if (!authPromise) {
    authPromise = fetch('/api/auth/me', {
      credentials: 'same-origin',
    })
      .then(r => r.ok)
      .catch(() => false);
  }
  return authPromise;
}

/**
 * Reset the auth cache. Call after logout so the next checkAuth() re-fetches.
 */
export function resetAuthCache(): void {
  authPromise = null;
}

/**
 * Validates an email address format.
 */
export function validateEmail(email: string): string | null {
  if (!email.trim()) {
    return 'Email is required';
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    return 'Please enter a valid email address';
  }
  return null;
}

/**
 * Validates a password.
 */
export function validatePassword(password: string): string | null {
  if (!password) {
    return 'Password is required';
  }
  if (password.length < 8) {
    return 'Password must be at least 8 characters';
  }
  return null;
}

/**
 * Logs in with email and password. Optionally includes TOTP code.
 */
export async function login(data: LoginRequest): Promise<LoginResponse> {
  const response = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });

  let result: LoginResponse;
  try {
    result = await response.json();
  } catch {
    throw new Error('Unexpected server response');
  }

  if (!response.ok) {
    if (result.totp_required) {
      return result; // Let caller handle TOTP flow
    }
    if (result.email_not_verified) {
      return result; // Let caller handle verification flow
    }
    if (result.retry_after_min !== undefined) {
      throw new Error(`Account locked. Try again in ${result.retry_after_min} minutes.`);
    }
    throw new Error(result.error || 'Login failed');
  }

  return result;
}

/**
 * Registers a new account with email and password.
 */
export async function register(data: RegisterRequest): Promise<RegisterResponse> {
  const response = await fetch('/api/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });

  let result: RegisterResponse;
  try {
    result = await response.json();
  } catch {
    throw new Error('Unexpected server response');
  }

  if (!response.ok) {
    throw new Error(result.error || 'Registration failed');
  }

  return result;
}

/**
 * Verifies an email address with a token.
 */
export async function verifyEmail(token: string): Promise<VerifyResponse> {
  const response = await fetch(`/api/auth/verify?token=${encodeURIComponent(token)}`);

  let result: VerifyResponse;
  try {
    result = await response.json();
  } catch {
    throw new Error('Unexpected server response');
  }

  if (!response.ok) {
    throw new Error(result.error || 'Verification failed');
  }

  return result;
}

/**
 * Verifies a magic link token and creates a session.
 */
export async function verifyMagicLink(token: string): Promise<MagicLinkVerifyResponse> {
  const response = await fetch(`/api/auth/magic-link/verify?token=${encodeURIComponent(token)}`);

  let result: MagicLinkVerifyResponse;
  try {
    result = await response.json();
  } catch {
    throw new Error('Unexpected server response');
  }

  if (!response.ok) {
    throw new Error(result.error || 'Magic link verification failed');
  }

  return result;
}

export interface LogoutResponse {
  message?: string;
  error?: string;
}

/**
 * Logs out the current user by POSTing to /api/auth/logout with CSRF token.
 * Clears cached CSRF token on success.
 */
export async function logout(): Promise<LogoutResponse> {
  const response = await fetch('/api/auth/logout', {
    method: 'POST',
    headers: getCSRFHeaders({ 'Content-Type': 'application/json' }),
    credentials: 'same-origin',
  });

  let result: LogoutResponse;
  try {
    result = await response.json();
  } catch {
    throw new Error('Unexpected server response');
  }

  if (!response.ok) {
    throw new Error(result.error || 'Logout failed');
  }

  clearCSRFToken();
  resetAuthCache();
  return result;
}
