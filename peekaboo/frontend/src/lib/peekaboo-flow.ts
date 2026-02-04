/**
 * Peekaboo Flow - Orchestrates the full voice-to-media flow
 *
 * Flow: button press -> record audio -> transcribe -> intent -> fetch media -> display
 */

import { AudioRecorder, transcribeAudio } from './audio-recorder';
import { extractIntent } from './intent';
import { MediaDisplay, fetchMedia } from './media-display';
import { getUserFriendlyMessage, ApiError } from './errors';

export type FlowState = 'idle' | 'recording' | 'transcribing' | 'searching' | 'displaying' | 'error';

export interface PeekabooFlowOptions {
  micButton: HTMLButtonElement;
  mediaContainer: HTMLElement;
  onStateChange?: (state: FlowState) => void;
  onError?: (error: Error) => void;
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

  constructor(options: PeekabooFlowOptions) {
    this.recorder = new AudioRecorder();
    this.display = new MediaDisplay(options.mediaContainer);
    this.micButton = options.micButton;
    this.onStateChange = options.onStateChange;
    this.onError = options.onError;

    this.setupEventListeners();
    this.updateUI(); // Initialize accessibility attributes
  }

  private setupEventListeners(): void {
    // Mouse events
    this.micButton.addEventListener('mousedown', () => this.startRecording());
    this.micButton.addEventListener('mouseup', () => this.stopRecordingAndProcess());
    this.micButton.addEventListener('mouseleave', () => {
      if (this.state === 'recording') {
        this.stopRecordingAndProcess();
      }
    });

    // Touch events
    this.micButton.addEventListener('touchstart', () => this.startRecording(), { passive: true });
    this.micButton.addEventListener('touchend', () => this.stopRecordingAndProcess());

    // Keyboard events - Enter and Space act like press-and-hold
    this.micButton.addEventListener('keydown', (e: KeyboardEvent) => {
      // Ignore repeated keydown events (key held down)
      if (e.repeat) return;

      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault(); // Prevent scrolling on Space, form submission on Enter
        this.startRecording();
      }
    });

    this.micButton.addEventListener('keyup', (e: KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        this.stopRecordingAndProcess();
      }
    });
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
        return 'Listening... Release to stop';
      case 'transcribing':
        return 'Processing your voice...';
      case 'searching':
        return 'Searching for media...';
      case 'error':
        return 'An error occurred. Press and hold to try again';
      case 'displaying':
        return 'Showing result. Press and hold to record a new command';
      default:
        return 'Press and hold to record voice command';
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
      await this.recorder.startRecording();
      this.setState('recording');
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  /**
   * Stop recording and process the voice command
   */
  async stopRecordingAndProcess(): Promise<void> {
    if (this.state !== 'recording') {
      return;
    }

    try {
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
    } catch (error) {
      this.handleError(error as Error);
    }
  }

  private handleError(error: Error): void {
    console.error('Peekaboo flow error:', error);
    this.setState('error');

    // Get user-friendly error message based on error type
    const message = getUserFriendlyMessage(error);
    this.display.reset(message);

    // Update aria-label with error message for accessibility
    this.micButton.setAttribute('aria-label', `${message}. Press and hold to try again`);

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
    this.display.reset();
    this.setState('idle');
  }
}
