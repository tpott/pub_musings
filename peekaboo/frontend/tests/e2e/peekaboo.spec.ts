import { test, expect, Page } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const fixturesDir = path.join(__dirname, '..', '..', '..', 'tests', 'fixtures');

// Helper to set up mocks and test animal media display
async function setupMocksAndTestAnimal(
  page: Page,
  animal: string,
  phrase: string
) {
  // Mock MediaRecorder and getUserMedia BEFORE page loads
  await page.addInitScript(() => {
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

    navigator.mediaDevices.getUserMedia = () => Promise.resolve(mockStream as unknown as MediaStream);
    (window as any).MediaRecorder = MockMediaRecorder;
  });

  // Mock transcribe API
  await page.route('**/api/transcribe', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ text: phrase }),
    })
  );

  // Mock intent API
  await page.route('**/api/intent', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ subject: animal }),
    })
  );

  // Mock media endpoint
  await page.route(`**/api/media/${animal}`, route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        photo_url: `/fixtures/mock-${animal}-photo.jpg`,
        audio_url: `/fixtures/mock-${animal}-audio.mp3`,
      }),
    })
  );

  // Serve test fixtures as static files
  await page.route('**/fixtures/**', async route => {
    const url = new URL(route.request().url());
    const filePath = path.join(fixturesDir, url.pathname.replace('/fixtures/', ''));
    await route.fulfill({ path: filePath });
  });
}

