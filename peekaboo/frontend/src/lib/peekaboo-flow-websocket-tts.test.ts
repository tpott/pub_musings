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
  cleanupTTSBlobUrls: vi.fn(),
}));

vi.mock('./websocket-audio', () => ({
  AudioWebSocket: vi.fn(),
}));

describe('PeekabooFlow WebSocket TTS and transcript', () => {
  let micButton: HTMLButtonElement;
  let mediaContainer: HTMLElement;

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

    // Setup AudioRecorder constructor mock
    (audioRecorder.AudioRecorder as unknown as Mock).mockImplementation(() => ({
      startRecording: vi.fn(),
      stopRecording: vi.fn(),
      isRecording: vi.fn().mockReturnValue(false),
      destroy: vi.fn(),
    }));

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
    const MockMediaRecorder = vi.fn().mockImplementation(() => ({
      start: vi.fn(),
      stop: vi.fn(),
      state: 'inactive',
      ondataavailable: null,
    }));
    MockMediaRecorder.isTypeSupported = vi.fn().mockReturnValue(true);
    (global as any).MediaRecorder = MockMediaRecorder;

    // Setup getUserMedia mock
    (global as any).navigator = {
      mediaDevices: {
        getUserMedia: vi.fn().mockResolvedValue({
          getTracks: vi.fn().mockReturnValue([{ stop: vi.fn() }]),
        }),
      },
    };
  });

  afterEach(() => {
    vi.clearAllMocks();
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
