/**
 * Error types for differentiated user-facing error messages
 */

import { logger } from './logger';

export type ErrorType = 'network' | 'rate_limit' | 'server' | 'client' | 'unknown';

/**
 * Custom error class with type information for user-friendly messages
 */
export class ApiError extends Error {
  readonly type: ErrorType;
  readonly status?: number;
  readonly retryAfter?: number; // Seconds to wait before retrying

  constructor(message: string, type: ErrorType, status?: number, retryAfter?: number) {
    super(message);
    this.name = 'ApiError';
    this.type = type;
    this.status = status;
    this.retryAfter = retryAfter;
  }
}

/**
 * Get user-friendly error message based on error type
 */
export function getUserFriendlyMessage(error: Error): string {
  if (error instanceof ApiError) {
    switch (error.type) {
      case 'network':
        return 'Network issue - please check your connection and try again';
      case 'rate_limit':
        if (error.retryAfter) {
          return `Too many requests - please wait ${error.retryAfter} seconds`;
        }
        return 'Too many requests - please wait a moment and try again';
      case 'server':
        return 'Service temporarily unavailable - please try again';
      case 'client':
        return error.message || 'Something went wrong - please try again';
      default:
        return 'Something went wrong - please try again';
    }
  }

  // Handle non-ApiError errors
  if (error.name === 'TypeError' && error.message.includes('Failed to fetch')) {
    return 'Network issue - please check your connection and try again';
  }

  return error.message || 'Something went wrong - please try again';
}

/**
 * Create ApiError from a fetch Response
 */
export async function createApiErrorFromResponse(response: Response, defaultMessage: string): Promise<ApiError> {
  const status = response.status;

  // Determine error type from status code
  let type: ErrorType;
  if (status === 429) {
    type = 'rate_limit';
  } else if (status >= 500) {
    type = 'server';
  } else if (status >= 400) {
    type = 'client';
  } else {
    type = 'unknown';
  }

  // Try to get error message from response body
  let message = defaultMessage;
  try {
    const data = await response.json();
    if (data.error) {
      message = data.error;
    }
  } catch (error) {
    logger.debug('Failed to parse error response JSON:', error);
  }

  // Get Retry-After header for rate limiting
  let retryAfter: number | undefined;
  const retryAfterHeader = response.headers.get('Retry-After');
  if (retryAfterHeader) {
    const seconds = parseInt(retryAfterHeader, 10);
    if (!isNaN(seconds)) {
      retryAfter = seconds;
    }
  }

  return new ApiError(message, type, status, retryAfter);
}

/**
 * Create ApiError from a network error (e.g., fetch failed)
 */
export function createNetworkError(originalError: Error): ApiError {
  return new ApiError(
    originalError.message || 'Network request failed',
    'network'
  );
}
