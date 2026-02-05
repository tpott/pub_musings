# Spec: No-Mocks E2E Test for Peekaboo

## Goal
Create an e2e test that uses real services (backend, whisper-server, Anthropic LLM) with only the browser's MediaRecorder mocked to read from a pre-recorded audio file.

## Approach
- Mock MediaRecorder to read from `tests/fixtures/me-show-me-a-cat.webm`
- Chunk the audio at 500ms intervals (same as real WebSocket mode)
- Send chunks to real backend via real WebSocket
- Backend processes through real whisper-server and Anthropic API
- Verify cat media is displayed from real SQLite database

## Files to Create

### `frontend/tests/e2e/real-services.spec.ts`

```typescript
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
    const audioPath = path.join(fixturesDir, 'me-show-me-a-cat.webm');
    const audioData = fs.readFileSync(audioPath);

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
        mimeType = 'audio/webm';

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
```

## Prerequisites for Running

1. **Backend running**: `cd backend && go run main.go`
2. **Frontend dev server**: `cd frontend && npm run dev`
3. **Whisper-server reachable**: at URL in `.env` (`WHISPER_SERVER_URL`)
4. **Anthropic API key**: set in `.env`
5. **Media seeded**: `data/media/cat/` must have media files

## Running the Test

```bash
cd frontend
npx playwright test real-services.spec.ts
```

## Verification

1. Test passes with cat media displayed
2. Backend logs show:
   - WebSocket connection from test
   - Audio forwarded to whisper-server
   - Transcript received (should contain "cat")
   - LLM intent extraction called
   - Media lookup for "cat" concept
3. No mocked API routes (only MediaRecorder mocked)

## Technical Details

### Why Mock MediaRecorder?
- Playwright runs in a browser context without real microphone access
- We need to inject pre-recorded audio as if it came from the mic
- Everything after MediaRecorder (WebSocket, backend, external services) is real

### Chunking Strategy
- Real MediaRecorder in WebSocket mode uses 500ms timeslice
- Audio file (~28KB) split into ~4KB chunks (7 chunks)
- Chunks emitted at 500ms intervals via `setInterval`
- Remaining chunks flushed on `stop()`

### Timing
- Chunk transmission: ~3.5 seconds
- Whisper transcription: 1-5 seconds
- LLM intent extraction: 0.5-2 seconds
- Total timeout: 30 seconds for safety
