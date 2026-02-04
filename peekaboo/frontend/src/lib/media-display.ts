/**
 * MediaDisplay - Handles rendering media content (photo/video/audio)
 */

export interface MediaContent {
  photoUrl?: string;
  videoUrl?: string;
  audioUrl?: string;
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
   */
  show(media: MediaContent): void {
    // Stop any existing audio
    this.stopAudio();

    // Clear container
    this.container.innerHTML = '';

    // Show video if available, otherwise show photo
    if (media.videoUrl) {
      this.showVideo(media.videoUrl);
    } else if (media.photoUrl) {
      this.showImage(media.photoUrl);
    }

    // Play audio if available
    if (media.audioUrl) {
      this.playAudio(media.audioUrl);
    }
  }

  /**
   * Show an image in the container
   */
  private showImage(url: string): void {
    const img = document.createElement('img');
    img.src = url;
    img.alt = 'Media content';
    img.dataset.testid = 'media-image';
    this.container.appendChild(img);
  }

  /**
   * Show a video in the container
   */
  private showVideo(url: string): void {
    const video = document.createElement('video');
    video.src = url;
    video.autoplay = true;
    video.loop = true;
    video.muted = true; // Required for autoplay
    video.playsInline = true;
    video.dataset.testid = 'media-video';
    this.container.appendChild(video);
  }

  /**
   * Play audio
   */
  private playAudio(url: string): void {
    this.audioElement = document.createElement('audio');
    this.audioElement.src = url;
    this.audioElement.dataset.testid = 'media-audio';

    // Add to container (hidden but in DOM for testing)
    this.audioElement.style.display = 'none';
    this.container.appendChild(this.audioElement);

    // Attempt autoplay (may be blocked by browser policies)
    this.audioElement.play().catch(error => {
      console.warn('Audio autoplay blocked:', error);
    });
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
    this.container.innerHTML = `<div class="placeholder"><p>${placeholderText}</p></div>`;
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
  const response = await fetch(`/api/media/${encodeURIComponent(concept)}`);

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || `Media fetch failed: ${response.status}`);
  }

  const data = await response.json();
  if (data.error) {
    throw new Error(data.error);
  }

  return {
    photoUrl: data.photo_url,
    videoUrl: data.video_url,
    audioUrl: data.audio_url,
  };
}
