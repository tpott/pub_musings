/**
 * Feedback submission library for Peekaboo.
 * Handles sending user feedback to the backend API.
 */

import { logger } from './logger';

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
  // Check localStorage
  const stored = localStorage.getItem(SESSION_ID_KEY);
  if (stored) {
    return stored;
  }

  // Generate new session ID
  const sessionId = generateSessionId();
  localStorage.setItem(SESSION_ID_KEY, sessionId);
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

  // Fallback for older browsers
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
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
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  });

  const result: FeedbackResponse = await response.json();

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
