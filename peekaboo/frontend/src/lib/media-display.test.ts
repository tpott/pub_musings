import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { MediaDisplay, fetchMedia, isValidMediaUrl } from './media-display';

describe('MediaDisplay', () => {
  let container: HTMLElement;
  let display: MediaDisplay;

  beforeEach(() => {
    // Create a mock container
    container = document.createElement('div');

    // Mock audio play method
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(() => Promise.resolve());
    vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {});

    display = new MediaDisplay(container);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('show', () => {
    it('displays image when photoUrl is provided', async () => {
      await display.show({ photoUrl: '/data/media/cat/photo.jpg' });

      const img = container.querySelector('img[data-testid="media-image"]');
      expect(img).toBeTruthy();
      expect(img?.getAttribute('src')).toBe('/data/media/cat/photo.jpg');
    });

    it('displays video when videoUrl is provided', async () => {
      await display.show({ videoUrl: '/data/media/cat/video.mp4' });

      const video = container.querySelector('video[data-testid="media-video"]');
      expect(video).toBeTruthy();
      expect(video?.getAttribute('src')).toBe('/data/media/cat/video.mp4');
    });

    it('prefers video over photo when both are provided', async () => {
      await display.show({
        photoUrl: '/data/media/cat/photo.jpg',
        videoUrl: '/data/media/cat/video.mp4',
      });

      const video = container.querySelector('video[data-testid="media-video"]');
      const img = container.querySelector('img[data-testid="media-image"]');

      expect(video).toBeTruthy();
      expect(img).toBeFalsy();
    });

    it('creates audio element when audioUrl is provided', async () => {
      await display.show({ audioUrl: '/data/media/cat/audio.mp3' });

      const audio = display.getAudioElement();
      expect(audio).toBeTruthy();
      expect(audio?.getAttribute('src')).toBe('/data/media/cat/audio.mp3');
    });

    it('calls play() on audio element', async () => {
      await display.show({ audioUrl: '/data/media/cat/audio.mp3' });

      expect(HTMLMediaElement.prototype.play).toHaveBeenCalled();
    });

    it('displays image and plays audio when both are provided', async () => {
      await display.show({
        photoUrl: '/data/media/cat/photo.jpg',
        audioUrl: '/data/media/cat/audio.mp3',
      });

      const img = container.querySelector('img[data-testid="media-image"]');
      const audio = display.getAudioElement();

      expect(img).toBeTruthy();
      expect(audio).toBeTruthy();
    });

    it('clears container before showing new content', async () => {
      container.innerHTML = '<p>Old content</p>';

      await display.show({ photoUrl: '/data/media/cat/photo.jpg' });

      expect(container.querySelector('p')).toBeFalsy();
      expect(container.querySelector('img')).toBeTruthy();
    });

    it('stops previous audio when showing new content', async () => {
      await display.show({ audioUrl: '/data/media/cat/audio.mp3' });
      await display.show({ audioUrl: '/data/media/dog/audio.mp3' });

      expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
    });

    it('returns true when audio plays successfully', async () => {
      const result = await display.show({ audioUrl: '/data/media/cat/audio.mp3' });
      expect(result).toBe(true);
    });

    it('returns true when no audio is provided', async () => {
      const result = await display.show({ photoUrl: '/data/media/cat/photo.jpg' });
      expect(result).toBe(true);
    });

    it('returns false when audio autoplay is blocked', async () => {
      vi.spyOn(HTMLMediaElement.prototype, 'play').mockRejectedValue(new Error('NotAllowedError'));

      const result = await display.show({ audioUrl: '/data/media/cat/audio.mp3' });
      expect(result).toBe(false);
    });

    it('shows audio blocked indicator when autoplay is blocked', async () => {
      vi.spyOn(HTMLMediaElement.prototype, 'play').mockRejectedValue(new Error('NotAllowedError'));

      await display.show({ audioUrl: '/data/media/cat/audio.mp3' });

      const indicator = container.querySelector('[data-testid="audio-blocked-indicator"]');
      expect(indicator).toBeTruthy();
      expect(indicator?.textContent).toContain('Tap to play');
    });

    it('updates aria-label when audio is blocked', async () => {
      vi.spyOn(HTMLMediaElement.prototype, 'play').mockRejectedValue(new Error('NotAllowedError'));

      await display.show({ photoUrl: '/data/media/cat/photo.jpg', audioUrl: '/data/media/cat/audio.mp3' }, 'cat');

      expect(container.getAttribute('aria-label')).toContain('Audio playback blocked');
    });

    it('does not show indicator when audio plays successfully', async () => {
      await display.show({ audioUrl: '/data/media/cat/audio.mp3' });

      const indicator = container.querySelector('[data-testid="audio-blocked-indicator"]');
      expect(indicator).toBeFalsy();
    });
  });

  describe('stopAudio', () => {
    it('stops audio playback', async () => {
      await display.show({ audioUrl: '/data/media/cat/audio.mp3' });

      display.stopAudio();

      expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
      expect(display.getAudioElement()).toBeNull();
    });

    it('does nothing if no audio is playing', () => {
      display.stopAudio();

      expect(HTMLMediaElement.prototype.pause).not.toHaveBeenCalled();
    });
  });

  describe('reset', () => {
    it('restores placeholder text', async () => {
      await display.show({ photoUrl: '/data/media/cat/photo.jpg' });

      display.reset();

      expect(container.innerHTML).toContain('Say something like "show me a cat"');
      expect(container.querySelector('img')).toBeFalsy();
    });

    it('accepts custom placeholder text', () => {
      display.reset('Custom message');

      expect(container.innerHTML).toContain('Custom message');
    });

    it('stops audio when resetting', async () => {
      await display.show({ audioUrl: '/data/media/cat/audio.mp3' });

      display.reset();

      expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
    });
  });

  describe('video element properties', () => {
    it('sets autoplay and loop properties', async () => {
      await display.show({ videoUrl: '/data/media/cat/video.mp4' });

      const video = container.querySelector('video') as HTMLVideoElement;
      expect(video.autoplay).toBe(true);
      expect(video.loop).toBe(true);
      expect(video.muted).toBe(true);
      expect(video.playsInline).toBe(true);
    });
  });

  describe('media load errors', () => {
    it('logs error when image fails to load', async () => {
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      await display.show({ photoUrl: '/data/media/missing/photo.jpg' });

      const img = container.querySelector('img') as HTMLImageElement;
      img.dispatchEvent(new Event('error'));

      expect(consoleSpy).toHaveBeenCalledWith(
        '[ERROR] Failed to load image:', '/data/media/missing/photo.jpg'
      );
      consoleSpy.mockRestore();
    });

    it('logs error when video fails to load', async () => {
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      await display.show({ videoUrl: '/data/media/missing/video.mp4' });

      const video = container.querySelector('video') as HTMLVideoElement;
      video.dispatchEvent(new Event('error'));

      expect(consoleSpy).toHaveBeenCalledWith(
        '[ERROR] Failed to load video:', '/data/media/missing/video.mp4'
      );
      consoleSpy.mockRestore();
    });
  });

  describe('accessibility', () => {
    it('sets aria-label on container when showing media with concept', async () => {
      await display.show({ photoUrl: '/data/media/cat/photo.jpg' }, 'cat');

      expect(container.getAttribute('aria-label')).toBe('Showing cat');
    });

    it('sets generic aria-label when no concept provided', async () => {
      await display.show({ photoUrl: '/data/media/cat/photo.jpg' });

      expect(container.getAttribute('aria-label')).toBe('Showing media content');
    });

    it('resets aria-label when reset is called', async () => {
      await display.show({ photoUrl: '/data/media/cat/photo.jpg' }, 'cat');
      display.reset();

      expect(container.getAttribute('aria-label')).toBe('Media display area');
    });

    it('sets descriptive alt text on images when concept is provided', async () => {
      await display.show({ photoUrl: '/data/media/cat/photo.jpg' }, 'cat');

      const img = container.querySelector('img');
      expect(img?.getAttribute('alt')).toBe('Photo of a cat');
    });

    it('sets generic alt text on images when no concept provided', async () => {
      await display.show({ photoUrl: '/data/media/cat/photo.jpg' });

      const img = container.querySelector('img');
      expect(img?.getAttribute('alt')).toBe('Media content');
    });

    it('sets aria-label on video when concept is provided', async () => {
      await display.show({ videoUrl: '/data/media/cat/video.mp4' }, 'cat');

      const video = container.querySelector('video');
      expect(video?.getAttribute('aria-label')).toBe('Video of a cat');
    });

    it('sets generic aria-label on video when no concept provided', async () => {
      await display.show({ videoUrl: '/data/media/cat/video.mp4' });

      const video = container.querySelector('video');
      expect(video?.getAttribute('aria-label')).toBe('Video content');
    });
  });
});

describe('isValidMediaUrl', () => {
  describe('valid URLs', () => {
    it('accepts relative URLs starting with /', () => {
      expect(isValidMediaUrl('/data/media/cat/photo.jpg')).toBe(true);
      expect(isValidMediaUrl('/api/media/cat')).toBe(true);
      expect(isValidMediaUrl('/')).toBe(true);
    });

    it('accepts http:// URLs', () => {
      expect(isValidMediaUrl('http://example.com/photo.jpg')).toBe(true);
      expect(isValidMediaUrl('http://localhost:8080/data/media/cat.jpg')).toBe(true);
    });

    it('accepts https:// URLs', () => {
      expect(isValidMediaUrl('https://example.com/photo.jpg')).toBe(true);
      expect(isValidMediaUrl('https://cdn.example.com/media/audio.mp3')).toBe(true);
    });

    it('accepts URLs with mixed case protocols', () => {
      expect(isValidMediaUrl('HTTP://example.com/photo.jpg')).toBe(true);
      expect(isValidMediaUrl('HTTPS://example.com/photo.jpg')).toBe(true);
    });
  });

  describe('invalid URLs', () => {
    it('rejects javascript: URLs', () => {
      expect(isValidMediaUrl('javascript:alert(1)')).toBe(false);
      expect(isValidMediaUrl('javascript:void(0)')).toBe(false);
      expect(isValidMediaUrl('JAVASCRIPT:alert(1)')).toBe(false);
    });

    it('rejects data: URLs', () => {
      expect(isValidMediaUrl('data:text/html,<script>alert(1)</script>')).toBe(false);
      expect(isValidMediaUrl('data:image/png;base64,abc')).toBe(false);
    });

    it('rejects vbscript: URLs', () => {
      expect(isValidMediaUrl('vbscript:alert(1)')).toBe(false);
    });

    it('rejects file: URLs', () => {
      expect(isValidMediaUrl('file:///etc/passwd')).toBe(false);
    });

    it('rejects empty and undefined URLs', () => {
      expect(isValidMediaUrl('')).toBe(false);
      expect(isValidMediaUrl(undefined)).toBe(false);
    });

    it('rejects URLs with whitespace padding that try to hide protocol', () => {
      // Note: We trim and lowercase before checking
      expect(isValidMediaUrl('  javascript:alert(1)')).toBe(false);
    });

    it('rejects relative paths that do not start with /', () => {
      expect(isValidMediaUrl('photo.jpg')).toBe(false);
      expect(isValidMediaUrl('../data/media/photo.jpg')).toBe(false);
    });
  });
});

describe('MediaDisplay URL validation', () => {
  let container: HTMLElement;
  let display: MediaDisplay;
  let consoleSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    container = document.createElement('div');
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(() => Promise.resolve());
    vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {});
    consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    display = new MediaDisplay(container);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('rejects javascript: URLs for images', async () => {
    await display.show({ photoUrl: 'javascript:alert(1)' });

    const img = container.querySelector('img');
    expect(img).toBeFalsy();
    expect(consoleSpy).toHaveBeenCalledWith('[ERROR] Invalid image URL rejected:', 'javascript:alert(1)');
  });

  it('rejects javascript: URLs for videos', async () => {
    await display.show({ videoUrl: 'javascript:alert(1)' });

    const video = container.querySelector('video');
    expect(video).toBeFalsy();
    expect(consoleSpy).toHaveBeenCalledWith('[ERROR] Invalid video URL rejected:', 'javascript:alert(1)');
  });

  it('rejects javascript: URLs for audio', async () => {
    await display.show({ audioUrl: 'javascript:alert(1)' });

    const audio = display.getAudioElement();
    expect(audio).toBeFalsy();
    expect(consoleSpy).toHaveBeenCalledWith('[ERROR] Invalid audio URL rejected:', 'javascript:alert(1)');
  });

  it('rejects data: URLs for images', async () => {
    await display.show({ photoUrl: 'data:text/html,<script>alert(1)</script>' });

    const img = container.querySelector('img');
    expect(img).toBeFalsy();
  });

  it('accepts valid relative URLs', async () => {
    await display.show({ photoUrl: '/data/media/cat/photo.jpg' });

    const img = container.querySelector('img');
    expect(img).toBeTruthy();
    expect(img?.getAttribute('src')).toBe('/data/media/cat/photo.jpg');
  });

  it('accepts valid https URLs', async () => {
    await display.show({ photoUrl: 'https://example.com/cat.jpg' });

    const img = container.querySelector('img');
    expect(img).toBeTruthy();
  });
});

