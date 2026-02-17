import { describe, it, expect, vi, beforeEach, afterEach, Mock } from 'vitest';
import { PeekabooFlow, FlowState } from './peekaboo-flow';
import * as audioRecorder from './audio-recorder';
import * as intent from './intent';
import * as mediaDisplay from './media-display';
import * as textToSpeech from './text-to-speech';
import * as websocketAudio from './websocket-audio';

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

describe('PeekabooFlow submitText', () => {
  let micButton: HTMLButtonElement;
  let mediaContainer: HTMLElement;
  let stateChanges: FlowState[];
  let flow: PeekabooFlow;
  let mockShow: Mock;
  let mockReset: Mock;

  beforeEach(() => {
    const mockStartRecording = vi.fn().mockResolvedValue(undefined);
    const mockStopRecording = vi.fn().mockResolvedValue({
      blob: new Blob(['mock audio'], { type: 'audio/webm' }),
      mimeType: 'audio/webm',
    });
    mockShow = vi.fn();
    mockReset = vi.fn();

    (audioRecorder.AudioRecorder as unknown as Mock).mockImplementation(() => ({
      startRecording: mockStartRecording,
      stopRecording: mockStopRecording,
      isRecording: vi.fn().mockReturnValue(false),
      destroy: vi.fn(),
    }));

    (intent.extractIntent as Mock).mockResolvedValue({ subject: 'cat' });

    (mediaDisplay.MediaDisplay as unknown as Mock).mockImplementation(() => ({
      show: mockShow,
      reset: mockReset,
      stopAudio: vi.fn(),
      getAudioElement: vi.fn(),
    }));

    (mediaDisplay.fetchMedia as Mock).mockResolvedValue({
      photoUrl: '/data/media/cat/photo.jpg',
      audioUrl: '/data/media/cat/audio.mp3',
    });

    (textToSpeech.speakSubject as Mock).mockResolvedValue(null);

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

  it('transitions through searching to displaying state', async () => {
    await flow.submitText('show me a cat');

    expect(stateChanges).toEqual(['searching', 'displaying']);
    expect(flow.getState()).toBe('displaying');
  });

  it('calls extractIntent with the provided text', async () => {
    await flow.submitText('show me a dog');

    expect(intent.extractIntent).toHaveBeenCalledWith('show me a dog');
  });

  it('calls fetchMedia with the extracted subject', async () => {
    await flow.submitText('show me a cat');

    expect(mediaDisplay.fetchMedia).toHaveBeenCalledWith('cat');
  });

  it('calls display.show with media and subject', async () => {
    await flow.submitText('show me a cat');

    expect(mockShow).toHaveBeenCalledWith(
      { photoUrl: '/data/media/cat/photo.jpg', audioUrl: '/data/media/cat/audio.mp3' },
      'cat'
    );
  });

  it('does not call transcribeAudio (bypasses audio)', async () => {
    await flow.submitText('show me a cat');

    expect(audioRecorder.transcribeAudio).not.toHaveBeenCalled();
  });

  it('transitions to error state on intent failure', async () => {
    (intent.extractIntent as Mock).mockRejectedValue(new Error('Intent failed'));

    await flow.submitText('show me a cat');

    expect(flow.getState()).toBe('error');
  });

  it('transitions to error state on media fetch failure', async () => {
    (mediaDisplay.fetchMedia as Mock).mockRejectedValue(new Error('Media not found'));

    await flow.submitText('show me a cat');

    expect(flow.getState()).toBe('error');
  });

  it('is a no-op during transcribing state', async () => {
    // Simulate being in transcribing state by starting a recording flow
    // We need to get the flow into transcribing state
    (audioRecorder.transcribeAudio as Mock).mockImplementation(
      () => new Promise(() => {}) // never resolves
    );

    await flow.startRecording();
    // Start stop without awaiting — this puts us in transcribing
    flow.stopRecordingAndProcess();
    await new Promise(resolve => setTimeout(resolve, 0));
    expect(flow.getState()).toBe('transcribing');

    stateChanges.length = 0;
    await flow.submitText('show me a cat');

    // Should not have changed state
    expect(stateChanges).toHaveLength(0);
  });

  it('works from displaying state (re-submit)', async () => {
    await flow.submitText('show me a cat');
    expect(flow.getState()).toBe('displaying');

    stateChanges.length = 0;
    (intent.extractIntent as Mock).mockResolvedValue({ subject: 'dog' });
    (mediaDisplay.fetchMedia as Mock).mockResolvedValue({
      photoUrl: '/data/media/dog/photo.jpg',
      audioUrl: '/data/media/dog/audio.mp3',
    });

    await flow.submitText('show me a dog');

    expect(stateChanges).toEqual(['searching', 'displaying']);
    expect(intent.extractIntent).toHaveBeenCalledWith('show me a dog');
  });

  it('attempts TTS after displaying media', async () => {
    await flow.submitText('show me a cat');

    expect(textToSpeech.speakSubject).toHaveBeenCalledWith('cat');
  });

  it('does not fail if TTS throws', async () => {
    (textToSpeech.speakSubject as Mock).mockRejectedValue(new Error('TTS unavailable'));

    await flow.submitText('show me a cat');

    expect(flow.getState()).toBe('displaying');
  });
});
