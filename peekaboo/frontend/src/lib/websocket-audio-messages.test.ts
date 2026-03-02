import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  AudioWebSocket,
  createAudioWebSocket,
  MediaMessage,
} from './websocket-audio';
import { ApiError } from './errors';

// Mock WebSocket
class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  readyState = MockWebSocket.CONNECTING;
  binaryType = 'blob';
  url: string;

  onopen: (() => void) | null = null;
  onclose: ((event: { wasClean: boolean; code: number }) => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((event: { data: string | ArrayBuffer }) => void) | null = null;

  sentMessages: (string | ArrayBuffer)[] = [];

  constructor(url: string) {
    this.url = url;
  }

  simulateOpen(): void {
    if (this.readyState === MockWebSocket.CONNECTING) {
      this.readyState = MockWebSocket.OPEN;
      this.onopen?.();
    }
  }

  send(data: string | ArrayBuffer): void {
    if (this.readyState !== MockWebSocket.OPEN) {
      throw new Error('WebSocket is not open');
    }
    this.sentMessages.push(data);
  }

  close(code?: number): void {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.({ wasClean: true, code: code || 1000 });
  }

  simulateMessage(data: string | ArrayBuffer): void {
    this.onmessage?.({ data });
  }

  simulateUnexpectedClose(): void {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.({ wasClean: false, code: 1006 });
  }
}

let mockWebSocketInstance: MockWebSocket | null = null;

async function connectWebSocket(ws: AudioWebSocket): Promise<void> {
  const connectPromise = ws.connect();
  mockWebSocketInstance?.simulateOpen();
  await connectPromise;
}

