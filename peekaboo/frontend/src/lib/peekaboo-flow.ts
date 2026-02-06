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
import { logger } from './logger';
import type { MediaMessage, ConnectionState } from './websocket-audio';

export type FlowState = 'idle' | 'recording' | 'transcribing' | 'searching' | 'displaying' | 'error';

export interface PeekabooFlowOptions {
  micButton: HTMLButtonElement;
  mediaContainer: HTMLElement;
  /** Optional container for transcript history display */
  transcriptContainer?: HTMLElement;
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

  // Transcript display
  private transcriptContainer: HTMLElement | null = null;

  // Bound event handlers (stored for removal in destroy)
  private handleClick: (e: MouseEvent) => void;
  private handleTouchEnd: (e: TouchEvent) => void;
  private handleKeyDown: (e: KeyboardEvent) => void;

  constructor(options: PeekabooFlowOptions) {
    this.recorder = new AudioRecorder();
    this.display = new MediaDisplay(options.mediaContainer);
    this.micButton = options.micButton;
    this.onStateChange = options.onStateChange;
    this.onError = options.onError;
    this.useWebSocket = options.useWebSocket ?? false;
    this.transcriptContainer = options.transcriptContainer ?? null;

    // Create bound handlers for event listener cleanup
    this.handleClick = (e: MouseEvent) => {
      e.preventDefault();
      this.toggleRecording();
    };
    this.handleTouchEnd = (e: TouchEvent) => {
      e.preventDefault();
      this.toggleRecording();
    };
    this.handleKeyDown = (e: KeyboardEvent) => {
      if (e.repeat) return;
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        this.toggleRecording();
      }
    };

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
    this.micButton.addEventListener('click', this.handleClick);
    this.micButton.addEventListener('touchend', this.handleTouchEnd);
    this.micButton.addEventListener('keydown', this.handleKeyDown);
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

    // Check for empty transcript (silence or no recognizable speech)
    if (!transcript || transcript.trim() === '') {
      throw new ApiError('No speech detected. Please try again.', 'client');
    }

    // Extract intent and fetch media
    this.setState('searching');
    const { subject } = await extractIntent(transcript);

    // Fetch media for the subject
    const media = await fetchMedia(subject);

    // Display the media with accessibility context
    // Audio autoplay may be blocked - display handles showing indicator
    await this.display.show(media, subject);
    this.setState('displaying');

    // Attempt to speak the subject using TTS (if available)
    // TTS is optional - we don't fail the flow if it's unavailable
    try {
      await speakSubject(subject);
    } catch (ttsError) {
      // Log TTS errors but don't interrupt the media display
      logger.warn('TTS unavailable:', ttsError);
    }
  }

  /**
   * Handle transcript received from WebSocket
   */
  private handleWsTranscript(text: string): void {
    // Display the transcript in the transcript container
    this.appendTranscript(text);

    // Transcript received - in continuous listening mode, we stay in recording state
    // The backend will send media shortly after
    // Only show searching indicator if we're not recording (e.g., user stopped mic)
    if (this.state !== 'recording') {
      this.setState('searching');
    }
    // In continuous listening, the recording indicator stays on while searching
  }

  /**
   * Append a transcript entry to the transcript display
   */
  private appendTranscript(text: string): void {
    if (!this.transcriptContainer) {
      return;
    }

    const entry = document.createElement('div');
    entry.className = 'transcript-entry';
    entry.textContent = `"${text}"`;
    this.transcriptContainer.appendChild(entry);

    // Scroll to bottom to show latest transcript
    this.transcriptContainer.scrollTop = this.transcriptContainer.scrollHeight;
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
    // Audio autoplay may be blocked - display handles showing indicator
    await this.display.show(formattedMedia, media.subject);

    // In continuous listening mode, keep recording state while showing media
    // The mic stays active until user explicitly clicks to stop
    // This enables "conversational" UX where multiple commands can be issued
    if (this.mediaRecorder && this.mediaRecorder.state === 'recording') {
      // Keep recording - don't change state, just update the UI
      // The mic button stays in "recording" state
      this.updateRecordingWithMediaUI(media.subject);
    } else {
      // Not recording (e.g., stop was already called or recorder failed)
      this.setState('displaying');
    }

    // Attempt to speak the subject using TTS (if available)
    try {
      await speakSubject(media.subject);
    } catch (ttsError) {
      logger.warn('TTS unavailable:', ttsError);
    }
  }

  /**
   * Update UI when showing media while still recording (continuous listening)
   */
  private updateRecordingWithMediaUI(subject: string): void {
    // Keep recording visual state on button
    this.micButton.classList.add('recording');
    this.micButton.classList.remove('processing');
    this.micButton.disabled = false;

    // Update aria-label to reflect continuous listening state
    this.micButton.setAttribute('aria-pressed', 'true');
    this.micButton.setAttribute('aria-label', `Showing ${subject}. Still listening... Click to stop recording`);
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
    logger.error('Peekaboo flow error:', error);
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
   * Disconnect WebSocket, remove event listeners, and clean up all resources
   */
  destroy(): void {
    this.cleanupWebSocketRecording();
    if (this.wsClient) {
      this.wsClient.disconnect();
    }
    this.micButton.removeEventListener('click', this.handleClick);
    this.micButton.removeEventListener('touchend', this.handleTouchEnd);
    this.micButton.removeEventListener('keydown', this.handleKeyDown);
  }
}
