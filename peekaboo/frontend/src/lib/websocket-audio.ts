/**
 * WebSocket Audio Client - Manages WebSocket connection for audio streaming
 *
 * Sends audio chunks in real-time during recording and receives transcripts/media.
 */

import { ApiError } from './errors';

// Message types from server
export interface TranscriptMessage {
  type: 'transcript';
  text: string;
}

export interface MediaMessage {
  type: 'media';
  subject: string;
  photo_url: string;
  audio_url?: string;
  video_url?: string;
}

export interface ErrorMessage {
  type: 'error';
  message: string;
}

export interface PongMessage {
  type: 'pong';
}

export type ServerMessage = TranscriptMessage | MediaMessage | ErrorMessage | PongMessage;

// Connection states
export type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';

export interface AudioWebSocketCallbacks {
  onTranscript?: (text: string) => void;
  onMedia?: (media: MediaMessage) => void;
  onError?: (error: Error) => void;
  onStateChange?: (state: ConnectionState) => void;
}

export interface AudioWebSocketOptions {
  /** WebSocket URL (default: derives from current location) */
  url?: string;
  /** Max reconnect attempts (default: 3) */
  maxReconnectAttempts?: number;
  /** Reconnect delay in ms (default: 1000, doubles each attempt) */
  reconnectDelay?: number;
  /** Ping interval in ms to keep connection alive (default: 30000) */
  pingInterval?: number;
}

const DEFAULT_OPTIONS: Required<AudioWebSocketOptions> = {
  url: '',
  maxReconnectAttempts: 3,
  reconnectDelay: 1000,
  pingInterval: 30000,
};

/**
 * AudioWebSocket manages WebSocket connection for streaming audio
 */
export class AudioWebSocket {
  private ws: WebSocket | null = null;
  private options: Required<AudioWebSocketOptions>;
  private callbacks: AudioWebSocketCallbacks;
  private state: ConnectionState = 'disconnected';
  private reconnectAttempts = 0;
  private pingTimer: ReturnType<typeof setInterval> | null = null;
  private isRecording = false;

  constructor(callbacks: AudioWebSocketCallbacks = {}, options: AudioWebSocketOptions = {}) {
    this.callbacks = callbacks;
    this.options = { ...DEFAULT_OPTIONS, ...options };
  }

