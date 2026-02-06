import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AudioRecorder, transcribeAudio } from './audio-recorder';

// Mock MediaRecorder
class MockMediaRecorder {
  state: 'inactive' | 'recording' | 'paused' = 'inactive';
  mimeType: string;
  ondataavailable: ((event: { data: Blob }) => void) | null = null;
  onstop: (() => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(stream: MediaStream, options?: { mimeType?: string }) {
    this.mimeType = options?.mimeType || 'audio/webm';
  }

  start() {
    this.state = 'recording';
  }

  stop() {
    this.state = 'inactive';
    // Simulate data available
    if (this.ondataavailable) {
      const mockAudioData = new Blob(['mock audio data'], { type: this.mimeType });
      this.ondataavailable({ data: mockAudioData });
    }
    // Trigger onstop after data available
    setTimeout(() => {
      if (this.onstop) this.onstop();
    }, 0);
  }

  static isTypeSupported(mimeType: string): boolean {
    return mimeType === 'audio/webm' || mimeType === 'audio/ogg';
  }
}

// Mock MediaStream
class MockMediaStream {
  private tracks: { stop: () => void }[] = [{ stop: vi.fn() }];

  getTracks() {
    return this.tracks;
  }
}

// Setup global mocks
beforeEach(() => {
  // @ts-expect-error - mocking global
  global.MediaRecorder = MockMediaRecorder;

  // Mock navigator.mediaDevices
  const mockGetUserMedia = vi.fn().mockResolvedValue(new MockMediaStream());
  Object.defineProperty(global, 'navigator', {
    value: {
      mediaDevices: {
        getUserMedia: mockGetUserMedia,
      },
    },
    writable: true,
    configurable: true,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('AudioRecorder', () => {
  describe('isSupported', () => {
    it('returns true when MediaRecorder is available', () => {
      expect(AudioRecorder.isSupported()).toBe(true);
    });

    it('returns false when MediaRecorder is not available', () => {
      // @ts-expect-error - testing undefined case
      global.MediaRecorder = undefined;
      expect(AudioRecorder.isSupported()).toBe(false);
    });
  });

  describe('startRecording', () => {
    it('requests microphone access', async () => {
      const recorder = new AudioRecorder();
      await recorder.startRecording();

      expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledWith({ audio: true });
    });

    it('creates MediaRecorder with webm mime type', async () => {
      const recorder = new AudioRecorder();
      await recorder.startRecording();

      expect(recorder.isRecording()).toBe(true);
    });

    it('does nothing if already recording', async () => {
      const recorder = new AudioRecorder();
      await recorder.startRecording();
      await recorder.startRecording(); // Second call should be ignored

      expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledTimes(1);
    });
  });

  describe('stopRecording', () => {
    it('returns audio blob after stopping', async () => {
      const recorder = new AudioRecorder();
      await recorder.startRecording();

      const result = await recorder.stopRecording();

      expect(result.blob).toBeInstanceOf(Blob);
      expect(result.mimeType).toBe('audio/webm');
    });

    it('throws error when not recording', async () => {
      const recorder = new AudioRecorder();

      await expect(recorder.stopRecording()).rejects.toThrow('Not recording');
    });

    it('cleans up stream tracks after stopping', async () => {
      const recorder = new AudioRecorder();
      await recorder.startRecording();
      await recorder.stopRecording();

      expect(recorder.isRecording()).toBe(false);
    });
  });

  describe('destroy', () => {
    it('stops recording and releases resources', async () => {
      const recorder = new AudioRecorder();
      await recorder.startRecording();
      expect(recorder.isRecording()).toBe(true);

      recorder.destroy();
      expect(recorder.isRecording()).toBe(false);
    });

    it('does nothing if not recording', () => {
      const recorder = new AudioRecorder();
      recorder.destroy(); // Should not throw
      expect(recorder.isRecording()).toBe(false);
    });
  });

  describe('isRecording', () => {
    it('returns false when not recording', () => {
      const recorder = new AudioRecorder();
      expect(recorder.isRecording()).toBe(false);
    });

    it('returns true when recording', async () => {
      const recorder = new AudioRecorder();
      await recorder.startRecording();
      expect(recorder.isRecording()).toBe(true);
    });

    it('returns false after stopping', async () => {
      const recorder = new AudioRecorder();
      await recorder.startRecording();
      await recorder.stopRecording();
      expect(recorder.isRecording()).toBe(false);
    });
  });
});

describe('transcribeAudio', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  it('sends audio blob to /api/transcribe', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({ text: 'show me a cat' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const blob = new Blob(['audio data'], { type: 'audio/webm' });
    const result = await transcribeAudio(blob);

    expect(global.fetch).toHaveBeenCalledWith('/api/transcribe', expect.objectContaining({
      method: 'POST',
    }));

    // Verify FormData was sent
    const fetchCall = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const formData = fetchCall[1].body as FormData;
    expect(formData).toBeInstanceOf(FormData);
    expect(formData.get('audio')).toBeInstanceOf(Blob);

    expect(result).toBe('show me a cat');
  });

  it('throws error on failed response', async () => {
    // Use 400 which is not retried by fetchWithRetry
    const mockResponse = {
      ok: false,
      status: 400,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({ error: 'Bad request' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const blob = new Blob(['audio data'], { type: 'audio/webm' });

    await expect(transcribeAudio(blob)).rejects.toThrow('Bad request');
  });

  it('throws server error when response is not valid JSON', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockRejectedValue(new SyntaxError('Unexpected token')),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const blob = new Blob(['audio data'], { type: 'audio/webm' });

    await expect(transcribeAudio(blob)).rejects.toThrow('Transcription failed: invalid response');
  });

  it('throws error when response contains error field', async () => {
    const mockResponse = {
      ok: true,
      headers: new Headers(),
      json: vi.fn().mockResolvedValue({ error: 'Transcription failed' }),
    };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const blob = new Blob(['audio data'], { type: 'audio/webm' });

    await expect(transcribeAudio(blob)).rejects.toThrow('Transcription failed');
  });
});
