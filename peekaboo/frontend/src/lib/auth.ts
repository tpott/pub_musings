/**
 * Auth API client for login, register, and session management.
 */

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