  /**
   * Get the WebSocket URL, deriving from current location if not specified
   */
  private getWebSocketUrl(): string {
    if (this.options.url) {
      return this.options.url;
    }

    // Derive WebSocket URL from current page location
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}/ws/audio`;
  }

  /**
   * Get current connection state
   */
  getState(): ConnectionState {
    return this.state;
  }

  /**
   * Check if currently recording
   */
  getIsRecording(): boolean {
    return this.isRecording;
  }

  /**
   * Connect to WebSocket server
   */
  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.ws && this.state === 'connected') {
        resolve();
        return;
      }

      this.setState('connecting');
      const url = this.getWebSocketUrl();

      try {
        this.ws = new WebSocket(url);
      } catch (error) {
        this.setState('disconnected');
        reject(new ApiError('Failed to create WebSocket connection', 'network'));
        return;
      }

      this.ws.binaryType = 'arraybuffer';

      this.ws.onopen = () => {
        this.setState('connected');
        this.reconnectAttempts = 0;
        this.startPingTimer();
        resolve();
      };

      this.ws.onclose = (event) => {
        this.stopPingTimer();
        const wasRecording = this.isRecording;
        this.isRecording = false;

        if (this.state === 'connecting') {
          // Connection failed during initial connect
          this.setState('disconnected');
          reject(new ApiError('WebSocket connection failed', 'network'));
        } else if (!event.wasClean && this.state !== 'disconnected') {
          // Unexpected close, try to reconnect
          this.attemptReconnect(wasRecording);
        } else {
          this.setState('disconnected');
        }
      };

      this.ws.onerror = () => {
        // Error handler is called before onclose, so just log
        // The onclose handler will take care of state management
      };

      this.ws.onmessage = (event) => {
        this.handleMessage(event);
      };
    });
  }

  /**
   * Disconnect from WebSocket server
   */
  disconnect(): void {
    this.stopPingTimer();
    this.isRecording = false;
    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
      this.ws = null;
    }
    this.setState('disconnected');
  }

  /**
   * Start recording session - sends start_recording message to server
   */
  startRecording(): void {
    if (!this.ws || this.state !== 'connected') {
      throw new ApiError('WebSocket not connected', 'client');
    }

    this.isRecording = true;
    this.sendControlMessage('start_recording');
  }

  /**
   * Stop recording session - sends stop_recording message to server
   */
  stopRecording(): void {
    if (!this.ws || this.state !== 'connected') {
      return;
    }

    this.isRecording = false;
    this.sendControlMessage('stop_recording');
  }

  /**
   * Send an audio chunk to the server
   * @param chunk Audio data (Blob or ArrayBuffer)
   */
  async sendAudioChunk(chunk: Blob | ArrayBuffer): Promise<void> {
    if (!this.ws || this.state !== 'connected' || !this.isRecording) {
      return;
    }

    let data: ArrayBuffer;
    if (chunk instanceof Blob) {
      data = await chunk.arrayBuffer();
    } else {
      data = chunk;
    }

    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(data);
    }
  }

  /**
   * Send a ping to keep connection alive
   */
  ping(): void {
    if (!this.ws || this.state !== 'connected') {
      return;
    }
    this.sendControlMessage('ping');
  }

  private setState(state: ConnectionState): void {
    if (this.state !== state) {
      this.state = state;
      this.callbacks.onStateChange?.(state);
    }
  }

  private sendControlMessage(type: string): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type }));
    }
  }

  private handleMessage(event: MessageEvent): void {
    // Binary messages are not expected from server, ignore them
    if (event.data instanceof ArrayBuffer) {
      return;
    }

    try {
      const message = JSON.parse(event.data) as ServerMessage;

      switch (message.type) {
        case 'transcript':
          this.callbacks.onTranscript?.(message.text);
          break;

        case 'media':
          this.callbacks.onMedia?.(message);
          break;

        case 'error':
          this.callbacks.onError?.(new ApiError(message.message, 'server'));
          break;

        case 'pong':
          // Pong received, connection is alive
          break;

        default:
          // Unknown message type, ignore
          break;
      }
    } catch {
      // Invalid JSON, ignore
    }
  }

  private startPingTimer(): void {
    this.stopPingTimer();
    this.pingTimer = setInterval(() => {
      this.ping();
    }, this.options.pingInterval);
  }

  private stopPingTimer(): void {
    if (this.pingTimer) {
      clearInterval(this.pingTimer);
      this.pingTimer = null;
    }
  }

  private attemptReconnect(wasRecording: boolean): void {
    if (this.reconnectAttempts >= this.options.maxReconnectAttempts) {
      this.setState('disconnected');
      this.callbacks.onError?.(
        new ApiError('WebSocket connection lost after max reconnect attempts', 'network')
      );
      return;
    }

    this.setState('reconnecting');
    this.reconnectAttempts++;

    const delay = this.options.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);

    setTimeout(async () => {
      try {
        await this.connect();
        // Reconnected successfully
        // If we were recording, the recording session is lost
        // The caller should handle starting a new session if needed
        if (wasRecording) {
          this.callbacks.onError?.(
            new ApiError('Recording interrupted by connection loss', 'network')
          );
        }
      } catch {
        // Reconnect failed, attemptReconnect will be called again via onclose
      }
    }, delay);
  }
}

/**
 * Create an AudioWebSocket instance with default configuration
 */
export function createAudioWebSocket(
  callbacks: AudioWebSocketCallbacks = {},
  options: AudioWebSocketOptions = {}
): AudioWebSocket {
  return new AudioWebSocket(callbacks, options);
}
