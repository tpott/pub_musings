/**
 * Intent API client - extracts subject from voice command text
 */

export interface IntentResult {
  subject: string;
}

/**
 * Send text to intent API to extract the subject
 * @param text Voice command transcript
 * @returns Subject extracted from the text (e.g., "cat", "dog")
 */
export async function extractIntent(text: string): Promise<IntentResult> {
  const response = await fetch('/api/intent', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ text }),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || `Intent extraction failed: ${response.status}`);
  }

  const data = await response.json();
  if (data.error) {
    throw new Error(data.error);
  }

  if (!data.subject) {
    throw new Error('No subject extracted');
  }

  return {
    subject: data.subject,
  };
}