test.describe('Peekaboo voice command flow', () => {
  test('voice command shows cat media', async ({ page }) => {
    // Mock MediaRecorder and getUserMedia BEFORE page loads
    await page.addInitScript(() => {
      // Mock MediaRecorder
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
          // Emit a fake audio blob after a short delay
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

      // Mock getUserMedia to return a fake stream
      const mockStream = {
        getTracks: () => [{ stop: () => {} }],
        getAudioTracks: () => [{ stop: () => {}, enabled: true }],
        getVideoTracks: () => [],
        active: true,
        id: 'mock-stream-id',
      };

      navigator.mediaDevices.getUserMedia = () => Promise.resolve(mockStream as unknown as MediaStream);
      (window as any).MediaRecorder = MockMediaRecorder;
    });

    // Mock transcribe API to return "show me a cat"
    await page.route('**/api/transcribe', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ text: 'show me a cat' }),
      })
    );

    // Mock intent API to return subject "cat"
    await page.route('**/api/intent', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ subject: 'cat' }),
      })
    );

    // Mock media endpoint to return test fixtures
    await page.route('**/api/media/cat', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          photo_url: '/fixtures/mock-cat-photo.jpg',
          audio_url: '/fixtures/mock-cat-audio.mp3',
        }),
      })
    );

    // Serve test fixtures as static files
    await page.route('**/fixtures/**', async route => {
      const url = new URL(route.request().url());
      const filePath = path.join(fixturesDir, url.pathname.replace('/fixtures/', ''));
      await route.fulfill({ path: filePath });
    });

    // Navigate to the app
    await page.goto('/');

    // Wait for page to load
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();
    await expect(page.locator('[data-testid="media-display"]')).toBeVisible();

    // Click and hold the mic button to start recording (use mouse events that the app listens for)
    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.dispatchEvent('mousedown');

    // Wait for recording state
    await page.waitForTimeout(200);

    // Release to trigger processing (use mouseup event)
    await micButton.dispatchEvent('mouseup');

    // Wait for the media to display
    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    // Verify the image is displayed with cat photo
    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    // Verify audio element was created
    const audio = page.locator('[data-testid="media-audio"]');
    await expect(audio).toHaveAttribute('src', '/fixtures/mock-cat-audio.mp3');
  });

  test('displays error state on API failure', async ({ page }) => {
    // Mock MediaRecorder BEFORE page loads
    await page.addInitScript(() => {
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

      navigator.mediaDevices.getUserMedia = () => Promise.resolve(mockStream as unknown as MediaStream);
      (window as any).MediaRecorder = MockMediaRecorder;
    });

    // Mock transcribe API to fail
    await page.route('**/api/transcribe', route =>
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Transcription failed' }),
      })
    );

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.dispatchEvent('mousedown');
    await page.waitForTimeout(200);
    await micButton.dispatchEvent('mouseup');

    // Wait for error processing
    await page.waitForTimeout(1000);

    // Media should not be displayed after an error
    await expect(page.locator('[data-testid="media-image"]')).not.toBeVisible();
  });

  test('voice command shows dog media', async ({ page }) => {
    await setupMocksAndTestAnimal(page, 'dog', 'show me a dog');

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();
    await expect(page.locator('[data-testid="media-display"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.dispatchEvent('mousedown');
    await page.waitForTimeout(200);
    await micButton.dispatchEvent('mouseup');

    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toHaveAttribute('src', '/fixtures/mock-dog-photo.jpg');

    const audio = page.locator('[data-testid="media-audio"]');
    await expect(audio).toHaveAttribute('src', '/fixtures/mock-dog-audio.mp3');
  });

  test('voice command shows duck media', async ({ page }) => {
    await setupMocksAndTestAnimal(page, 'duck', 'I want to see a duck');

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();
    await expect(page.locator('[data-testid="media-display"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.dispatchEvent('mousedown');
    await page.waitForTimeout(200);
    await micButton.dispatchEvent('mouseup');

    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toHaveAttribute('src', '/fixtures/mock-duck-photo.jpg');

    const audio = page.locator('[data-testid="media-audio"]');
    await expect(audio).toHaveAttribute('src', '/fixtures/mock-duck-audio.mp3');
  });

  test('error recovery: transcribe succeeds but intent fails, then retry succeeds', async ({ page }) => {
    // Track API call count to simulate failure then success
    let intentCallCount = 0;

    // Mock MediaRecorder BEFORE page loads
    await page.addInitScript(() => {
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

      navigator.mediaDevices.getUserMedia = () => Promise.resolve(mockStream as unknown as MediaStream);
      (window as any).MediaRecorder = MockMediaRecorder;
    });

    // Mock transcribe API to always succeed
    await page.route('**/api/transcribe', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ text: 'show me a cat' }),
      })
    );

    // Mock intent API to fail first user attempt, then succeed
    // Use 400 (Bad Request) which is NOT retried by fetchWithRetry (only 429, 500-504 are retried)
    await page.route('**/api/intent', route => {
      intentCallCount++;
      if (intentCallCount === 1) {
        route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'invalid request' }),
        });
      } else {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ subject: 'cat' }),
        });
      }
    });

    // Mock media endpoint
    await page.route('**/api/media/cat', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          photo_url: '/fixtures/mock-cat-photo.jpg',
          audio_url: '/fixtures/mock-cat-audio.mp3',
        }),
      })
    );

    // Serve test fixtures
    await page.route('**/fixtures/**', async route => {
      const url = new URL(route.request().url());
      const filePath = path.join(fixturesDir, url.pathname.replace('/fixtures/', ''));
      await route.fulfill({ path: filePath });
    });

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const mediaDisplay = page.locator('[data-testid="media-display"]');

    // First attempt - should fail on intent
    await micButton.dispatchEvent('mousedown');
    await page.waitForTimeout(200);
    await micButton.dispatchEvent('mouseup');

    // Wait for error state
    await page.waitForTimeout(1000);

    // Verify error message is shown (media display contains error text)
    // 400 errors show the error from the response body
    await expect(mediaDisplay).toContainText(/invalid request|something went wrong|try again/i);

    // Verify mic button indicates retry option via aria-label
    await expect(micButton).toHaveAttribute('aria-label', /try again/i);

    // Image should NOT be visible after error
    await expect(page.locator('[data-testid="media-image"]')).not.toBeVisible();

    // Wait for automatic reset to idle state (3 seconds)
    await page.waitForTimeout(3500);

    // Retry - second attempt should succeed
    await micButton.dispatchEvent('mousedown');
    await page.waitForTimeout(200);
    await micButton.dispatchEvent('mouseup');

    // Wait for success - image should now be visible
    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    // Verify the cat image is displayed after retry
    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');
  });
});
