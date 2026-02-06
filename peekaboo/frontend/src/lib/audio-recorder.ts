/**
 * AudioRecorder - Handles microphone capture using MediaRecorder API
 */

import { fetchWithRetry } from './fetch-with-retry';
import { createApiErrorFromResponse, createNetworkError, ApiError } from './errors';

export interface RecordingResult {
  blob: Blob;
  mimeType: string;
}

export class AudioRecorder {
  private mediaRecorder: MediaRecorder | null = null;
  private audioChunks: Blob[] = [];
  private stream: MediaStream | null = null;

  /**
   * Check if MediaRecorder is available in the browser
   */
  static isSupported(): boolean {
    return typeof MediaRecorder !== 'undefined' && typeof navigator.mediaDevices !== 'undefined';
  }

  /**
   * Start recording audio from the microphone
   * @throws Error if microphone access is denied or unavailable
   */
  async startRecording(): Promise<void> {
    if (this.mediaRecorder?.state === 'recording') {
      return;
    }

    this.audioChunks = [];

    this.stream = await navigator.mediaDevices.getUserMedia({ audio: true });

    // Prefer webm for wide compatibility with speech recognition
    const mimeType = MediaRecorder.isTypeSupported('audio/webm')
      ? 'audio/webm'
      : 'audio/ogg';

    this.mediaRecorder = new MediaRecorder(this.stream, { mimeType });

    this.mediaRecorder.ondataavailable = (event) => {
      if (event.data.size > 0) {
        this.audioChunks.push(event.data);
      }
    };

    this.mediaRecorder.start();
  }

  /**
   * Stop recording and return the audio blob
   * @returns Promise with the recorded audio blob
   */
  async stopRecording(): Promise<RecordingResult> {
    return new Promise((resolve, reject) => {
      if (!this.mediaRecorder || this.mediaRecorder.state !== 'recording') {
        reject(new Error('Not recording'));
        return;
      }

      this.mediaRecorder.onstop = () => {
        const mimeType = this.mediaRecorder?.mimeType || 'audio/webm';
        const blob = new Blob(this.audioChunks, { type: mimeType });
        this.cleanup();
        resolve({ blob, mimeType });
      };

      this.mediaRecorder.onerror = (event) => {
        this.cleanup();
        reject(new Error('MediaRecorder error'));
      };

      this.mediaRecorder.stop();
    });
  }

  /**
   * Check if currently recording
   */
  isRecording(): boolean {
    return this.mediaRecorder?.state === 'recording';
  }

  /**
   * Stop recording (if active) and release all resources
   */
  destroy(): void {
    if (this.mediaRecorder?.state === 'recording') {
      this.mediaRecorder.stop();
    }
    this.cleanup();
  }

  /**
   * Clean up resources
   */
  private cleanup(): void {
    if (this.stream) {
      this.stream.getTracks().forEach(track => track.stop());
      this.stream = null;
    }
    this.mediaRecorder = null;
    this.audioChunks = [];
  }
}

/**
 * Send audio blob to transcription API
 * @param blob Audio blob to transcribe
 * @returns Transcribed text
 */
export async function transcribeAudio(blob: Blob): Promise<string> {
  const formData = new FormData();
  formData.append('audio', blob, 'audio.webm');

  let response: Response;
  try {
    response = await fetchWithRetry('/api/transcribe', {
      method: 'POST',
      body: formData,
    });
  } catch (error) {
    // Network error after all retries
    throw createNetworkError(error instanceof Error ? error : new Error(String(error)));
  }

  if (!response.ok) {
    throw await createApiErrorFromResponse(response, 'Transcription failed');
  }

  let data;
  try {
    data = await response.json();
  } catch {
    throw new ApiError('Transcription failed: invalid response', 'server');
  }

  if (data.error) {
    throw new ApiError(data.error, 'client');
  }

  return data.text;
}
