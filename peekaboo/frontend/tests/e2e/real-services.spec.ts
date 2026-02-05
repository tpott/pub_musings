import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const fixturesDir = path.join(__dirname, '..', '..', '..', 'tests', 'fixtures');

test.describe('Real services e2e', () => {
  test('voice command "show me a cat" with real whisper and LLM', async ({ page }) => {
    // Read audio file and split into ~4KB chunks
    // Note: webm/opus works with whisper-server when started with --convert flag
    const audioPath = path.join(fixturesDir, 'me-show-me-a-cat.webm');
    const audioData = fs.readFileSync(audioPath);

    // webm file is ~28KB, split into ~4KB chunks (7 chunks at 500ms = 3.5s)
    const chunkSize = 4096;
    const chunksB64: string[] = [];
    for (let i = 0; i < audioData.length; i += chunkSize) {
      chunksB64.push(audioData.subarray(i, i + chunkSize).toString('base64'));
    }

    // Mock only MediaRecorder to read from file with 500ms chunking
    await page.addInitScript((chunks: string[]) => {
      const audioChunks = chunks.map(b64 =>
        Uint8Array.from(atob(b64), c => c.charCodeAt(0))
      );

      class FileMediaRecorder {
        private idx = 0;
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
          this.state = 'recording';
          this.idx = 0;
          if (timeslice && timeslice > 0) {
            this.intervalId = setInterval(() => {
              if (this.state === 'recording' && this.idx < audioChunks.length) {
                this.ondataavailable?.({
                  data: new Blob([audioChunks[this.idx++]], { type: this.mimeType })
                });
              }
            }, timeslice);
          }
        }

        stop() {
          if (this.intervalId) clearInterval(this.intervalId);
          this.state = 'inactive';
          // Emit remaining chunks
          while (this.idx < audioChunks.length) {
            this.ondataavailable?.({
              data: new Blob([audioChunks[this.idx++]], { type: this.mimeType })
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
        id: 'file-stream',
      };

      navigator.mediaDevices.getUserMedia = () =>
        Promise.resolve(mockStream as unknown as MediaStream);
      (window as any).MediaRecorder = FileMediaRecorder;
    }, chunksB64);

    // Navigate to app (WebSocket mode is now the default)
    await page.goto('/');

    // Wait for UI
    const micButton = page.locator('[data-testid="mic-button"]');
    await expect(micButton).toBeVisible();

    // Start recording - backend should auto-process after 3 seconds of audio
    // (per websocket-audio.md spec: "Time threshold - 3 seconds of audio accumulated")
    await micButton.click();

    // Wait for:
    // - 3+ seconds of audio chunks to trigger threshold
    // - Whisper transcription (1-5s)
    // - LLM intent extraction (0.5-2s)
    // - Media lookup and response
    // Total expected: ~10-15 seconds, timeout at 30s for safety
    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toBeVisible({ timeout: 30000 });

    // Verify cat media from real database
    const src = await img.getAttribute('src');
    expect(src).toContain('/data/media/cat/');
  });
});
