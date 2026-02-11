/**
 * Feedback submission library for Peekaboo.
 * Handles sending user feedback to the backend API.
 */

import { logger } from './logger';
import { getCSRFHeaders } from './csrf';

const SESSION_ID_KEY = 'peekaboo_session_id';

/**
 * Feedback types.
 */
export type FeedbackType = 'general' | 'bug' | 'feature';

/**
 * Context data captured when feedback is submitted.
 */
export interface FeedbackContext {
  sessionId: string;
  pageUrl: string;
  conceptId?: string | null;
  transcript?: string | null;
  userAgent?: string;
}

/**
 * Data required to submit feedback.
 */
export interface FeedbackData {
  type: FeedbackType;
  message: string;
  rating?: number | null;
  context: FeedbackContext;
}

/**
 * Response from feedback API.
 */
export interface FeedbackResponse {
  id?: string;
  status?: string;
  error?: string;
}

/**
 * Gets or creates a session ID for anonymous tracking.
 * Stored in localStorage for persistence across page loads.
 */
export function getOrCreateSessionId(): string {
  try {
    const stored = localStorage.getItem(SESSION_ID_KEY);
    if (stored) {
      return stored;
    }
  } catch { /* private browsing - fall through to generate */ }

  const sessionId = generateSessionId();
  try {
    localStorage.setItem(SESSION_ID_KEY, sessionId);
  } catch { /* private browsing - session ID won't persist */ }
  return sessionId;
}

/**
 * Generates a random session ID.
 */
function generateSessionId(): string {
  // Use crypto.randomUUID if available, otherwise fallback
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }

  // Fallback for older browsers without randomUUID
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
  bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant 1
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

/**
 * Submits feedback to the backend API.
 * Throws an error if the submission fails.
 */
export async function submitFeedback(data: FeedbackData): Promise<FeedbackResponse> {
  const body = {
    type: data.type,
    message: data.message,
    rating: data.rating ?? undefined,
    context: {
      session_id: data.context.sessionId,
      page_url: data.context.pageUrl,
      concept_id: data.context.conceptId ?? undefined,
      transcript: data.context.transcript ?? undefined,
      user_agent: data.context.userAgent ?? undefined,
    },
  };

  logger.debug('Submitting feedback', { type: data.type, hasRating: !!data.rating });

  const response = await fetch('/api/feedback', {
    method: 'POST',
    headers: getCSRFHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(body),
  });

  let result: FeedbackResponse;
  try {
    result = await response.json();
  } catch {
    logger.debug('Failed to parse feedback response as JSON');
    if (!response.ok) {
      throw new Error('Failed to submit feedback');
    }
    return {};
  }

  if (!response.ok) {
    logger.warn('Feedback submission failed', {
      status: response.status,
      error: result.error,
    });

    if (response.status === 429) {
      throw new Error('Too many requests. Please try again later.');
    }

    throw new Error(result.error || 'Failed to submit feedback');
  }

  logger.info('Feedback submitted successfully', { feedbackId: result.id });

  return result;
}
