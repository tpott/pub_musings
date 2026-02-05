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

    it('transitions to error state on empty transcript', async () => {
      (audioRecorder.transcribeAudio as Mock).mockResolvedValue('');

      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(flow.getState()).toBe('error');
      expect(mockReset).toHaveBeenCalledWith(expect.stringContaining('speech'));
    });

    it('transitions to error state on whitespace-only transcript', async () => {
      (audioRecorder.transcribeAudio as Mock).mockResolvedValue('   \n  ');

      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      expect(flow.getState()).toBe('error');
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

  describe('click events (toggle behavior)', () => {
    it('starts recording on first click', async () => {
      const clickEvent = new MouseEvent('click');
      micButton.dispatchEvent(clickEvent);

      // Wait for async operation
      await new Promise(resolve => setTimeout(resolve, 0));

      expect(flow.getState()).toBe('recording');
    });

    it('stops recording on second click', async () => {
      // First click starts recording
      micButton.dispatchEvent(new MouseEvent('click'));
      await new Promise(resolve => setTimeout(resolve, 0));
      expect(flow.getState()).toBe('recording');

      // Second click stops recording and processes
      micButton.dispatchEvent(new MouseEvent('click'));
      await new Promise(resolve => setTimeout(resolve, 10));

      expect(flow.getState()).toBe('displaying');
    });

    it('can start new recording after displaying', async () => {
      // Complete a full flow
      micButton.dispatchEvent(new MouseEvent('click'));
      await new Promise(resolve => setTimeout(resolve, 0));
      micButton.dispatchEvent(new MouseEvent('click'));
      await new Promise(resolve => setTimeout(resolve, 10));
      expect(flow.getState()).toBe('displaying');

      // Click again to start new recording
      micButton.dispatchEvent(new MouseEvent('click'));
      await new Promise(resolve => setTimeout(resolve, 0));
      expect(flow.getState()).toBe('recording');
    });
  });

  describe('touch events (toggle behavior)', () => {
    it('starts recording on first touchend', async () => {
      const touchEndEvent = new TouchEvent('touchend');
      micButton.dispatchEvent(touchEndEvent);

      await new Promise(resolve => setTimeout(resolve, 0));

      expect(flow.getState()).toBe('recording');
    });

    it('stops recording on second touchend', async () => {
      // First touch starts recording
      micButton.dispatchEvent(new TouchEvent('touchend'));
      await new Promise(resolve => setTimeout(resolve, 0));
      expect(flow.getState()).toBe('recording');

      // Second touch stops recording and processes
      micButton.dispatchEvent(new TouchEvent('touchend'));
      await new Promise(resolve => setTimeout(resolve, 10));

      expect(flow.getState()).toBe('displaying');
    });
  });

  describe('keyboard events (toggle behavior)', () => {
    it('starts recording on Space keydown', async () => {
      const keydownEvent = new KeyboardEvent('keydown', { key: ' ' });
      micButton.dispatchEvent(keydownEvent);

      await new Promise(resolve => setTimeout(resolve, 0));

      expect(flow.getState()).toBe('recording');
    });

    it('starts recording on Enter keydown', async () => {
      const keydownEvent = new KeyboardEvent('keydown', { key: 'Enter' });
      micButton.dispatchEvent(keydownEvent);

      await new Promise(resolve => setTimeout(resolve, 0));

      expect(flow.getState()).toBe('recording');
    });

    it('stops recording on second Space keydown', async () => {
      // First keydown starts recording
      micButton.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }));
      await new Promise(resolve => setTimeout(resolve, 0));
      expect(flow.getState()).toBe('recording');

      // Second keydown stops recording and processes
      micButton.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }));
      await new Promise(resolve => setTimeout(resolve, 10));

      expect(flow.getState()).toBe('displaying');
    });

    it('stops recording on second Enter keydown', async () => {
      // First keydown starts recording
      micButton.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
      await new Promise(resolve => setTimeout(resolve, 0));
      expect(flow.getState()).toBe('recording');

      // Second keydown stops recording and processes
      micButton.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
      await new Promise(resolve => setTimeout(resolve, 10));

      expect(flow.getState()).toBe('displaying');
    });

    it('ignores repeated keydown events (key held down)', async () => {
      // First keydown starts recording
      micButton.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }));
      await new Promise(resolve => setTimeout(resolve, 0));
      expect(flow.getState()).toBe('recording');

      // Clear state tracking
      stateChanges.length = 0;

      // Repeated keydown (e.repeat=true) should be ignored - doesn't toggle
      micButton.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', repeat: true }));
      await new Promise(resolve => setTimeout(resolve, 0));

      // State should still be recording, no new state change
      expect(flow.getState()).toBe('recording');
      expect(stateChanges).toHaveLength(0);
    });

    it('ignores other keys', async () => {
      micButton.dispatchEvent(new KeyboardEvent('keydown', { key: 'a' }));
      await new Promise(resolve => setTimeout(resolve, 0));

      expect(flow.getState()).toBe('idle');
    });
  });
});

