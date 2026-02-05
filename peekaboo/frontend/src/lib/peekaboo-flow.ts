/**
 * Peekaboo Flow - Orchestrates the full voice-to-media flow
 *
 * Supports two modes:
 * - Legacy (HTTP): button press -> record audio -> transcribe -> intent -> fetch media -> display
 * - WebSocket: continuous streaming -> media displays while mic stays active
 */

import { AudioRecorder, transcribeAudio } from './audio-recorder';
import { extractIntent } from './intent';
import { MediaDisplay, fetchMedia } from './media-display';
import { getUserFriendlyMessage, ApiError } from './errors';
import { speakSubject } from './text-to-speech';
import { AudioWebSocket } from './websocket-audio';
import type { MediaMessage, ConnectionState } from './websocket-audio';

export type FlowState = 'idle' | 'recording' | 'transcribing' | 'searching' | 'displaying' | 'error';

export interface PeekabooFlowOptions {
  micButton: HTMLButtonElement;
  mediaContainer: HTMLElement;
  onStateChange?: (state: FlowState) => void;
  onError?: (error: Error) => void;
  /** Use WebSocket for audio streaming (default: false for backward compatibility) */
  useWebSocket?: boolean;
  /** WebSocket URL (optional, derived from page location if not specified) */
  webSocketUrl?: string;
}

/**
 * PeekabooFlow handles the complete voice command flow
 */
export class PeekabooFlow {
  private recorder: AudioRecorder;
  private display: MediaDisplay;
  private micButton: HTMLButtonElement;
  private state: FlowState = 'idle';
  private onStateChange?: (state: FlowState) => void;
  private onError?: (error: Error) => void;

  // WebSocket mode
  private useWebSocket: boolean;
  private wsClient: AudioWebSocket | null = null;
  private stream: MediaStream | null = null;
  private mediaRecorder: MediaRecorder | null = null;

  constructor(options: PeekabooFlowOptions) {
    this.recorder = new AudioRecorder();
    this.display = new MediaDisplay(options.mediaContainer);
    this.micButton = options.micButton;
    this.onStateChange = options.onStateChange;
    this.onError = options.onError;
    this.useWebSocket = options.useWebSocket ?? false;

    if (this.useWebSocket) {
      this.wsClient = new AudioWebSocket(
        {
          onTranscript: (text) => this.handleWsTranscript(text),
          onMedia: (media) => this.handleWsMedia(media),
          onError: (error) => this.handleError(error),
          onStateChange: (wsState) => this.handleWsStateChange(wsState),
        },
        { url: options.webSocketUrl }
      );
    }

    this.setupEventListeners();
    this.updateUI(); // Initialize accessibility attributes
  }

