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
    // Note: Using wav format because the whisper server doesn't support webm/opus
    const audioPath = path.join(fixturesDir, 'me-show-me-a-cat.wav');
    const audioData = fs.readFileSync(audioPath);

    // Use larger chunks to reduce count - wav file is ~200KB, we want ~7 chunks to match timing
    const chunkSize = 32768; // 32KB chunks
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
        // Use audio/wav since that's what we're actually sending
        mimeType = 'audio/wav';

        constructor(_stream: MediaStream, opts?: { mimeType?: string }) {
          // Ignore requested mimeType, always use wav since that's our fixture format
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

    // Navigate with WebSocket mode
    await page.goto('/?useWebSocket=true');

    // Wait for UI
    const micButton = page.locator('[data-testid="mic-button"]');
    await expect(micButton).toBeVisible();

    // Start recording
    await micButton.click();

    // Wait for all chunks to send (~7 chunks at 500ms = 3.5s)
    await page.waitForTimeout(4000);

    // Stop recording
    await micButton.click();

    // Wait for real services to process (whisper + LLM can take 5-15s)
    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toBeVisible({ timeout: 30000 });

    // Verify cat media from real database
    const src = await img.getAttribute('src');
    expect(src).toContain('/data/media/cat/');
  });
});
