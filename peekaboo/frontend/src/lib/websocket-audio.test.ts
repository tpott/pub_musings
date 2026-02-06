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
  closeCode?: number;
  closeReason?: string;

  constructor(url: string) {
    this.url = url;
  }

  // Call this to simulate successful connection
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

  close(code?: number, reason?: string): void {
    this.closeCode = code;
    this.closeReason = reason;
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.({ wasClean: true, code: code || 1000 });
  }

  // Test helpers
  simulateMessage(data: string | ArrayBuffer): void {
    this.onmessage?.({ data });
  }

  simulateError(): void {
    this.onerror?.();
  }

  simulateUnexpectedClose(): void {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.({ wasClean: false, code: 1006 });
  }

  simulateConnectionFailure(): void {
    // Simulate connection failure before onopen
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.({ wasClean: false, code: 1006 });
  }
}

let mockWebSocketInstance: MockWebSocket | null = null;

// Helper to connect and wait
async function connectWebSocket(ws: AudioWebSocket): Promise<void> {
  const connectPromise = ws.connect();
  // Synchronously simulate open after connect() sets up handlers
  mockWebSocketInstance?.simulateOpen();
  await connectPromise;
}

beforeEach(() => {
  mockWebSocketInstance = null;

  // Mock WebSocket constructor with static constants
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

  // Mock window.location
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

describe('AudioWebSocket', () => {
  describe('constructor', () => {
    it('initializes with default options', () => {
      const ws = new AudioWebSocket();
      expect(ws.getState()).toBe('disconnected');
      expect(ws.getIsRecording()).toBe(false);
    });

    it('accepts custom options', () => {
      const ws = new AudioWebSocket({}, {
        url: 'wss://custom.host/ws',
        maxReconnectAttempts: 5,
      });
      expect(ws.getState()).toBe('disconnected');
    });
  });

  describe('connect', () => {
    it('establishes WebSocket connection', async () => {
      const onStateChange = vi.fn();
      const ws = new AudioWebSocket({ onStateChange });

      const connectPromise = ws.connect();

      // Should be connecting
      expect(ws.getState()).toBe('connecting');
      expect(onStateChange).toHaveBeenCalledWith('connecting');

      // Simulate successful connection
      mockWebSocketInstance?.simulateOpen();
      await connectPromise;

      expect(ws.getState()).toBe('connected');
      expect(onStateChange).toHaveBeenCalledWith('connected');
    });

    it('derives WebSocket URL from window.location', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      expect(mockWebSocketInstance?.url).toBe('wss://localhost:4321/ws/audio');
    });

    it('uses http to ws protocol conversion', async () => {
      window.location.protocol = 'http:';
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      expect(mockWebSocketInstance?.url).toBe('ws://localhost:4321/ws/audio');
    });

    it('uses custom URL when provided', async () => {
      const ws = new AudioWebSocket({}, { url: 'wss://custom.host/audio' });
      await connectWebSocket(ws);

      expect(mockWebSocketInstance?.url).toBe('wss://custom.host/audio');
    });

    it('resolves immediately if already connected', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      // Second connect should resolve immediately
      await ws.connect();
      expect(ws.getState()).toBe('connected');
    });

    it('sets binaryType to arraybuffer', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      expect(mockWebSocketInstance?.binaryType).toBe('arraybuffer');
    });

    it('rejects if connection fails', async () => {
      const ws = new AudioWebSocket();
      const connectPromise = ws.connect();

      // Simulate connection failure
      mockWebSocketInstance?.simulateConnectionFailure();

      await expect(connectPromise).rejects.toThrow('WebSocket connection failed');
      expect(ws.getState()).toBe('disconnected');
    });
  });

  describe('disconnect', () => {
    it('closes WebSocket connection', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      ws.disconnect();

      expect(mockWebSocketInstance?.closeCode).toBe(1000);
      expect(ws.getState()).toBe('disconnected');
    });

    it('resets recording state', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      ws.startRecording();
      expect(ws.getIsRecording()).toBe(true);

      ws.disconnect();
      expect(ws.getIsRecording()).toBe(false);
    });
  });

  describe('startRecording', () => {
    it('sends start_recording message', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      ws.startRecording();

      const messages = mockWebSocketInstance?.sentMessages || [];
      expect(JSON.parse(messages[0] as string)).toEqual({ type: 'start_recording' });
      expect(ws.getIsRecording()).toBe(true);
    });

    it('throws if not connected', () => {
      const ws = new AudioWebSocket();
      expect(() => ws.startRecording()).toThrow('WebSocket not connected');
    });
  });

  describe('stopRecording', () => {
    it('sends stop_recording message', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      ws.startRecording();
      ws.stopRecording();

      const messages = mockWebSocketInstance?.sentMessages || [];
      expect(JSON.parse(messages[1] as string)).toEqual({ type: 'stop_recording' });
      expect(ws.getIsRecording()).toBe(false);
    });

    it('does nothing if not connected', () => {
      const ws = new AudioWebSocket();
      ws.stopRecording(); // Should not throw
    });
  });

  describe('sendAudioChunk', () => {
    it('sends ArrayBuffer while recording', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      ws.startRecording();

      // Create a real ArrayBuffer to send
      const testData = new TextEncoder().encode('test audio data');
      await ws.sendAudioChunk(testData.buffer);

      const messages = mockWebSocketInstance?.sentMessages || [];
      // First message is start_recording, second is the audio chunk
      expect(messages.length).toBe(2);
      expect(messages[1]).toBe(testData.buffer);
    });

    it('does not send if not recording', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      // Not recording
      const blob = new Blob(['test']);
      await ws.sendAudioChunk(blob);

      const messages = mockWebSocketInstance?.sentMessages || [];
      expect(messages.length).toBe(0);
    });

    it('does not send if not connected', async () => {
      const ws = new AudioWebSocket();
      const blob = new Blob(['test']);
      await ws.sendAudioChunk(blob); // Should not throw
    });
  });

  describe('message handling', () => {
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

      // No error should be called for valid pong
      expect(onError).not.toHaveBeenCalled();
    });

    it('ignores invalid JSON', async () => {
      const onError = vi.fn();
      const ws = new AudioWebSocket({ onError });
      await connectWebSocket(ws);

      mockWebSocketInstance?.simulateMessage('not valid json');

      // Should not call onError for parse errors
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

  describe('reconnection', () => {
    it('attempts reconnect on unexpected close', async () => {
      const onStateChange = vi.fn();
      const ws = new AudioWebSocket({ onStateChange }, { reconnectDelay: 100 });
      await connectWebSocket(ws);

      // Simulate unexpected close
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

      // Unexpected close - should give up immediately since max is 0
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

      // Simulate unexpected close - triggers reconnect timer
      mockWebSocketInstance?.simulateUnexpectedClose();
      expect(ws.getState()).toBe('reconnecting');

      // Disconnect before the reconnect timer fires
      ws.disconnect();
      expect(ws.getState()).toBe('disconnected');

      // Advance past the reconnect delay - timer should have been cleared
      vi.advanceTimersByTime(10000);

      // State should still be disconnected (no reconnect happened)
      expect(ws.getState()).toBe('disconnected');
      vi.useRealTimers();
    });

    it('does not reconnect on clean close', async () => {
      const onStateChange = vi.fn();
      const ws = new AudioWebSocket({ onStateChange });
      await connectWebSocket(ws);

      // Clean close via disconnect
      ws.disconnect();

      expect(ws.getState()).toBe('disconnected');
      // Should not have gone through reconnecting state
      const states = onStateChange.mock.calls.map(call => call[0]);
      expect(states).not.toContain('reconnecting');
    });
  });

  describe('ping', () => {
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
