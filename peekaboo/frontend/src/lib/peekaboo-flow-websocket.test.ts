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

  describe('TTS audio handling', () => {
    it('plays TTS audio when tts_audio message received', async () => {
      mockWsGetState.mockReturnValue('connected');
      const mockPlay = vi.fn().mockResolvedValue(undefined);
      const mockAudioInstance = {
        play: mockPlay, pause: vi.fn(),
        addEventListener: vi.fn(), src: '',
      };
      (global as any).Audio = vi.fn(() => mockAudioInstance);

      new PeekabooFlow({ micButton, mediaContainer, useWebSocket: true });

      wsCallbacks.onTTSAudio({
        type: 'tts_audio', audio_data: 'dGVzdA==', text: 'Here is a cat!',
      });

      expect(global.Audio).toHaveBeenCalledWith('data:audio/wav;base64,dGVzdA==');
      expect(mockPlay).toHaveBeenCalled();
    });

    it('waits for TTS to finish before showing media', async () => {
      mockWsGetState.mockReturnValue('connected');
      let endedCallback: (() => void) | null = null;
      const mockAudioInstance = {
        play: vi.fn().mockResolvedValue(undefined), pause: vi.fn(),
        addEventListener: vi.fn((event: string, cb: () => void) => {
          if (event === 'ended') endedCallback = cb;
        }),
        src: '',
      };
      (global as any).Audio = vi.fn(() => mockAudioInstance);

      const flow = new PeekabooFlow({ micButton, mediaContainer, useWebSocket: true });
      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      wsCallbacks.onTTSAudio({
        type: 'tts_audio', audio_data: 'dGVzdA==', text: 'Here is a cat!',
      });

      const mediaPromise = wsCallbacks.onMedia({
        type: 'media', subject: 'cat', photo_url: '/cat.jpg',
      });

      // Media display should not be called yet (TTS still playing)
      expect(mockShow).not.toHaveBeenCalled();

      // Simulate TTS playback ending
      endedCallback?.();
      await mediaPromise;

      expect(mockShow).toHaveBeenCalled();
    });

    it('cleans up TTS audio on destroy', () => {
      mockWsGetState.mockReturnValue('connected');
      const mockPause = vi.fn();
      const mockAudioInstance = {
        play: vi.fn().mockResolvedValue(undefined), pause: mockPause,
        addEventListener: vi.fn(), src: '',
      };
      (global as any).Audio = vi.fn(() => mockAudioInstance);

      const flow = new PeekabooFlow({ micButton, mediaContainer, useWebSocket: true });
      wsCallbacks.onTTSAudio({
        type: 'tts_audio', audio_data: 'dGVzdA==', text: 'Hello!',
      });

      flow.destroy();
      expect(mockPause).toHaveBeenCalled();
    });

    it('skips client-side TTS when server TTS was played', async () => {
      mockWsGetState.mockReturnValue('connected');
      let endedCallback: (() => void) | null = null;
      const mockAudioInstance = {
        play: vi.fn().mockResolvedValue(undefined), pause: vi.fn(),
        addEventListener: vi.fn((event: string, cb: () => void) => {
          if (event === 'ended') endedCallback = cb;
        }),
        src: '',
      };
      (global as any).Audio = vi.fn(() => mockAudioInstance);

      const flow = new PeekabooFlow({ micButton, mediaContainer, useWebSocket: true });
      await flow.startRecording();
      await flow.stopRecordingAndProcess();

      wsCallbacks.onTTSAudio({
        type: 'tts_audio', audio_data: 'dGVzdA==', text: 'Here is a cat!',
      });

      endedCallback?.();
      await wsCallbacks.onMedia({
        type: 'media', subject: 'cat', photo_url: '/cat.jpg',
      });

      // speakSubject should NOT be called when server TTS was used
      expect(textToSpeech.speakSubject).not.toHaveBeenCalled();
    });
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
