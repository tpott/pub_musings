/**
 * MediaDisplay - Handles rendering media content (photo/video/audio)
 */

import { fetchWithRetry } from './fetch-with-retry';
import { createApiErrorFromResponse, createNetworkError, ApiError } from './errors';
import { logger } from './logger';

export interface MediaContent {
  photoUrl?: string;
  videoUrl?: string;
  audioUrl?: string;
}

/**
 * Validates that a URL is safe to use for media sources.
 * Only allows http://, https://, and relative URLs (starting with /).
 * Rejects dangerous protocols like javascript:, data:, vbscript:, etc.
 *
 * @param url The URL to validate
 * @returns true if the URL is safe, false otherwise
 */
export function isValidMediaUrl(url: string | undefined): url is string {
  if (!url || typeof url !== 'string') {
    return false;
  }

  const trimmed = url.trim().toLowerCase();

  // Allow relative URLs (start with /)
  if (url.startsWith('/')) {
    return true;
  }

  // Allow http:// and https://
  if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
    return true;
  }

  // Reject everything else (javascript:, data:, vbscript:, etc.)
  return false;
}

/**
 * MediaDisplay handles showing images/videos and playing audio
 */
export class MediaDisplay {
  private container: HTMLElement;
  private audioElement: HTMLAudioElement | null = null;

  constructor(container: HTMLElement) {
    this.container = container;
  }

  /**
   * Display media content in the container
   * @param media Object with photoUrl, videoUrl, audioUrl
   * @param concept Optional concept name for accessibility
   * @returns Promise that resolves to true if audio played successfully, false if blocked
   */
  async show(media: MediaContent, concept?: string): Promise<boolean> {
    // Stop any existing audio
    this.stopAudio();

    // Clear container
    this.container.innerHTML = '';

    // Show video if available, otherwise show photo
    if (media.videoUrl) {
      this.showVideo(media.videoUrl, concept);
    } else if (media.photoUrl) {
      this.showImage(media.photoUrl, concept);
    }

    // Play audio if available
    let audioPlayed = true;
    if (media.audioUrl) {
      audioPlayed = await this.playAudio(media.audioUrl);
    }

    // Update aria-label for the container
    let description = concept ? `Showing ${concept}` : 'Showing media content';
    if (!audioPlayed && media.audioUrl) {
      description += '. Audio playback blocked - tap to play';
      this.showAudioBlockedIndicator();
    }
    this.container.setAttribute('aria-label', description);

    return audioPlayed;
  }

  /**
   * Show an image in the container
   */
  private showImage(url: string, concept?: string): void {
    if (!isValidMediaUrl(url)) {
      logger.error('Invalid image URL rejected:', url);
      return;
    }

    const img = document.createElement('img');
    img.src = url;
    img.alt = concept ? `Photo of a ${concept}` : 'Media content';
    img.dataset.testid = 'media-image';
    img.addEventListener('error', () => {
      logger.error('Failed to load image:', url);
    });
    this.container.appendChild(img);
  }

  /**
   * Show a video in the container
   */
  private showVideo(url: string, concept?: string): void {
    if (!isValidMediaUrl(url)) {
      logger.error('Invalid video URL rejected:', url);
      return;
    }

    const video = document.createElement('video');
    video.src = url;
    video.autoplay = true;
    video.loop = true;
    video.muted = true; // Required for autoplay
    video.playsInline = true;
    video.dataset.testid = 'media-video';
    video.setAttribute('aria-label', concept ? `Video of a ${concept}` : 'Video content');
    video.addEventListener('error', () => {
      logger.error('Failed to load video:', url);
    });
    this.container.appendChild(video);
  }

  /**
   * Play audio
   * @returns Promise resolving to true if audio played successfully, false if blocked
   */
  private async playAudio(url: string): Promise<boolean> {
    if (!isValidMediaUrl(url)) {
      logger.error('Invalid audio URL rejected:', url);
      return false;
    }

    this.audioElement = document.createElement('audio');
    this.audioElement.src = url;
    this.audioElement.dataset.testid = 'media-audio';

    // Add to container (hidden but in DOM for testing)
    this.audioElement.style.display = 'none';
    this.container.appendChild(this.audioElement);

    // Attempt autoplay (may be blocked by browser policies)
    try {
      await this.audioElement.play();
      return true;
    } catch (error) {
      logger.warn('Audio autoplay blocked:', error);
      return false;
    }
  }

  /**
   * Show visual indicator that audio playback was blocked
   */
  private showAudioBlockedIndicator(): void {
    const indicator = document.createElement('div');
    indicator.className = 'audio-blocked-indicator';
    indicator.dataset.testid = 'audio-blocked-indicator';
    indicator.textContent = '🔇 Tap to play sound';
    indicator.setAttribute('role', 'button');
    indicator.setAttribute('tabindex', '0');

    // Clicking the indicator attempts to play the audio
    const playHandler = () => {
      if (this.audioElement) {
        this.audioElement.play().then(() => {
          indicator.remove();
        }).catch(() => {
          // Still blocked, keep indicator
        });
      }
    };

    indicator.addEventListener('click', playHandler);
    indicator.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        playHandler();
      }
    });

    this.container.appendChild(indicator);
  }

  /**
   * Stop playing audio
   */
  stopAudio(): void {
    if (this.audioElement) {
      this.audioElement.pause();
      this.audioElement.src = '';
      this.audioElement = null;
    }
  }

  /**
   * Reset display to placeholder state
   */
  reset(placeholderText: string = 'Say something like "show me a cat"'): void {
    this.stopAudio();
    this.container.innerHTML = '';
    const placeholder = document.createElement('div');
    placeholder.className = 'placeholder';
    const p = document.createElement('p');
    p.textContent = placeholderText;
    placeholder.appendChild(p);
    this.container.appendChild(placeholder);
    this.container.setAttribute('aria-label', 'Media display area');
  }

  /**
   * Get the current audio element (for testing)
   */
  getAudioElement(): HTMLAudioElement | null {
    return this.audioElement;
  }
}

/**
 * Fetch media data for a concept from the API
 * @param concept The concept to fetch (e.g., "cat", "dog")
 * @returns MediaContent object
 */
export async function fetchMedia(concept: string): Promise<MediaContent> {
  let response: Response;
  try {
    response = await fetchWithRetry(`/api/media/${encodeURIComponent(concept)}`);
  } catch (error) {
    // Network error after all retries
    throw createNetworkError(error instanceof Error ? error : new Error(String(error)));
  }

  if (!response.ok) {
    throw await createApiErrorFromResponse(response, 'Media fetch failed');
  }

  const data = await response.json();
  if (data.error) {
    throw new ApiError(data.error, 'client');
  }

  return {
    photoUrl: data.photo_url,
    videoUrl: data.video_url,
    audioUrl: data.audio_url,
  };
}
