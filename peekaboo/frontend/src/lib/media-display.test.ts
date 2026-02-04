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
    it('displays image when photoUrl is provided', () => {
      display.show({ photoUrl: '/data/media/cat/photo.jpg' });

      const img = container.querySelector('img[data-testid="media-image"]');
      expect(img).toBeTruthy();
      expect(img?.getAttribute('src')).toBe('/data/media/cat/photo.jpg');
    });

    it('displays video when videoUrl is provided', () => {
      display.show({ videoUrl: '/data/media/cat/video.mp4' });

      const video = container.querySelector('video[data-testid="media-video"]');
      expect(video).toBeTruthy();
      expect(video?.getAttribute('src')).toBe('/data/media/cat/video.mp4');
    });

    it('prefers video over photo when both are provided', () => {
      display.show({
        photoUrl: '/data/media/cat/photo.jpg',
        videoUrl: '/data/media/cat/video.mp4',
      });

      const video = container.querySelector('video[data-testid="media-video"]');
      const img = container.querySelector('img[data-testid="media-image"]');

      expect(video).toBeTruthy();
      expect(img).toBeFalsy();
    });

    it('creates audio element when audioUrl is provided', () => {
      display.show({ audioUrl: '/data/media/cat/audio.mp3' });

      const audio = display.getAudioElement();
      expect(audio).toBeTruthy();
      expect(audio?.getAttribute('src')).toBe('/data/media/cat/audio.mp3');
    });

    it('calls play() on audio element', () => {
      display.show({ audioUrl: '/data/media/cat/audio.mp3' });

      expect(HTMLMediaElement.prototype.play).toHaveBeenCalled();
    });

    it('displays image and plays audio when both are provided', () => {
      display.show({
        photoUrl: '/data/media/cat/photo.jpg',
        audioUrl: '/data/media/cat/audio.mp3',
      });

      const img = container.querySelector('img[data-testid="media-image"]');
      const audio = display.getAudioElement();

      expect(img).toBeTruthy();
      expect(audio).toBeTruthy();
    });

    it('clears container before showing new content', () => {
      container.innerHTML = '<p>Old content</p>';

      display.show({ photoUrl: '/data/media/cat/photo.jpg' });

      expect(container.querySelector('p')).toBeFalsy();
      expect(container.querySelector('img')).toBeTruthy();
    });

    it('stops previous audio when showing new content', () => {
      display.show({ audioUrl: '/data/media/cat/audio.mp3' });
      display.show({ audioUrl: '/data/media/dog/audio.mp3' });

      expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
    });
  });

  describe('stopAudio', () => {
    it('stops audio playback', () => {
      display.show({ audioUrl: '/data/media/cat/audio.mp3' });

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
    it('restores placeholder text', () => {
      display.show({ photoUrl: '/data/media/cat/photo.jpg' });

      display.reset();

      expect(container.innerHTML).toContain('Say something like "show me a cat"');
      expect(container.querySelector('img')).toBeFalsy();
    });

    it('accepts custom placeholder text', () => {
      display.reset('Custom message');

      expect(container.innerHTML).toContain('Custom message');
    });

    it('stops audio when resetting', () => {
      display.show({ audioUrl: '/data/media/cat/audio.mp3' });

      display.reset();

      expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
    });
  });

  describe('video element properties', () => {
    it('sets autoplay and loop properties', () => {
      display.show({ videoUrl: '/data/media/cat/video.mp4' });

      const video = container.querySelector('video') as HTMLVideoElement;
      expect(video.autoplay).toBe(true);
      expect(video.loop).toBe(true);
      expect(video.muted).toBe(true);
      expect(video.playsInline).toBe(true);
    });
  });

  describe('accessibility', () => {
    it('sets aria-label on container when showing media with concept', () => {
      display.show({ photoUrl: '/data/media/cat/photo.jpg' }, 'cat');

      expect(container.getAttribute('aria-label')).toBe('Showing cat');
    });

    it('sets generic aria-label when no concept provided', () => {
      display.show({ photoUrl: '/data/media/cat/photo.jpg' });

      expect(container.getAttribute('aria-label')).toBe('Showing media content');
    });

    it('resets aria-label when reset is called', () => {
      display.show({ photoUrl: '/data/media/cat/photo.jpg' }, 'cat');
      display.reset();

      expect(container.getAttribute('aria-label')).toBe('Media display area');
    });

    it('sets descriptive alt text on images when concept is provided', () => {
      display.show({ photoUrl: '/data/media/cat/photo.jpg' }, 'cat');

      const img = container.querySelector('img');
      expect(img?.getAttribute('alt')).toBe('Photo of a cat');
    });

    it('sets generic alt text on images when no concept provided', () => {
      display.show({ photoUrl: '/data/media/cat/photo.jpg' });

      const img = container.querySelector('img');
      expect(img?.getAttribute('alt')).toBe('Media content');
    });

    it('sets aria-label on video when concept is provided', () => {
      display.show({ videoUrl: '/data/media/cat/video.mp4' }, 'cat');

      const video = container.querySelector('video');
      expect(video?.getAttribute('aria-label')).toBe('Video of a cat');
    });

    it('sets generic aria-label on video when no concept provided', () => {
      display.show({ videoUrl: '/data/media/cat/video.mp4' });

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

  it('rejects javascript: URLs for images', () => {
    display.show({ photoUrl: 'javascript:alert(1)' });

    const img = container.querySelector('img');
    expect(img).toBeFalsy();
    expect(consoleSpy).toHaveBeenCalledWith('Invalid image URL rejected:', 'javascript:alert(1)');
  });

  it('rejects javascript: URLs for videos', () => {
    display.show({ videoUrl: 'javascript:alert(1)' });

    const video = container.querySelector('video');
    expect(video).toBeFalsy();
    expect(consoleSpy).toHaveBeenCalledWith('Invalid video URL rejected:', 'javascript:alert(1)');
  });

  it('rejects javascript: URLs for audio', () => {
    display.show({ audioUrl: 'javascript:alert(1)' });

    const audio = display.getAudioElement();
    expect(audio).toBeFalsy();
    expect(consoleSpy).toHaveBeenCalledWith('Invalid audio URL rejected:', 'javascript:alert(1)');
  });

  it('rejects data: URLs for images', () => {
    display.show({ photoUrl: 'data:text/html,<script>alert(1)</script>' });

    const img = container.querySelector('img');
    expect(img).toBeFalsy();
  });

  it('accepts valid relative URLs', () => {
    display.show({ photoUrl: '/data/media/cat/photo.jpg' });

    const img = container.querySelector('img');
    expect(img).toBeTruthy();
    expect(img?.getAttribute('src')).toBe('/data/media/cat/photo.jpg');
  });

  it('accepts valid https URLs', () => {
    display.show({ photoUrl: 'https://example.com/cat.jpg' });

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