describe('PeekabooFlow WebSocket mode', () => {
  let micButton: HTMLButtonElement;
  let mediaContainer: HTMLElement;
  let stateChanges: FlowState[];

  // WebSocket mock functions
  let mockWsConnect: Mock;
  let mockWsDisconnect: Mock;
  let mockWsStartRecording: Mock;
  let mockWsStopRecording: Mock;
  let mockWsSendAudioChunk: Mock;
  let mockWsGetState: Mock;
  let wsCallbacks: any;

  // MediaDisplay mock functions
  let mockShow: Mock;
  let mockReset: Mock;

  // MediaRecorder and getUserMedia mocks
  let mockMediaRecorderStart: Mock;
  let mockMediaRecorderStop: Mock;
  let mockGetTracks: Mock;

  beforeEach(() => {
    // Create fresh WebSocket mocks
    mockWsConnect = vi.fn().mockResolvedValue(undefined);
    mockWsDisconnect = vi.fn();
    mockWsStartRecording = vi.fn();
    mockWsStopRecording = vi.fn();
    mockWsSendAudioChunk = vi.fn();
    mockWsGetState = vi.fn().mockReturnValue('connected');

    // Setup AudioWebSocket constructor mock
    (websocketAudio.AudioWebSocket as unknown as Mock).mockImplementation((callbacks) => {
      wsCallbacks = callbacks;
      return {
        connect: mockWsConnect,
        disconnect: mockWsDisconnect,
        startRecording: mockWsStartRecording,
        stopRecording: mockWsStopRecording,
        sendAudioChunk: mockWsSendAudioChunk,
        getState: mockWsGetState,
      };
    });

    // Setup MediaDisplay constructor mock
    mockShow = vi.fn();
    mockReset = vi.fn();
    (mediaDisplay.MediaDisplay as unknown as Mock).mockImplementation(() => ({
      show: mockShow,
      reset: mockReset,
      stopAudio: vi.fn(),
      getAudioElement: vi.fn(),
    }));

    // Setup speakSubject mock
    (textToSpeech.speakSubject as Mock).mockResolvedValue(null);

    // Create DOM elements
    micButton = document.createElement('button');
    mediaContainer = document.createElement('div');

    // Setup MediaRecorder mock
    mockMediaRecorderStart = vi.fn();
    mockMediaRecorderStop = vi.fn();

    const MockMediaRecorder = vi.fn().mockImplementation(() => ({
      start: mockMediaRecorderStart,
      stop: mockMediaRecorderStop,
      state: 'inactive',
      ondataavailable: null,
    }));
    MockMediaRecorder.isTypeSupported = vi.fn().mockReturnValue(true);
    (global as any).MediaRecorder = MockMediaRecorder;

    // Setup getUserMedia mock
    mockGetTracks = vi.fn().mockReturnValue([{ stop: vi.fn() }]);
    (global as any).navigator = {
      mediaDevices: {
        getUserMedia: vi.fn().mockResolvedValue({
          getTracks: mockGetTracks,
        }),
      },
    };

    stateChanges = [];
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('creates WebSocket client when useWebSocket is true', () => {
    new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
    });

    expect(websocketAudio.AudioWebSocket).toHaveBeenCalled();
  });

  it('does not create WebSocket client when useWebSocket is false', () => {
    new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: false,
    });

    expect(websocketAudio.AudioWebSocket).not.toHaveBeenCalled();
  });

  it('connects to WebSocket and starts MediaRecorder on startRecording', async () => {
    mockWsGetState.mockReturnValue('disconnected');

    const flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
      onStateChange: (state) => stateChanges.push(state),
    });

    await flow.startRecording();

    expect(mockWsConnect).toHaveBeenCalled();
    expect(navigator.mediaDevices.getUserMedia).toHaveBeenCalledWith({ audio: true });
    expect(mockWsStartRecording).toHaveBeenCalled();
    expect(mockMediaRecorderStart).toHaveBeenCalledWith(500);
    expect(flow.getState()).toBe('recording');
  });

  it('skips connect if already connected', async () => {
    mockWsGetState.mockReturnValue('connected');

    const flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
    });

    await flow.startRecording();

    expect(mockWsConnect).not.toHaveBeenCalled();
    expect(mockWsStartRecording).toHaveBeenCalled();
  });

  it('stops recording and notifies WebSocket on stopRecordingAndProcess', async () => {
    mockWsGetState.mockReturnValue('connected');

    const flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
      onStateChange: (state) => stateChanges.push(state),
    });

    await flow.startRecording();
    stateChanges.length = 0;

    await flow.stopRecordingAndProcess();

    expect(mockWsStopRecording).toHaveBeenCalled();
    expect(flow.getState()).toBe('transcribing');
  });

  it('handles transcript from WebSocket and transitions to searching', async () => {
    mockWsGetState.mockReturnValue('connected');

    const flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
      onStateChange: (state) => stateChanges.push(state),
    });

    await flow.startRecording();
    await flow.stopRecordingAndProcess();
    stateChanges.length = 0;

    // Simulate WebSocket transcript callback
    wsCallbacks.onTranscript('show me a cat');

    expect(flow.getState()).toBe('searching');
    expect(stateChanges).toContain('searching');
  });

  it('handles media from WebSocket and displays it', async () => {
    mockWsGetState.mockReturnValue('connected');

    const flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
      onStateChange: (state) => stateChanges.push(state),
    });

    await flow.startRecording();
    await flow.stopRecordingAndProcess();
    stateChanges.length = 0;

    // Simulate WebSocket media callback
    await wsCallbacks.onMedia({
      type: 'media',
      subject: 'cat',
      photo_url: '/data/media/cat/photo.jpg',
      audio_url: '/data/media/cat/audio.mp3',
    });

    expect(flow.getState()).toBe('displaying');
    expect(mockShow).toHaveBeenCalledWith(
      {
        photoUrl: '/data/media/cat/photo.jpg',
        audioUrl: '/data/media/cat/audio.mp3',
        videoUrl: undefined,
      },
      'cat'
    );
    expect(textToSpeech.speakSubject).toHaveBeenCalledWith('cat');
  });

  it('handles WebSocket errors gracefully', async () => {
    mockWsGetState.mockReturnValue('connected');
    const onError = vi.fn();

    const flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
      onError,
    });

    await flow.startRecording();

    // Simulate WebSocket error callback
    wsCallbacks.onError(new Error('WebSocket error'));

    expect(flow.getState()).toBe('error');
    expect(onError).toHaveBeenCalled();
  });

  it('cleans up resources when connection is lost during recording', async () => {
    mockWsGetState.mockReturnValue('connected');
    const onError = vi.fn();

    const flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
      onError,
    });

    await flow.startRecording();

    // Simulate connection state change to disconnected
    wsCallbacks.onStateChange('disconnected');

    expect(flow.getState()).toBe('error');
    expect(onError).toHaveBeenCalled();
  });

  it('disconnects WebSocket on destroy', () => {
    const flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
    });

    flow.destroy();

    expect(mockWsDisconnect).toHaveBeenCalled();
  });

  it('passes webSocketUrl option to AudioWebSocket', () => {
    new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
      webSocketUrl: 'ws://custom-url/ws',
    });

    expect(websocketAudio.AudioWebSocket).toHaveBeenCalledWith(
      expect.any(Object),
      { url: 'ws://custom-url/ws' }
    );
  });

  it('keeps recording state when receiving media (continuous listening)', async () => {
    // Create a MediaRecorder mock that reports it is recording
    let recorderState = 'inactive';
    const MockMediaRecorder = vi.fn().mockImplementation(() => ({
      start: vi.fn(() => { recorderState = 'recording'; }),
      stop: vi.fn(() => { recorderState = 'inactive'; }),
      get state() { return recorderState; },
      ondataavailable: null,
    }));
    MockMediaRecorder.isTypeSupported = vi.fn().mockReturnValue(true);
    (global as any).MediaRecorder = MockMediaRecorder;

    const flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
    });

    await flow.startRecording();
    expect(flow.getState()).toBe('recording');

    // Simulate receiving media while MediaRecorder is still in 'recording' state
    wsCallbacks.onMedia({
      type: 'media',
      subject: 'cat',
      photo_url: '/cat.jpg',
    });

    // Allow async display.show to complete
    await Promise.resolve();

    // Should still be in recording state (not displaying) - continuous listening mode
    expect(flow.getState()).toBe('recording');
    // Button should show recording state
    expect(micButton.classList.contains('recording')).toBe(true);
    // Aria-label should mention still listening
    expect(micButton.getAttribute('aria-label')).toContain('Still listening');
  });

  it('transitions to displaying when MediaRecorder is not recording', async () => {
    // Use a MediaRecorder that will report inactive state when onMedia is called
    let recorderState = 'inactive';
    const MockMediaRecorder = vi.fn().mockImplementation(() => ({
      start: vi.fn(() => { recorderState = 'recording'; }),
      stop: vi.fn(() => { recorderState = 'inactive'; }),
      get state() { return recorderState; },
      ondataavailable: null,
    }));
    MockMediaRecorder.isTypeSupported = vi.fn().mockReturnValue(true);
    (global as any).MediaRecorder = MockMediaRecorder;

    const flow = new PeekabooFlow({
      micButton,
      mediaContainer,
      useWebSocket: true,
    });

    await flow.startRecording();
    expect(flow.getState()).toBe('recording');

    // Manually set recorder to inactive to simulate user clicked stop
    recorderState = 'inactive';

    // Simulate receiving media when MediaRecorder reports inactive
    wsCallbacks.onMedia({
      type: 'media',
      subject: 'dog',
      photo_url: '/dog.jpg',
    });

    // Allow async display.show to complete
    await Promise.resolve();

    // Should transition to displaying since not recording
    expect(flow.getState()).toBe('displaying');
  });

  describe('transcript display', () => {
    it('appends transcript to transcript container when provided', async () => {
      mockWsGetState.mockReturnValue('connected');
      const transcriptContainer = document.createElement('div');

      new PeekabooFlow({
        micButton,
        mediaContainer,
        transcriptContainer,
        useWebSocket: true,
      });

      // Simulate WebSocket transcript callback
      wsCallbacks.onTranscript('show me a cat');

      const entries = transcriptContainer.querySelectorAll('.transcript-entry');
      expect(entries.length).toBe(1);
      expect(entries[0].textContent).toBe('"show me a cat"');
    });

    it('appends multiple transcripts to transcript container', async () => {
      mockWsGetState.mockReturnValue('connected');
      const transcriptContainer = document.createElement('div');

      new PeekabooFlow({
        micButton,
        mediaContainer,
        transcriptContainer,
        useWebSocket: true,
      });

      // Simulate multiple transcript callbacks
      wsCallbacks.onTranscript('show me a cat');
      wsCallbacks.onTranscript('show me a dog');
      wsCallbacks.onTranscript('show me a duck');

      const entries = transcriptContainer.querySelectorAll('.transcript-entry');
      expect(entries.length).toBe(3);
      expect(entries[0].textContent).toBe('"show me a cat"');
      expect(entries[1].textContent).toBe('"show me a dog"');
      expect(entries[2].textContent).toBe('"show me a duck"');
    });

    it('does not fail when no transcript container is provided', async () => {
      mockWsGetState.mockReturnValue('connected');

      // No transcriptContainer passed
      new PeekabooFlow({
        micButton,
        mediaContainer,
        useWebSocket: true,
      });

      // Should not throw
      expect(() => wsCallbacks.onTranscript('show me a cat')).not.toThrow();
    });

    it('scrolls transcript container to bottom after appending', async () => {
      mockWsGetState.mockReturnValue('connected');
      const transcriptContainer = document.createElement('div');
      // Set up a scrollable container
      Object.defineProperty(transcriptContainer, 'scrollHeight', { value: 200 });
      transcriptContainer.scrollTop = 0;

      new PeekabooFlow({
        micButton,
        mediaContainer,
        transcriptContainer,
        useWebSocket: true,
      });

      wsCallbacks.onTranscript('show me a cat');

      // Verify scrollTop was set to scrollHeight
      expect(transcriptContainer.scrollTop).toBe(200);
    });
  });
});
