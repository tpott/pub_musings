/**
 * Intent API client - extracts subject from voice command text
 */

import { fetchWithRetry } from './fetch-with-retry';
import { createApiErrorFromResponse, createNetworkError, ApiError } from './errors';

export interface IntentResult {
  subject: string;
}

/**
 * Send text to intent API to extract the subject
 * @param text Voice command transcript
 * @returns Subject extracted from the text (e.g., "cat", "dog")
 */
export async function extractIntent(text: string): Promise<IntentResult> {
  let response: Response;
  try {
    response = await fetchWithRetry('/api/intent', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ text }),
    });
  } catch (error) {
    // Network error after all retries
    throw createNetworkError(error instanceof Error ? error : new Error(String(error)));
  }

  if (!response.ok) {
    throw await createApiErrorFromResponse(response, 'Intent extraction failed');
  }

  let data;
  try {
    data = await response.json();
  } catch {
    throw new ApiError('Intent extraction failed: invalid response', 'server');
  }

  if (data.error) {
    throw new ApiError(data.error, 'client');
  }

  if (!data.subject) {
    throw new ApiError('No subject extracted', 'client');
  }

  return {
    subject: data.subject,
  };
}
