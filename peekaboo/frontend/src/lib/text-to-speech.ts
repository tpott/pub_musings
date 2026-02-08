/**
 * Text-to-Speech integration using Piper TTS via /api/speak endpoint
 *
 * This module provides TTS synthesis functionality. TTS is optional -
 * if the backend doesn't have PIPER_SERVER_URL configured, the /api/speak
 * endpoint will return 404 and we gracefully handle that case.
 */

import { fetchWithRetry } from './fetch-with-retry';
import { ApiError } from './errors';
import { logger } from './logger';
import { getCSRFHeaders } from './csrf';

/**
 * Check if TTS is available by testing the /api/speak endpoint.
 * Returns true if the backend has TTS configured.
 */
let ttsAvailable: boolean | null = null;

export async function checkTTSAvailability(): Promise<boolean> {
  if (ttsAvailable !== null) {
    return ttsAvailable;
  }

  try {
    // Send a minimal request to check if endpoint exists
    const response = await fetch('/api/speak', {
      method: 'POST',
      headers: getCSRFHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ text: 'test' }),
    });

    // 200 means TTS is available
    // 404 means endpoint doesn't exist (TTS not configured)
    // Other errors might be temporary
    ttsAvailable = response.status === 200;
    return ttsAvailable;
  } catch {
    // Network error - assume TTS not available
    ttsAvailable = false;
    return false;
  }
}

/**
 * Generate a spoken phrase for a given subject.
 *
 * @param subject The subject/animal name (e.g., "cat", "dog")
 * @returns A phrase like "Here is a cat!"
 */
export function generatePhrase(subject: string): string {
  // Simple phrase generation - could be made more varied in the future
  return `Here is a ${subject}!`;
}

/**
 * Synthesize speech from text using the Piper TTS backend.
 *
 * @param text The text to synthesize (max 256 characters)
 * @returns An Audio element ready to play, or null if TTS unavailable
 * @throws ApiError on network or server errors
 */
export async function synthesizeSpeech(text: string): Promise<HTMLAudioElement | null> {
  if (text.length === 0) {
    throw new Error('Text is required for speech synthesis');
  }

  if (text.length > 256) {
    throw new Error('Text too long for speech synthesis (max 256 characters)');
  }

  try {
    const response = await fetchWithRetry('/api/speak', {
      method: 'POST',
      headers: getCSRFHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ text }),
    });

    // Handle 404 - TTS not configured on backend
    if (response.status === 404) {
      ttsAvailable = false;
      return null;
    }

    if (!response.ok) {
      // Parse error response
      const contentType = response.headers.get('content-type');
      if (contentType?.includes('application/json')) {
        const data = await response.json();
        throw new ApiError(
          data.error || 'Speech synthesis failed',
          'server',
          response.status
        );
      }
      throw new ApiError('Speech synthesis failed', 'server', response.status);
    }

    // Convert audio blob to Audio element
    const audioBlob = await response.blob();
    const audioUrl = URL.createObjectURL(audioBlob);
    const audio = new Audio(audioUrl);

    // Clean up blob URL when audio is done or errors
    audio.addEventListener('ended', () => URL.revokeObjectURL(audioUrl), { once: true });
    audio.addEventListener('error', () => URL.revokeObjectURL(audioUrl), { once: true });

    ttsAvailable = true;
    return audio;
  } catch (error) {
    if (error instanceof ApiError) {
      throw error;
    }
    throw new ApiError('Network error during speech synthesis', 'network');
  }
}

/**
 * Play synthesized speech for a subject.
 * This is a convenience function that generates a phrase and synthesizes it.
 *
 * @param subject The subject/animal name
 * @returns Promise that resolves when audio starts playing, or null if TTS unavailable
 */
export async function speakSubject(subject: string): Promise<HTMLAudioElement | null> {
  const phrase = generatePhrase(subject);
  const audio = await synthesizeSpeech(phrase);

  if (audio) {
    // Start playing - don't wait for it to finish
    audio.play().catch((error) => {
      // Autoplay might be blocked - log but don't throw
      logger.warn('TTS autoplay blocked:', error);
    });
  }

  return audio;
}

/**
 * Reset TTS availability check (useful for testing).
 */
export function resetTTSAvailability(): void {
  ttsAvailable = null;
}
