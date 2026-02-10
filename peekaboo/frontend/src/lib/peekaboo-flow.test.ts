import { describe, it, expect, vi, beforeEach, afterEach, Mock } from 'vitest';
import { PeekabooFlow, FlowState } from './peekaboo-flow';
import * as audioRecorder from './audio-recorder';
import * as intent from './intent';
import * as mediaDisplay from './media-display';
import * as textToSpeech from './text-to-speech';
import * as websocketAudio from './websocket-audio';

// Mock modules at the top level
vi.mock('./audio-recorder', () => ({
  AudioRecorder: vi.fn(),
  transcribeAudio: vi.fn(),
}));

vi.mock('./intent', () => ({
  extractIntent: vi.fn(),
}));

vi.mock('./media-display', () => ({
  MediaDisplay: vi.fn(),
  fetchMedia: vi.fn(),
}));

vi.mock('./text-to-speech', () => ({
  speakSubject: vi.fn(),
}));

vi.mock('./websocket-audio', () => ({
  AudioWebSocket: vi.fn(),
}));

describe('PeekabooFlow', () => {
  let micButton: HTMLButtonElement;
  let mediaContainer: HTMLElement;
  let stateChanges: FlowState[];
  let flow: PeekabooFlow;

  // Store mock function references
  let mockStartRecording: Mock;
  let mockStopRecording: Mock;
  let mockIsRecording: Mock;
  let mockShow: Mock;
  let mockReset: Mock;

  beforeEach(() => {
    // Create fresh mocks
    mockStartRecording = vi.fn().mockResolvedValue(undefined);
    mockStopRecording = vi.fn().mockResolvedValue({
      blob: new Blob(['mock audio'], { type: 'audio/webm' }),
      mimeType: 'audio/webm',
    });
    mockIsRecording = vi.fn().mockReturnValue(false);
    mockShow = vi.fn();
    mockReset = vi.fn();

    // Setup AudioRecorder constructor mock
    (audioRecorder.AudioRecorder as unknown as Mock).mockImplementation(() => ({
      startRecording: mockStartRecording,
      stopRecording: mockStopRecording,
      isRecording: mockIsRecording,
      destroy: vi.fn(),
    }));

    // Setup transcribeAudio mock
    (audioRecorder.transcribeAudio as Mock).mockResolvedValue('show me a cat');

    // Setup extractIntent mock
    (intent.extractIntent as Mock).mockResolvedValue({ subject: 'cat' });

    // Setup MediaDisplay constructor mock
    (mediaDisplay.MediaDisplay as unknown as Mock).mockImplementation(() => ({
      show: mockShow,
      reset: mockReset,
      stopAudio: vi.fn(),
      getAudioElement: vi.fn(),
    }));

    // Setup fetchMedia mock
    (mediaDisplay.fetchMedia as Mock).mockResolvedValue({
      photoUrl: '/data/media/cat/photo.jpg',
      audioUrl: '/data/media/cat/audio.mp3',
    });

    // Setup speakSubject mock - TTS is optional, return null (TTS unavailable)
    (textToSpeech.speakSubject as Mock).mockResolvedValue(null);

    // Create DOM elements
    micButton = document.createElement('button');
    mediaContainer = document.createElement('div');

    stateChanges = [];
    flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      onStateChange: (state) => stateChanges.push(state),
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('initial state', () => {
    it('starts in idle state', () => {
      expect(flow.getState()).toBe('idle');
    });

    it('button is not disabled initially', () => {
      expect(micButton.disabled).toBe(false);
    });
  });

  describe('startRecording', () => {
    it('transitions to recording state', async () => {
      await flow.startRecording();

      expect(flow.getState()).toBe('recording');
      expect(stateChanges).toContain('recording');
    });

    it('adds recording class to button', async () => {
      await flow.startRecording();

      expect(micButton.classList.contains('recording')).toBe(true);
    });

    it('calls AudioRecorder.startRecording', async () => {
      await flow.startRecording();

      expect(mockStartRecording).toHaveBeenCalled();
    });

    it('does nothing if already recording', async () => {
      await flow.startRecording();
      stateChanges.length = 0;

      await flow.startRecording();

      expect(stateChanges).toHaveLength(0);
    });
  });

  describe('stopRecordingAndProcess', () => {
    it('does nothing if not recording', async () => {
      await flow.stopRecordingAndProcess();

      expect(flow.getState()).toBe('idle');
      expect(stateChanges).toHaveLength(0);
    });

    it('transitions through transcribing state', async () => {
      await flow.startRecording();
      stateChanges.length = 0;

      await flow.stopRecordingAndProcess();

      expect(stateChanges).toContain('transcribing');
    });

    it('transitions through searching state', async () => {
      await flow.startRecording();
      stateChanges.length = 0;

      await flow.stopRecordingAndProcess();

      expect(stateChanges).toContain('searching');
    });

    it('ends in displaying state on success', async () => {
      await flow.startRecording();

      await flow.stopRecordingAndProcess();

      expect(flow.getState()).toBe('displaying');
    });

    it('calls stopRecording on the recorder', async () => {
      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(mockStopRecording).toHaveBeenCalled();
    });
  });

  describe('full flow integration', () => {
    it('completes full flow: idle -> recording -> transcribing -> searching -> displaying', async () => {
      expect(flow.getState()).toBe('idle');

      await flow.startRecording();
      expect(flow.getState()).toBe('recording');

      await flow.stopRecordingAndProcess();
      expect(flow.getState()).toBe('displaying');

      // Verify state transitions
      expect(stateChanges).toEqual(['recording', 'transcribing', 'searching', 'displaying']);
    });

    it('calls all API functions in correct order', async () => {
      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      // Verify transcription was called with the blob
      expect(audioRecorder.transcribeAudio).toHaveBeenCalled();

      // Verify intent extraction was called with transcript
      expect(intent.extractIntent).toHaveBeenCalledWith('show me a cat');

      // Verify media fetch was called with subject
      expect(mediaDisplay.fetchMedia).toHaveBeenCalledWith('cat');

      // Verify display.show was called with media and subject
      expect(mockShow).toHaveBeenCalledWith(
        {
          photoUrl: '/data/media/cat/photo.jpg',
          audioUrl: '/data/media/cat/audio.mp3',
        },
        'cat'
      );
    });
  });

  describe('accessibility', () => {
    it('updates aria-pressed attribute when recording', async () => {
      expect(micButton.getAttribute('aria-pressed')).toBe('false');

      await flow.startRecording();

      expect(micButton.getAttribute('aria-pressed')).toBe('true');
    });

    it('resets aria-pressed when not recording', async () => {
      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(micButton.getAttribute('aria-pressed')).toBe('false');
    });

    it('updates aria-label based on state', async () => {
      // Initial idle state
      expect(micButton.getAttribute('aria-label')).toContain('Click to start');

      // Recording state
      await flow.startRecording();
      expect(micButton.getAttribute('aria-label')).toContain('Listening');

      // Processing state is brief, but we can check displaying state
      await flow.stopRecordingAndProcess();
      expect(micButton.getAttribute('aria-label')).toContain('Showing result');
    });

    it('updates aria-label to error message on failure', async () => {
      (audioRecorder.transcribeAudio as Mock).mockRejectedValue(
        new Error('Test error')
      );

      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(micButton.getAttribute('aria-label')).toContain('error');
    });
  });

  describe('error handling', () => {
    it('transitions to error state on transcription failure', async () => {
      (audioRecorder.transcribeAudio as Mock).mockRejectedValue(
        new Error('Transcription failed')
      );

      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(flow.getState()).toBe('error');
    });

    it('transitions to error state on intent extraction failure', async () => {
      (intent.extractIntent as Mock).mockRejectedValue(
        new Error('Intent extraction failed')
      );

      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(flow.getState()).toBe('error');
    });

    it('transitions to error state on media fetch failure', async () => {
      (mediaDisplay.fetchMedia as Mock).mockRejectedValue(
        new Error('Media fetch failed')
      );

      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(flow.getState()).toBe('error');
    });

    it('silently resets to idle on empty transcript', async () => {
      (audioRecorder.transcribeAudio as Mock).mockResolvedValue('');

      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(flow.getState()).toBe('idle');
    });

    it('silently resets to idle on whitespace-only transcript', async () => {
      (audioRecorder.transcribeAudio as Mock).mockResolvedValue('   \n  ');

      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(flow.getState()).toBe('idle');
    });

    it('calls onError callback on failure', async () => {
      const onError = vi.fn();
      const errorFlow = new PeekabooFlow({
        micButton,
        mediaContainer,
        onError,
      });

      (audioRecorder.transcribeAudio as Mock).mockRejectedValue(
        new Error('Test error')
      );

      await errorFlow.startRecording();
      await errorFlow.stopRecordingAndProcess();

      expect(onError).toHaveBeenCalledWith(expect.any(Error));
    });

    it('displays error message in media container', async () => {
      (audioRecorder.transcribeAudio as Mock).mockRejectedValue(
        new Error('Test error message')
      );

      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(mockReset).toHaveBeenCalledWith(expect.stringContaining('Test error message'));
    });
  });

  describe('loading state indicators', () => {
    it('shows "Listening..." during recording state', async () => {
      await flow.startRecording();

      expect(mockReset).toHaveBeenCalledWith('Listening...');
    });

    it('shows "Processing..." during transcribing state', async () => {
      await flow.startRecording();
      mockReset.mockClear();

      // Start processing - this will transition to transcribing
      const processPromise = flow.stopRecordingAndProcess();

      // After the first state change (transcribing), check display
      await new Promise(resolve => setTimeout(resolve, 0));
      expect(mockReset).toHaveBeenCalledWith('Processing...');

      await processPromise;
    });

    it('shows "Searching..." during searching state', async () => {
      await flow.startRecording();
      mockReset.mockClear();

      await flow.stopRecordingAndProcess();

      expect(mockReset).toHaveBeenCalledWith('Searching...');
    });

    it('disables button during processing states', async () => {
      // Track whether button was ever disabled during processing
      let wasDisabledDuringProcessing = false;
      const checkDisabled = () => {
        if (flow.getState() === 'transcribing' || flow.getState() === 'searching') {
          wasDisabledDuringProcessing = micButton.disabled;
        }
      };

      // Override onStateChange to check button state
      const stateFlow = new PeekabooFlow({
        micButton,
        mediaContainer,
        onStateChange: (state) => {
          if (state === 'transcribing' || state === 'searching') {
            wasDisabledDuringProcessing = wasDisabledDuringProcessing || micButton.disabled;
          }
        },
      });

      await stateFlow.startRecording();
      await stateFlow.stopRecordingAndProcess();

      expect(wasDisabledDuringProcessing).toBe(true);
    });

    it('adds processing class during processing states', async () => {
      let hadProcessingClass = false;

      const stateFlow = new PeekabooFlow({
        micButton,
        mediaContainer,
        onStateChange: (state) => {
          if (state === 'transcribing' || state === 'searching') {
            hadProcessingClass = hadProcessingClass || micButton.classList.contains('processing');
          }
        },
      });

      await stateFlow.startRecording();
      await stateFlow.stopRecordingAndProcess();

      expect(hadProcessingClass).toBe(true);
    });
  });

  describe('reset', () => {
    it('resets to idle state', async () => {
      await flow.startRecording();
      await flow.stopRecordingAndProcess();
      stateChanges.length = 0;

      flow.reset();

      expect(flow.getState()).toBe('idle');
      expect(stateChanges).toContain('idle');
    });

    it('calls display.reset', () => {
      flow.reset();

      expect(mockReset).toHaveBeenCalled();
    });
  });

  describe('destroy', () => {
    it('clears pending error timeout', async () => {
      vi.useFakeTimers();

      (audioRecorder.transcribeAudio as Mock).mockRejectedValue(
        new Error('Test error')
      );

      await flow.startRecording();
      await flow.stopRecordingAndProcess();
      expect(flow.getState()).toBe('error');

      // Destroy while error timeout is pending
      flow.destroy();

      // Advance past the error timeout (3 seconds)
      vi.advanceTimersByTime(5000);

      // State should still be error (timeout was cleared, no transition to idle)
      expect(flow.getState()).toBe('error');
      vi.useRealTimers();
    });

    it('calls recorder.destroy() to clean up HTTP-mode resources', () => {
      const mockDestroy = vi.fn();
      (audioRecorder.AudioRecorder as unknown as Mock).mockImplementation(() => ({
        startRecording: vi.fn().mockResolvedValue(undefined),
        stopRecording: vi.fn(),
        isRecording: vi.fn().mockReturnValue(false),
        destroy: mockDestroy,
      }));

      const f = new PeekabooFlow({ micButton, mediaContainer });
      f.destroy();
      expect(mockDestroy).toHaveBeenCalled();
    });

    it('calls display.stopAudio() to clean up media audio', () => {
      const mockStopAudioFn = vi.fn();
      (mediaDisplay.MediaDisplay as unknown as Mock).mockImplementation(() => ({
        show: vi.fn(),
        reset: vi.fn(),
        stopAudio: mockStopAudioFn,
        getAudioElement: vi.fn(),
      }));

      const f = new PeekabooFlow({ micButton, mediaContainer });
      f.destroy();
      expect(mockStopAudioFn).toHaveBeenCalled();
    });
  });

  // Event tests (click, touch, keyboard) moved to peekaboo-flow-events.test.ts
  // WebSocket mode tests moved to peekaboo-flow-websocket.test.ts
});
