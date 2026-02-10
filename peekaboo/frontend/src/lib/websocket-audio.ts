/**
 * WebSocket Audio Client - Manages WebSocket connection for audio streaming
 *
 * Sends audio chunks in real-time during recording and receives transcripts/media.
 */

import { ApiError } from './errors';
import { logger } from './logger';

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

export interface TTSAudioMessage {
  type: 'tts_audio';
  audio_data: string; // base64-encoded WAV audio
  text: string;       // the text that was spoken
}

export interface PongMessage {
  type: 'pong';
}

export type ServerMessage = TranscriptMessage | MediaMessage | TTSAudioMessage | ErrorMessage | PongMessage;

// Connection states
export type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';

export interface AudioWebSocketCallbacks {
  onTranscript?: (text: string) => void;
  onMedia?: (media: MediaMessage) => void;
  onTTSAudio?: (tts: TTSAudioMessage) => void;
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

// Client-to-server audio message sent as JSON text frame.
interface AudioDataMessage {
  type: 'audio_data';
  data: string;        // base64-encoded audio bytes
  seq: number;         // sequence number (wraps at 65535)
  client_time: number; // Date.now() in milliseconds
}

export class AudioWebSocket {
  private ws: WebSocket | null = null;
  private options: Required<AudioWebSocketOptions>;
  private callbacks: AudioWebSocketCallbacks;
  private state: ConnectionState = 'disconnected';
  private reconnectAttempts = 0;
  private pingTimer: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private isRecording = false;
  private sequenceNumber = 0;

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
    this.clearReconnectTimer();
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
    this.sequenceNumber = 0;
    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: 'start_recording', client_time: Date.now() }));
    }
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
   * Send an audio chunk to the server as a base64-encoded JSON message.
   *
   * Message format:
   *   {"type": "audio_data", "data": "<base64>", "seq": N, "client_time": <ms>}
   *
   * @param chunk Audio data (Blob or ArrayBuffer)
   */
  async sendAudioChunk(chunk: Blob | ArrayBuffer): Promise<void> {
    if (!this.ws || this.state !== 'connected' || !this.isRecording) {
      return;
    }

    let audioData: ArrayBuffer;
    if (chunk instanceof Blob) {
      audioData = await chunk.arrayBuffer();
    } else {
      audioData = chunk;
    }

    if (this.ws.readyState === WebSocket.OPEN) {
      // Convert audio bytes to base64 string using chunked approach
      // to avoid call stack limits and O(n^2) string concatenation
      const bytes = new Uint8Array(audioData);
      const chunkSize = 8192;
      const parts: string[] = [];
      for (let i = 0; i < bytes.length; i += chunkSize) {
        const slice = bytes.subarray(i, Math.min(i + chunkSize, bytes.length));
        parts.push(String.fromCharCode.apply(null, slice as unknown as number[]));
      }
      const base64Data = btoa(parts.join(''));

      const msg: AudioDataMessage = {
        type: 'audio_data',
        data: base64Data,
        seq: this.sequenceNumber & 0xFFFF,
        client_time: Date.now(),
      };

      this.sequenceNumber = (this.sequenceNumber + 1) & 0xFFFF;
      this.ws.send(JSON.stringify(msg));
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

        case 'tts_audio':
          this.callbacks.onTTSAudio?.(message);
          break;

        case 'error':
          this.callbacks.onError?.(new ApiError(message.message, 'client'));
          break;

        case 'pong':
          // Pong received, connection is alive
          break;

        default:
          // Unknown message type, ignore
          break;
      }
    } catch (error) {
      logger.debug('WebSocket received invalid JSON:', error);
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

  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
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

    this.reconnectTimer = setTimeout(async () => {
      this.reconnectTimer = null;
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
      } catch (error) {
        logger.debug('WebSocket reconnect attempt failed:', error);
        this.attemptReconnect(wasRecording);
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
