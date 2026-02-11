import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  AudioWebSocket,
} from './websocket-audio';

// Mock WebSocket for connection and recording tests.
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

  simulateConnectionFailure(): void {
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

      expect(ws.getState()).toBe('connecting');
      expect(onStateChange).toHaveBeenCalledWith('connecting');

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

      mockWebSocketInstance?.simulateConnectionFailure();

      await expect(connectPromise).rejects.toThrow('WebSocket connection failed');
      expect(ws.getState()).toBe('disconnected');
    });

    it('rejects with timeout if server never responds', async () => {
      vi.useFakeTimers();
      try {
        const onStateChange = vi.fn();
        const ws = new AudioWebSocket({ onStateChange }, { connectTimeout: 5000 });
        const connectPromise = ws.connect();

        expect(ws.getState()).toBe('connecting');

        // Advance past the timeout
        vi.advanceTimersByTime(5000);

        await expect(connectPromise).rejects.toThrow('WebSocket connection timeout');
        expect(ws.getState()).toBe('disconnected');
      } finally {
        vi.useRealTimers();
      }
    });

    it('clears timeout on successful connection', async () => {
      vi.useFakeTimers();
      try {
        const ws = new AudioWebSocket({}, { connectTimeout: 5000 });
        const connectPromise = ws.connect();

        // Connect before timeout
        mockWebSocketInstance?.simulateOpen();
        await connectPromise;

        expect(ws.getState()).toBe('connected');

        // Advance past what would have been the timeout
        vi.advanceTimersByTime(10000);

        // Should still be connected (timeout didn't fire)
        expect(ws.getState()).toBe('connected');
      } finally {
        vi.useRealTimers();
      }
    });

    it('clears timeout on connection failure', async () => {
      vi.useFakeTimers();
      try {
        const ws = new AudioWebSocket({}, { connectTimeout: 5000 });
        const connectPromise = ws.connect();

        // Connection fails before timeout
        mockWebSocketInstance?.simulateConnectionFailure();
        await expect(connectPromise).rejects.toThrow('WebSocket connection failed');

        expect(ws.getState()).toBe('disconnected');
      } finally {
        vi.useRealTimers();
      }
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
    it('sends start_recording message with client_time', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      const beforeTime = Date.now();
      ws.startRecording();
      const afterTime = Date.now();

      const messages = mockWebSocketInstance?.sentMessages || [];
      const msg = JSON.parse(messages[0] as string);
      expect(msg.type).toBe('start_recording');
      expect(msg.client_time).toBeGreaterThanOrEqual(beforeTime);
      expect(msg.client_time).toBeLessThanOrEqual(afterTime);
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
    it('sends base64-encoded JSON message while recording', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      ws.startRecording();

      const testData = new TextEncoder().encode('test audio data');
      const beforeTime = Date.now();
      await ws.sendAudioChunk(testData.buffer);
      const afterTime = Date.now();

      const messages = mockWebSocketInstance?.sentMessages || [];
      expect(messages.length).toBe(2);

      // Message should be a JSON string, not an ArrayBuffer
      expect(typeof messages[1]).toBe('string');
      const msg = JSON.parse(messages[1] as string);
      expect(msg.type).toBe('audio_data');
      expect(msg.seq).toBe(0);
      expect(msg.client_time).toBeGreaterThanOrEqual(beforeTime);
      expect(msg.client_time).toBeLessThanOrEqual(afterTime);

      // Verify base64 data decodes to the original audio bytes
      const decoded = Uint8Array.from(atob(msg.data), c => c.charCodeAt(0));
      expect(Array.from(decoded)).toEqual(Array.from(testData));
    });

    it('increments sequence number for each chunk', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      ws.startRecording();

      const chunk1 = new Uint8Array([1, 2, 3]).buffer;
      const chunk2 = new Uint8Array([4, 5, 6]).buffer;
      const chunk3 = new Uint8Array([7, 8, 9]).buffer;

      await ws.sendAudioChunk(chunk1);
      await ws.sendAudioChunk(chunk2);
      await ws.sendAudioChunk(chunk3);

      const messages = mockWebSocketInstance?.sentMessages || [];
      expect(messages.length).toBe(4);

      expect(JSON.parse(messages[1] as string).seq).toBe(0);
      expect(JSON.parse(messages[2] as string).seq).toBe(1);
      expect(JSON.parse(messages[3] as string).seq).toBe(2);
    });

    it('resets sequence number on new recording session', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

      ws.startRecording();
      await ws.sendAudioChunk(new Uint8Array([1]).buffer);
      await ws.sendAudioChunk(new Uint8Array([2]).buffer);
      ws.stopRecording();

      ws.startRecording();
      await ws.sendAudioChunk(new Uint8Array([3]).buffer);

      const messages = mockWebSocketInstance?.sentMessages || [];
      expect(messages.length).toBe(6);

      const lastMsg = JSON.parse(messages[5] as string);
      expect(lastMsg.seq).toBe(0);
    });

    it('does not send if not recording', async () => {
      const ws = new AudioWebSocket();
      await connectWebSocket(ws);

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
});