describe('fetchMedia', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches media from /api/media/{concept}', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({
        photo_url: '/data/media/cat/photo.jpg',
        audio_url: '/data/media/cat/audio.mp3',
      }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const result = await fetchMedia('cat');

    expect(global.fetch).toHaveBeenCalledWith('/api/media/cat', undefined);
    expect(result).toEqual({
      photoUrl: '/data/media/cat/photo.jpg',
      videoUrl: undefined,
      audioUrl: '/data/media/cat/audio.mp3',
    });
  });

  it('encodes concept parameter', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({ photo_url: '/photo.jpg' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await fetchMedia('my cat');

    expect(global.fetch).toHaveBeenCalledWith('/api/media/my%20cat', undefined);
  });

  it('throws error on failed response', async () => {
    // Use 404 which is not retried
    const mockResponse = {
      ok: false,
      status: 404,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({ error: 'concept not found' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await expect(fetchMedia('elephant')).rejects.toThrow('concept not found');
  });

  it('throws error when response contains error field', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({ error: 'no media found for concept' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await expect(fetchMedia('cat')).rejects.toThrow('no media found for concept');
  });

  it('includes video_url when available', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({
        photo_url: '/photo.jpg',
        video_url: '/video.mp4',
        audio_url: '/audio.mp3',
      }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const result = await fetchMedia('cat');

    expect(result.videoUrl).toBe('/video.mp4');
  });
});
