import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { AudioWebSocket } from './websocket-audio';

// Mock WebSocket for reconnection, message handling, and ping tests.
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

  close(code?: number, _reason?: string): void {
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

describe('AudioWebSocket message handling', () => {
  it('calls onTTSAudio callback for tts_audio messages', async () => {
    const onTTSAudio = vi.fn();
    const ws = new AudioWebSocket({ onTTSAudio });
    await connectWebSocket(ws);

    mockWebSocketInstance?.onmessage?.({
      data: JSON.stringify({
        type: 'tts_audio',
        audio_data: 'base64audiodata',
        text: 'hello',
      }),
    } as MessageEvent);

    expect(onTTSAudio).toHaveBeenCalledWith({
      type: 'tts_audio',
      audio_data: 'base64audiodata',
      text: 'hello',
    });
  });

  it('ignores binary messages from server', async () => {
    const onTranscript = vi.fn();
    const ws = new AudioWebSocket({ onTranscript });
    await connectWebSocket(ws);

    mockWebSocketInstance?.onmessage?.({
      data: new ArrayBuffer(10),
    } as MessageEvent);

    expect(onTranscript).not.toHaveBeenCalled();
  });

  it('ignores invalid JSON messages', async () => {
    const onError = vi.fn();
    const ws = new AudioWebSocket({ onError });
    await connectWebSocket(ws);

    mockWebSocketInstance?.onmessage?.({
      data: 'not valid json{',
    } as MessageEvent);

    expect(onError).not.toHaveBeenCalled();
  });
});

describe('AudioWebSocket reconnection', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('attempts reconnect on unclean close', async () => {
    const onStateChange = vi.fn();
    const ws = new AudioWebSocket({ onStateChange }, {
      url: 'wss://test/ws',
      maxReconnectAttempts: 3,
      reconnectDelay: 100,
    });
    await connectWebSocket(ws);
    onStateChange.mockClear();

    // Simulate unclean close
    mockWebSocketInstance?.onclose?.({ wasClean: false, code: 1006 });

    expect(ws.getState()).toBe('reconnecting');
    expect(onStateChange).toHaveBeenCalledWith('reconnecting');
  });

  it('gives up after max reconnect attempts', async () => {
    const onError = vi.fn();
    const ws = new AudioWebSocket({ onError }, {
      url: 'wss://test/ws',
      maxReconnectAttempts: 2,
      reconnectDelay: 100,
    });
    await connectWebSocket(ws);

    // First unclean close — starts reconnect attempt 1
    mockWebSocketInstance?.onclose?.({ wasClean: false, code: 1006 });
    expect(ws.getState()).toBe('reconnecting');

    // Advance timer for first retry attempt
    await vi.advanceTimersByTimeAsync(100);
    // connect() called, new MockWebSocket created — simulate failure
    // The onclose handler fires, which calls attemptReconnect for attempt 2
    mockWebSocketInstance?.simulateConnectionFailure();
    // Allow microtasks to flush
    await vi.advanceTimersByTimeAsync(0);

    // Advance timer for second retry attempt (200ms backoff)
    await vi.advanceTimersByTimeAsync(200);
    // connect() called again — simulate failure again
    mockWebSocketInstance?.simulateConnectionFailure();
    await vi.advanceTimersByTimeAsync(0);

    // Max attempts reached — should be disconnected with error
    expect(ws.getState()).toBe('disconnected');
    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({ message: expect.stringContaining('max reconnect attempts') })
    );
  });

  it('resets reconnect counter on successful reconnect', async () => {
    const onStateChange = vi.fn();
    const ws = new AudioWebSocket({ onStateChange }, {
      url: 'wss://test/ws',
      maxReconnectAttempts: 3,
      reconnectDelay: 100,
    });
    await connectWebSocket(ws);

    // Unclean close
    mockWebSocketInstance?.onclose?.({ wasClean: false, code: 1006 });
    expect(ws.getState()).toBe('reconnecting');

    // Advance timer and simulate successful reconnect
    await vi.advanceTimersByTimeAsync(100);
    mockWebSocketInstance?.simulateOpen();

    expect(ws.getState()).toBe('connected');
  });

  it('reports recording interrupted on reconnect if was recording', async () => {
    const onError = vi.fn();
    const ws = new AudioWebSocket({ onError }, {
      url: 'wss://test/ws',
      maxReconnectAttempts: 3,
      reconnectDelay: 100,
    });
    await connectWebSocket(ws);

    ws.startRecording();
    expect(ws.getIsRecording()).toBe(true);

    // Unclean close while recording
    mockWebSocketInstance?.onclose?.({ wasClean: false, code: 1006 });
    expect(ws.getIsRecording()).toBe(false);

    // Advance timer and simulate successful reconnect
    await vi.advanceTimersByTimeAsync(100);
    mockWebSocketInstance?.simulateOpen();
    // Allow microtasks from async reconnect callback to complete
    await vi.advanceTimersByTimeAsync(0);

    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({ message: expect.stringContaining('Recording interrupted') })
    );
  });

  it('does not reconnect on clean close', async () => {
    const onStateChange = vi.fn();
    const ws = new AudioWebSocket({ onStateChange }, {
      url: 'wss://test/ws',
      maxReconnectAttempts: 3,
    });
    await connectWebSocket(ws);
    onStateChange.mockClear();

    // Clean close
    mockWebSocketInstance?.onclose?.({ wasClean: true, code: 1000 });

    expect(ws.getState()).toBe('disconnected');
    expect(onStateChange).toHaveBeenCalledWith('disconnected');
    expect(onStateChange).not.toHaveBeenCalledWith('reconnecting');
  });

  it('cancels pending reconnect on disconnect', async () => {
    const ws = new AudioWebSocket({}, {
      url: 'wss://test/ws',
      maxReconnectAttempts: 3,
      reconnectDelay: 5000,
    });
    await connectWebSocket(ws);

    // Unclean close
    mockWebSocketInstance?.onclose?.({ wasClean: false, code: 1006 });
    expect(ws.getState()).toBe('reconnecting');

    // User calls disconnect before timer fires
    ws.disconnect();
    expect(ws.getState()).toBe('disconnected');

    // Advance past the reconnect delay — nothing should happen
    await vi.advanceTimersByTimeAsync(10000);
    expect(ws.getState()).toBe('disconnected');
  });
});

