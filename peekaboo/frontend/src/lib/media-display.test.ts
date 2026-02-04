import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { MediaDisplay, fetchMedia } from './media-display';

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
      json: vi.fn().mockResolvedValue({
        photo_url: '/data/media/cat/photo.jpg',
        audio_url: '/data/media/cat/audio.mp3',
      }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const result = await fetchMedia('cat');

    expect(global.fetch).toHaveBeenCalledWith('/api/media/cat');
    expect(result).toEqual({
      photoUrl: '/data/media/cat/photo.jpg',
      videoUrl: undefined,
      audioUrl: '/data/media/cat/audio.mp3',
    });
  });

  it('encodes concept parameter', async () => {
    const mockResponse = {
      ok: true,
      json: vi.fn().mockResolvedValue({ photo_url: '/photo.jpg' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await fetchMedia('my cat');

    expect(global.fetch).toHaveBeenCalledWith('/api/media/my%20cat');
  });

  it('throws error on failed response', async () => {
    const mockResponse = {
      ok: false,
      status: 404,
      json: vi.fn().mockResolvedValue({ error: 'concept not found' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await expect(fetchMedia('elephant')).rejects.toThrow('concept not found');
  });

  it('throws error when response contains error field', async () => {
    const mockResponse = {
      ok: true,
      json: vi.fn().mockResolvedValue({ error: 'no media found for concept' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    await expect(fetchMedia('cat')).rejects.toThrow('no media found for concept');
  });

  it('includes video_url when available', async () => {
    const mockResponse = {
      ok: true,
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