beforeEach(() => {
  mockWebSocketInstance = null;

  const MockWebSocketClass = class extends MockWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;

    constructor(url: string) {
      super(url);
      mockWebSocketInstance = this;
    }
  };

  // @ts-expect-error - mocking global
  global.WebSocket = MockWebSocketClass;

  Object.defineProperty(global, 'window', {
    value: {
      location: {
        protocol: 'https:',
        host: 'localhost:4321',
      },
    },
    writable: true,
    configurable: true,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  mockWebSocketInstance = null;
});

describe('AudioWebSocket message handling', () => {
  it('calls onTranscript for transcript messages', async () => {
    const onTranscript = vi.fn();
    const ws = new AudioWebSocket({ onTranscript });
    await connectWebSocket(ws);

    mockWebSocketInstance?.simulateMessage(JSON.stringify({
      type: 'transcript',
      text: 'show me a cat',
    }));

    expect(onTranscript).toHaveBeenCalledWith('show me a cat');
  });

  it('calls onMedia for media messages', async () => {
    const onMedia = vi.fn();
    const ws = new AudioWebSocket({ onMedia });
    await connectWebSocket(ws);

    const mediaMsg: MediaMessage = {
      type: 'media',
      subject: 'cat',
      photo_url: '/data/media/cat/set1/photo.jpg',
      audio_url: '/data/media/cat/set1/audio.mp3',
    };
    mockWebSocketInstance?.simulateMessage(JSON.stringify(mediaMsg));

    expect(onMedia).toHaveBeenCalledWith(mediaMsg);
  });

  it('calls onTTSAudio for tts_audio messages', async () => {
    const onTTSAudio = vi.fn();
    const ws = new AudioWebSocket({ onTTSAudio });
    await connectWebSocket(ws);

    const ttsMsg = {
      type: 'tts_audio' as const,
      audio_data: 'dGVzdA==', // base64 "test"
      text: 'Here is a cat!',
    };
    mockWebSocketInstance?.simulateMessage(JSON.stringify(ttsMsg));

    expect(onTTSAudio).toHaveBeenCalledWith(ttsMsg);
  });

  it('calls onError for error messages', async () => {
    const onError = vi.fn();
    const ws = new AudioWebSocket({ onError });
    await connectWebSocket(ws);

    mockWebSocketInstance?.simulateMessage(JSON.stringify({
      type: 'error',
      message: 'transcription failed',
    }));

    expect(onError).toHaveBeenCalled();
    const error = onError.mock.calls[0][0];
    expect(error).toBeInstanceOf(ApiError);
    expect(error.message).toBe('transcription failed');
  });

  it('handles pong messages silently', async () => {
    const onError = vi.fn();
    const ws = new AudioWebSocket({ onError });
    await connectWebSocket(ws);

    mockWebSocketInstance?.simulateMessage(JSON.stringify({ type: 'pong' }));

    expect(onError).not.toHaveBeenCalled();
  });

  it('ignores invalid JSON', async () => {
    const onError = vi.fn();
    const ws = new AudioWebSocket({ onError });
    await connectWebSocket(ws);

    mockWebSocketInstance?.simulateMessage('not valid json');

    expect(onError).not.toHaveBeenCalled();
  });

  it('ignores binary messages', async () => {
    const onTranscript = vi.fn();
    const ws = new AudioWebSocket({ onTranscript });
    await connectWebSocket(ws);

    mockWebSocketInstance?.simulateMessage(new ArrayBuffer(10));

    expect(onTranscript).not.toHaveBeenCalled();
  });
});

describe('AudioWebSocket reconnection', () => {
  it('attempts reconnect on unexpected close', async () => {
    const onStateChange = vi.fn();
    const ws = new AudioWebSocket({ onStateChange }, { reconnectDelay: 100 });
    await connectWebSocket(ws);

    mockWebSocketInstance?.simulateUnexpectedClose();

    expect(ws.getState()).toBe('reconnecting');
    expect(onStateChange).toHaveBeenCalledWith('reconnecting');
  });

  it('gives up after max attempts', async () => {
    const onError = vi.fn();
    const onStateChange = vi.fn();
    const ws = new AudioWebSocket({ onError, onStateChange }, {
      reconnectDelay: 10,
      maxReconnectAttempts: 0, // Give up immediately
    });
    await connectWebSocket(ws);

    mockWebSocketInstance?.simulateUnexpectedClose();

    expect(ws.getState()).toBe('disconnected');
    expect(onError).toHaveBeenCalled();
    expect(onError.mock.calls[0][0].message).toContain('max reconnect attempts');
  });

  it('clears reconnect timer on disconnect', async () => {
    vi.useFakeTimers();
    const onStateChange = vi.fn();
    const ws = new AudioWebSocket({ onStateChange }, {
      reconnectDelay: 5000,
      maxReconnectAttempts: 3,
    });
    await connectWebSocket(ws);

    mockWebSocketInstance?.simulateUnexpectedClose();
    expect(ws.getState()).toBe('reconnecting');

    ws.disconnect();
    expect(ws.getState()).toBe('disconnected');

    vi.advanceTimersByTime(10000);
    expect(ws.getState()).toBe('disconnected');
    vi.useRealTimers();
  });

  it('does not reconnect on clean close', async () => {
    const onStateChange = vi.fn();
    const ws = new AudioWebSocket({ onStateChange });
    await connectWebSocket(ws);

    ws.disconnect();

    expect(ws.getState()).toBe('disconnected');
    const states = onStateChange.mock.calls.map(call => call[0]);
    expect(states).not.toContain('reconnecting');
  });
});

describe('AudioWebSocket ping', () => {
  it('sends ping message', async () => {
    const ws = new AudioWebSocket();
    await connectWebSocket(ws);

    ws.ping();

    const messages = mockWebSocketInstance?.sentMessages || [];
    expect(JSON.parse(messages[0] as string)).toEqual({ type: 'ping' });
  });

  it('does nothing if not connected', () => {
    const ws = new AudioWebSocket();
    ws.ping(); // Should not throw
  });
});

describe('createAudioWebSocket', () => {
  it('creates AudioWebSocket instance', () => {
    const ws = createAudioWebSocket();
    expect(ws).toBeInstanceOf(AudioWebSocket);
  });

  it('passes callbacks and options', () => {
    const onTranscript = vi.fn();
    const ws = createAudioWebSocket(
      { onTranscript },
      { url: 'wss://test.host/ws' }
    );
    expect(ws).toBeInstanceOf(AudioWebSocket);
  });
});