  private setupEventListeners(): void {
    // Toggle recording on click - click to start, click again to stop
    this.micButton.addEventListener('click', (e: MouseEvent) => {
      e.preventDefault();
      this.toggleRecording();
    });

    // Touch events - use touchend to toggle (prevents double-firing with click on some devices)
    this.micButton.addEventListener('touchend', (e: TouchEvent) => {
      e.preventDefault(); // Prevent click event from also firing
      this.toggleRecording();
    });

    // Keyboard events - Enter and Space toggle recording
    this.micButton.addEventListener('keydown', (e: KeyboardEvent) => {
      // Ignore repeated keydown events (key held down)
      if (e.repeat) return;

      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault(); // Prevent scrolling on Space, form submission on Enter
        this.toggleRecording();
      }
    });
  }

  /**
   * Toggle recording state - start if idle, stop if recording
   */
  private toggleRecording(): void {
    if (this.state === 'recording') {
      this.stopRecordingAndProcess();
    } else if (this.state === 'idle' || this.state === 'displaying' || this.state === 'error') {
      this.startRecording();
    }
    // In WebSocket mode, allow starting recording even during searching/transcribing
    // In HTTP mode, ignore toggle during transcribing/searching states (button is disabled anyway)
  }

  private setState(state: FlowState): void {
    this.state = state;
    this.updateUI();
    this.onStateChange?.(state);
  }

  private updateUI(): void {
    const isProcessing = this.state === 'transcribing' || this.state === 'searching';

    // Update button visual state
    this.micButton.classList.toggle('recording', this.state === 'recording');
    this.micButton.classList.toggle('processing', isProcessing);
    this.micButton.disabled = isProcessing;

    // Update accessibility attributes
    this.micButton.setAttribute('aria-pressed', String(this.state === 'recording'));
    this.micButton.setAttribute('aria-label', this.getAriaLabel());

    // Show loading indicator in media display during processing states
    this.updateLoadingIndicator();
  }

  private updateLoadingIndicator(): void {
    switch (this.state) {
      case 'recording':
        this.display.reset('Listening...');
        break;
      case 'transcribing':
        this.display.reset('Processing...');
        break;
      case 'searching':
        this.display.reset('Searching...');
        break;
    }
  }

  private getAriaLabel(): string {
    switch (this.state) {
      case 'recording':
        return 'Listening... Click to stop';
      case 'transcribing':
        return 'Processing your voice...';
      case 'searching':
        return 'Searching for media...';
      case 'error':
        return 'An error occurred. Click to try again';
      case 'displaying':
        return 'Showing result. Click to record a new command';
      default:
        return 'Click to start recording';
    }
  }

  /**
   * Start recording audio
   */
  async startRecording(): Promise<void> {
    if (this.state !== 'idle' && this.state !== 'displaying' && this.state !== 'error') {
      return;
    }

    try {
      if (this.useWebSocket && this.wsClient) {
        await this.startWebSocketRecording();
      } else {
        await this.recorder.startRecording();
      }
      this.setState('recording');
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  /**
   * Start WebSocket-based recording with streaming
   */
  private async startWebSocketRecording(): Promise<void> {
    if (!this.wsClient) {
      throw new ApiError('WebSocket client not initialized', 'client');
    }

    // Connect to WebSocket if not already connected
    if (this.wsClient.getState() !== 'connected') {
      await this.wsClient.connect();
    }

    // Get microphone access
    this.stream = await navigator.mediaDevices.getUserMedia({ audio: true });

    // Create MediaRecorder to capture audio chunks
    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
      ? 'audio/webm;codecs=opus'
      : MediaRecorder.isTypeSupported('audio/webm')
        ? 'audio/webm'
        : 'audio/ogg';

    this.mediaRecorder = new MediaRecorder(this.stream, { mimeType });

    // Send audio chunks every 500ms
    this.mediaRecorder.ondataavailable = (event) => {
      if (event.data.size > 0 && this.wsClient) {
        this.wsClient.sendAudioChunk(event.data);
      }
    };

    // Tell server we're starting a recording session
    this.wsClient.startRecording();

    // Start recording with timeslice of 500ms
    this.mediaRecorder.start(500);
  }

  /**
   * Stop recording and process the voice command
   */
  async stopRecordingAndProcess(): Promise<void> {
    if (this.state !== 'recording') {
      return;
    }

    try {
      if (this.useWebSocket && this.wsClient) {
        await this.stopWebSocketRecording();
      } else {
        await this.stopHttpRecording();
      }
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  /**
   * Stop WebSocket recording - server handles processing
   */
  private async stopWebSocketRecording(): Promise<void> {
    // Stop MediaRecorder
    if (this.mediaRecorder && this.mediaRecorder.state === 'recording') {
      this.mediaRecorder.stop();
    }

    // Tell server we're done recording
    if (this.wsClient) {
      this.wsClient.stopRecording();
    }

    // Clean up microphone stream
    if (this.stream) {
      this.stream.getTracks().forEach(track => track.stop());
      this.stream = null;
    }
    this.mediaRecorder = null;

    // Transition to transcribing - server will send back results via WebSocket
    this.setState('transcribing');
  }

  /**
   * Stop HTTP recording and process via REST APIs (legacy mode)
   */
  private async stopHttpRecording(): Promise<void> {
    // Stop recording and get audio blob
    this.setState('transcribing');
    const { blob } = await this.recorder.stopRecording();

    // Transcribe audio
    const transcript = await transcribeAudio(blob);

    // Extract intent and fetch media
    this.setState('searching');
    const { subject } = await extractIntent(transcript);

    // Fetch media for the subject
    const media = await fetchMedia(subject);

    // Display the media with accessibility context
    this.display.show(media, subject);
    this.setState('displaying');

    // Attempt to speak the subject using TTS (if available)
    // TTS is optional - we don't fail the flow if it's unavailable
    try {
      await speakSubject(subject);
    } catch (ttsError) {
      // Log TTS errors but don't interrupt the media display
      console.warn('TTS unavailable:', ttsError);
    }
  }

  /**
   * Handle transcript received from WebSocket
   */
  private handleWsTranscript(text: string): void {
    // Transcript received - transitioning to searching
    this.setState('searching');
  }

  /**
   * Handle media received from WebSocket
   */
  private async handleWsMedia(media: MediaMessage): Promise<void> {
    const formattedMedia = {
      photoUrl: media.photo_url,
      audioUrl: media.audio_url,
      videoUrl: media.video_url,
    };

    // Display the media with accessibility context
    this.display.show(formattedMedia, media.subject);
    this.setState('displaying');

    // Attempt to speak the subject using TTS (if available)
    try {
      await speakSubject(media.subject);
    } catch (ttsError) {
      console.warn('TTS unavailable:', ttsError);
    }
  }

  /**
   * Handle WebSocket connection state changes
   */
  private handleWsStateChange(wsState: ConnectionState): void {
    // If connection is lost during recording, handle gracefully
    if (wsState === 'disconnected' && this.state === 'recording') {
      this.cleanupWebSocketRecording();
      this.handleError(new ApiError('Connection lost while recording', 'network'));
    }
  }

  /**
   * Clean up WebSocket recording resources
   */
  private cleanupWebSocketRecording(): void {
    if (this.mediaRecorder && this.mediaRecorder.state === 'recording') {
      this.mediaRecorder.stop();
    }
    if (this.stream) {
      this.stream.getTracks().forEach(track => track.stop());
      this.stream = null;
    }
    this.mediaRecorder = null;
  }

  private handleError(error: Error): void {
    console.error('Peekaboo flow error:', error);
    this.setState('error');

    // Get user-friendly error message based on error type
    const message = getUserFriendlyMessage(error);
    this.display.reset(message);

    // Update aria-label with error message for accessibility
    this.micButton.setAttribute('aria-label', `${message}. Click to try again`);

    this.onError?.(error);

    // Longer timeout for rate limit errors
    const timeout = (error instanceof ApiError && error.type === 'rate_limit') ? 5000 : 3000;

    // Reset to idle after showing error
    setTimeout(() => {
      if (this.state === 'error') {
        this.setState('idle');
      }
    }, timeout);
  }

  /**
   * Get current state
   */
  getState(): FlowState {
    return this.state;
  }

  /**
   * Reset to idle state
   */
  reset(): void {
    this.cleanupWebSocketRecording();
    this.display.reset();
    this.setState('idle');
  }

  /**
   * Disconnect WebSocket and clean up all resources
   */
  destroy(): void {
    this.cleanupWebSocketRecording();
    if (this.wsClient) {
      this.wsClient.disconnect();
    }
  }
}
