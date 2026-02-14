/**
 * ScriptedMediaRecorder for PBT tests.
 *
 * Injected via page.addInitScript(). Reads sessions from
 * window.__PBT_SESSIONS__ (array of string[][] — each session is an
 * array of base64 chunks). On each start() call, advances to the next
 * session and emits chunks at timeslice intervals. On stop(), flushes
 * remaining chunks.
 *
 * This generalizes the FileMediaRecorder from real-services.spec.ts
 * to support multiple sequential sessions within one page lifecycle.
 */

/** Returns a function suitable for page.addInitScript(). */
export function getScriptedRecorderScript(): () => void {
  return () => {
    // window.__PBT_SESSIONS__: string[][] — set by runner before navigation
    // Each session is an array of base64-encoded audio chunks
    const w = window as any;
    if (!w.__PBT_SESSIONS__) {
      w.__PBT_SESSIONS__ = [];
    }
    let sessionIdx = 0;

    class ScriptedMediaRecorder {
      private chunkIdx = 0;
      private currentChunks: Uint8Array[] = [];
      private intervalId: ReturnType<typeof setInterval> | null = null;
      ondataavailable: ((e: { data: Blob }) => void) | null = null;
      onstop: (() => void) | null = null;
      state = 'inactive';
      mimeType = 'audio/webm;codecs=opus';

      constructor(_stream: MediaStream, opts?: { mimeType?: string }) {
        if (opts?.mimeType) this.mimeType = opts.mimeType;
      }

      static isTypeSupported(t: string) {
        return t.includes('audio/webm');
      }

      start(timeslice?: number) {
        const sessions: string[][] = w.__PBT_SESSIONS__ || [];
        if (sessionIdx < sessions.length) {
          this.currentChunks = sessions[sessionIdx].map((b64: string) =>
            Uint8Array.from(atob(b64), (c: string) => c.charCodeAt(0)),
          );
          sessionIdx++;
        } else {
          this.currentChunks = [];
        }
        this.chunkIdx = 0;
        this.state = 'recording';

        if (timeslice && timeslice > 0) {
          this.intervalId = setInterval(() => {
            if (this.state === 'recording' && this.chunkIdx < this.currentChunks.length) {
              this.ondataavailable?.({
                data: new Blob([this.currentChunks[this.chunkIdx++]], {
                  type: this.mimeType,
                }),
              });
            }
          }, timeslice);
        }
      }

      stop() {
        if (this.intervalId) clearInterval(this.intervalId);
        this.intervalId = null;
        this.state = 'inactive';
        // Flush remaining chunks
        while (this.chunkIdx < this.currentChunks.length) {
          this.ondataavailable?.({
            data: new Blob([this.currentChunks[this.chunkIdx++]], {
              type: this.mimeType,
            }),
          });
        }
        this.onstop?.();
      }
    }

    const mockStream = {
      getTracks: () => [{ stop: () => {} }],
      getAudioTracks: () => [{ stop: () => {}, enabled: true }],
      getVideoTracks: () => [],
      active: true,
      id: 'pbt-stream',
    };

    navigator.mediaDevices.getUserMedia = () =>
      Promise.resolve(mockStream as unknown as MediaStream);
    (w as any).MediaRecorder = ScriptedMediaRecorder;
  };
}
