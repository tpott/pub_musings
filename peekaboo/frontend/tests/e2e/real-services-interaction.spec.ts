import { test, expect } from '@playwright/test';
import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const fixturesDir = path.join(__dirname, '..', '..', '..', 'tests', 'fixtures');

// DB path matches .env (DB_PATH=backend/data/peekaboo.db, relative to project root)
const projectRoot = path.join(__dirname, '..', '..', '..');
const dbPath = path.join(projectRoot, 'backend', 'data', 'peekaboo.db');

/** Query the SQLite database and return parsed JSON rows. */
function queryDB(sql: string): Record<string, unknown>[] {
  const result = execSync(`sqlite3 -json "${dbPath}" "${sql}"`, {
    encoding: 'utf-8',
    timeout: 5000,
  });
  if (!result.trim()) return [];
  return JSON.parse(result);
}

/** Execute a non-query SQL statement against the SQLite database. */
function execDB(sql: string): void {
  execSync(`sqlite3 "${dbPath}" "${sql}"`, { timeout: 5000 });
}

/** Generate a unique test email to avoid collisions between runs. */
function testEmail(): string {
  return `e2e-interaction-${Date.now()}@test.local`;
}

/**
 * Register a user, verify email via DB, login, and return the user_id.
 * The page will have the session cookie set after this function returns.
 */
async function registerAndLogin(
  page: import('@playwright/test').Page,
  email: string,
  password: string,
): Promise<string> {
  // Register via API
  const registerResp = await page.request.post('/api/auth/register', {
    data: { email, password },
  });
  expect(registerResp.status()).toBe(201);

  // Verify email directly in DB (dev mode — no real email sent)
  execDB(`UPDATE users SET email_verified = 1 WHERE email = '${email}'`);

  // Login via API — sets session cookie on the page context
  const loginResp = await page.request.post('/api/auth/login', {
    data: { email, password },
  });
  expect(loginResp.status()).toBe(200);
  const loginBody = await loginResp.json();

  return loginBody.user.id as string;
}

/**
 * Inject a FileMediaRecorder mock that replays the fixture audio file.
 * Only MediaRecorder is mocked — everything else (WS, backend, whisper, LLM) is real.
 */
async function injectFileMediaRecorder(page: import('@playwright/test').Page): Promise<void> {
  const audioPath = path.join(fixturesDir, 'me-show-me-a-cat.webm');
  const audioData = fs.readFileSync(audioPath);

  const chunkSize = 4096;
  const chunksB64: string[] = [];
  for (let i = 0; i < audioData.length; i += chunkSize) {
    chunksB64.push(audioData.subarray(i, i + chunkSize).toString('base64'));
  }

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
    (window as unknown as Record<string, unknown>).MediaRecorder = FileMediaRecorder;
  }, chunksB64);
}

test.describe('Interaction logging (real services)', () => {

  test('authenticated voice command creates interaction row with user_id', async ({ page }) => {
    const email = testEmail();
    const password = 'test-password-secure-123';

    // Register and login (sets session cookie)
    const userId = await registerAndLogin(page, email, password);

    // Inject MediaRecorder mock
    await injectFileMediaRecorder(page);

    // Navigate to app — WebSocket connects with session cookie (authenticated)
    await page.goto('/');
    const micButton = page.locator('[data-testid="mic-button"]');
    await expect(micButton).toBeVisible();

    // Record timestamp before voice command for DB query filtering
    const beforeCommand = new Date().toISOString();

    // Start recording — backend auto-processes after 3s of audio
    await micButton.click();

    // Wait for media to appear (whisper + LLM processing)
    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toBeVisible({ timeout: 30000 });

    // Give the deferred saveInteraction a moment to write
    await page.waitForTimeout(1000);

    // Query the interactions table for rows matching this user
    const rows = queryDB(
      `SELECT id, user_id, stt_transcript, action_type, llm_model, total_latency_ms ` +
      `FROM interactions ` +
      `WHERE user_id = '${userId}' AND created_at >= '${beforeCommand}' ` +
      `ORDER BY created_at DESC LIMIT 1`
    );

    expect(rows.length).toBeGreaterThanOrEqual(1);

    const row = rows[0];
    expect(row.user_id).toBe(userId);
    expect(row.stt_transcript).toBeTruthy();
    expect(typeof row.stt_transcript).toBe('string');
    expect((row.stt_transcript as string).length).toBeGreaterThan(0);
    expect(['show_media', 'text_to_speech', 'wait_for_more']).toContain(row.action_type);
    expect(row.llm_model).toBeTruthy();
    expect(typeof row.llm_model).toBe('string');
    expect(row.total_latency_ms).toBeTruthy();
    expect(Number(row.total_latency_ms)).toBeGreaterThan(0);

    // Cleanup: remove test user (cascading to sessions, etc.)
    execDB(`DELETE FROM interactions WHERE user_id = '${userId}'`);
    execDB(`DELETE FROM sessions WHERE user_id = '${userId}'`);
    execDB(`DELETE FROM users WHERE id = '${userId}'`);
  });

  test('INTERACTION_LOG_AUDIO=true creates audio blob file', async ({ page }) => {
    // This test requires the backend to be started with INTERACTION_LOG_AUDIO=true.
    // Skip if the env var is not set (detected by checking for any recent audio blobs
    // or checking the env — we infer from the DB after the interaction).

    const email = testEmail();
    const password = 'test-password-secure-456';

    const userId = await registerAndLogin(page, email, password);

    await injectFileMediaRecorder(page);

    await page.goto('/');
    const micButton = page.locator('[data-testid="mic-button"]');
    await expect(micButton).toBeVisible();

    const beforeCommand = new Date().toISOString();
    await micButton.click();

    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toBeVisible({ timeout: 30000 });

    // Wait for deferred save + async audio blob write
    await page.waitForTimeout(2000);

    const rows = queryDB(
      `SELECT id, user_id, audio_blob_path ` +
      `FROM interactions ` +
      `WHERE user_id = '${userId}' AND created_at >= '${beforeCommand}' ` +
      `ORDER BY created_at DESC LIMIT 1`
    );

    expect(rows.length).toBeGreaterThanOrEqual(1);
    const row = rows[0];

    if (row.audio_blob_path === null) {
      // INTERACTION_LOG_AUDIO is not enabled — skip assertions about file
      test.skip(true, 'INTERACTION_LOG_AUDIO is not enabled on the running backend');
    }

    // Verify the audio blob file exists on disk
    const blobPath = path.join(projectRoot, 'backend', row.audio_blob_path as string);
    expect(fs.existsSync(blobPath)).toBe(true);

    const stat = fs.statSync(blobPath);
    expect(stat.size).toBeGreaterThan(0);
    expect((row.audio_blob_path as string).endsWith('.webm')).toBe(true);

    // Cleanup
    if (fs.existsSync(blobPath)) fs.unlinkSync(blobPath);
    execDB(`DELETE FROM interactions WHERE user_id = '${userId}'`);
    execDB(`DELETE FROM sessions WHERE user_id = '${userId}'`);
    execDB(`DELETE FROM users WHERE id = '${userId}'`);
  });
});