describe('AudioWebSocket ping', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('sends ping message when connected', async () => {
    const ws = new AudioWebSocket({}, { url: 'wss://test/ws' });
    await connectWebSocket(ws);

    ws.ping();

    const messages = mockWebSocketInstance?.sentMessages || [];
    expect(JSON.parse(messages[0] as string)).toEqual({ type: 'ping' });
  });

  it('does nothing when not connected', () => {
    const ws = new AudioWebSocket();
    ws.ping(); // Should not throw
  });

  it('starts ping timer on connect', async () => {
    const ws = new AudioWebSocket({}, {
      url: 'wss://test/ws',
      pingInterval: 1000,
    });
    await connectWebSocket(ws);

    // No pings yet
    const initialMessages = mockWebSocketInstance?.sentMessages.length || 0;

    // Advance past one ping interval
    await vi.advanceTimersByTimeAsync(1000);

    const messagesAfterPing = mockWebSocketInstance?.sentMessages || [];
    const pingMsg = JSON.parse(messagesAfterPing[initialMessages] as string);
    expect(pingMsg).toEqual({ type: 'ping' });
  });

  it('stops ping timer on disconnect', async () => {
    const ws = new AudioWebSocket({}, {
      url: 'wss://test/ws',
      pingInterval: 1000,
    });
    await connectWebSocket(ws);

    ws.disconnect();

    const messageCount = mockWebSocketInstance?.sentMessages.length || 0;

    // Advance past several ping intervals
    await vi.advanceTimersByTimeAsync(5000);

    // No new messages should have been sent (timer stopped)
    expect(mockWebSocketInstance?.sentMessages.length || 0).toBe(messageCount);
  });
});
