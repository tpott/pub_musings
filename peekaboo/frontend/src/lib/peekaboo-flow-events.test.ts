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

describe('PeekabooFlow events', () => {
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
