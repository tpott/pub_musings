/**
 * Shared MockMediaRecorder implementations for E2E tests.
 *
 * Playwright's addInitScript() runs in the browser context, so we export
 * functions that return the script content to be injected.
 *
 * Usage in tests:
 *   import { getSimpleMockScript, getWebSocketMockScript } from '../helpers/mock-media-recorder';
 *   await page.addInitScript(getSimpleMockScript());
 *   // or
 *   await page.addInitScript(getWebSocketMockScript());
 */

/**
 * Simple MockMediaRecorder for HTTP mode tests.
 * Emits a single blob when stop() is called.
 */
export function getSimpleMockScript(): () => void {
  return () => {
    class MockMediaRecorder {
      state = 'inactive';
      ondataavailable: ((event: { data: Blob }) => void) | null = null;
      onstop: (() => void) | null = null;
      stream: MediaStream | null = null;

      constructor(stream: MediaStream) {
        this.stream = stream;
      }

      static isTypeSupported(type: string) {
        return type === 'audio/webm' || type === 'audio/webm;codecs=opus';
      }

      start() {
        this.state = 'recording';
      }

      stop() {
        this.state = 'inactive';
        setTimeout(() => {
          if (this.ondataavailable) {
            this.ondataavailable({ data: new Blob(['fake audio'], { type: 'audio/webm' }) });
          }
          if (this.onstop) {
            this.onstop();
          }
        }, 10);
      }
    }

    const mockStream = {
      getTracks: () => [{ stop: () => {} }],
      getAudioTracks: () => [{ stop: () => {}, enabled: true }],
      getVideoTracks: () => [],
      active: true,
      id: 'mock-stream-id',
    };

    navigator.mediaDevices.getUserMedia = () =>
      Promise.resolve(mockStream as unknown as MediaStream);
    (window as any).MediaRecorder = MockMediaRecorder;
  };
}

/**
 * WebSocket-capable MockMediaRecorder for continuous audio streaming tests.
 * Supports timeslice parameter to emit periodic audio chunks.
 */
export function getWebSocketMockScript(): () => void {
  return () => {
    class MockMediaRecorder {
      state = 'inactive' as string;
      ondataavailable: ((event: { data: Blob }) => void) | null = null;
      onstop: (() => void) | null = null;
      stream: MediaStream | null = null;
      mimeType = 'audio/webm';
      private intervalId: ReturnType<typeof setInterval> | null = null;

      constructor(stream: MediaStream, options?: { mimeType?: string }) {
        this.stream = stream;
        if (options?.mimeType) this.mimeType = options.mimeType;
      }

      static isTypeSupported(type: string) {
        return type === 'audio/webm' || type === 'audio/webm;codecs=opus';
      }

      start(timeslice?: number) {
        this.state = 'recording';
        if (timeslice && timeslice > 0) {
          this.intervalId = setInterval(() => {
            if (this.state === 'recording' && this.ondataavailable) {
              this.ondataavailable({
                data: new Blob(['audio chunk'], { type: this.mimeType }),
              });
            }
          }, timeslice);
        }
      }

      stop() {
        if (this.intervalId) {
          clearInterval(this.intervalId);
          this.intervalId = null;
        }
        this.state = 'inactive';
        if (this.ondataavailable) {
          this.ondataavailable({
            data: new Blob(['final audio'], { type: this.mimeType }),
          });
        }
        if (this.onstop) {
          this.onstop();
        }
      }
    }

    const mockStream = {
      getTracks: () => [{ stop: () => {} }],
      getAudioTracks: () => [{ stop: () => {}, enabled: true }],
      getVideoTracks: () => [],
      active: true,
      id: 'mock-stream-id',
    };

    navigator.mediaDevices.getUserMedia = () =>
      Promise.resolve(mockStream as unknown as MediaStream);
    (window as any).MediaRecorder = MockMediaRecorder;
  };
}

/**
 * Mock for testing microphone permission denial.
 * Sets up MockMediaRecorder for browser support check, but getUserMedia rejects.
 */
export function getPermissionDeniedMockScript(): () => void {
  return () => {
    // MockMediaRecorder is needed so browser support check passes
    class MockMediaRecorder {
      state = 'inactive';
      ondataavailable: ((event: { data: Blob }) => void) | null = null;
      onstop: (() => void) | null = null;
      stream: MediaStream | null = null;

      constructor(stream: MediaStream) {
        this.stream = stream;
      }

      static isTypeSupported(type: string) {
        return type === 'audio/webm' || type === 'audio/webm;codecs=opus';
      }

      start() {
        this.state = 'recording';
      }

      stop() {
        this.state = 'inactive';
        if (this.ondataavailable) {
          this.ondataavailable({ data: new Blob(['fake audio'], { type: 'audio/webm' }) });
        }
        if (this.onstop) {
          this.onstop();
        }
      }
    }

    (window as any).MediaRecorder = MockMediaRecorder;

    // getUserMedia rejects with NotAllowedError (permission denied)
    navigator.mediaDevices.getUserMedia = () => {
      const error = new DOMException('Permission denied', 'NotAllowedError');
      return Promise.reject(error);
    };
  };
}
