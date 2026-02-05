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

    // Click the mic button to start recording (toggle behavior)
    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.click();

    // Wait for recording state
    await page.waitForTimeout(200);

    // Click again to stop recording and trigger processing
    await micButton.click();

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
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

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
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

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
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

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
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

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
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

    // Wait for success - image should now be visible
    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    // Verify the cat image is displayed after retry
    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');
  });

  test('TTS synthesis is called after displaying media', async ({ page }) => {
    let ttsWasCalled = false;
    let ttsText = '';

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

    // Mock transcribe API
    await page.route('**/api/transcribe', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ text: 'show me a cat' }),
      })
    );

    // Mock intent API
    await page.route('**/api/intent', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ subject: 'cat' }),
      })
    );

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

    // Mock TTS speak endpoint - return a minimal WAV file
    await page.route('**/api/speak', async route => {
      const request = route.request();
      const postData = request.postDataJSON();
      ttsWasCalled = true;
      ttsText = postData?.text || '';

      // Create a minimal WAV header (44 bytes) + some silent audio data
      const wavHeader = new Uint8Array([
        0x52, 0x49, 0x46, 0x46, // "RIFF"
        0x24, 0x00, 0x00, 0x00, // File size - 8
        0x57, 0x41, 0x56, 0x45, // "WAVE"
        0x66, 0x6d, 0x74, 0x20, // "fmt "
        0x10, 0x00, 0x00, 0x00, // Subchunk size (16)
        0x01, 0x00,             // Audio format (1 = PCM)
        0x01, 0x00,             // Num channels (1)
        0x44, 0xac, 0x00, 0x00, // Sample rate (44100)
        0x88, 0x58, 0x01, 0x00, // Byte rate
        0x02, 0x00,             // Block align
        0x10, 0x00,             // Bits per sample (16)
        0x64, 0x61, 0x74, 0x61, // "data"
        0x00, 0x00, 0x00, 0x00, // Data size (0 = silent)
      ]);

      route.fulfill({
        status: 200,
        contentType: 'audio/wav',
        body: Buffer.from(wavHeader),
      });
    });

    // Serve test fixtures
    await page.route('**/fixtures/**', async route => {
      const url = new URL(route.request().url());
      const filePath = path.join(fixturesDir, url.pathname.replace('/fixtures/', ''));
      await route.fulfill({ path: filePath });
    });

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

    // Wait for media to display
    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    // Wait a bit more for TTS to be called (it's async after media display)
    await page.waitForTimeout(500);

    // Verify TTS was called with correct phrase
    expect(ttsWasCalled).toBe(true);
    expect(ttsText).toBe('Here is a cat!');
  });
});

test.describe('WebSocket continuous listening', () => {
  test('user can issue multiple commands via WebSocket mode', async ({ page }) => {
    let commandCount = 0;
    const animals = ['cat', 'dog'];

    // Mock MediaRecorder to support streaming with timeslice
    await page.addInitScript(() => {
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
          // When timeslice is provided (WebSocket mode), send periodic chunks
          if (timeslice && timeslice > 0) {
            this.intervalId = setInterval(() => {
              if (this.state === 'recording' && this.ondataavailable) {
                this.ondataavailable({ data: new Blob(['audio chunk'], { type: this.mimeType }) });
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
          // Emit final data chunk
          if (this.ondataavailable) {
            this.ondataavailable({ data: new Blob(['final audio'], { type: this.mimeType }) });
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

      navigator.mediaDevices.getUserMedia = () => Promise.resolve(mockStream as unknown as MediaStream);
      (window as any).MediaRecorder = MockMediaRecorder;
      // Enable WebSocket mode via global flag
      (window as any).__PEEKABOO_USE_WEBSOCKET__ = true;
    });

    // Mock WebSocket with routeWebSocket
    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioReceived = false;

      ws.onMessage(message => {
        // Handle control messages (JSON strings)
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'stop_recording' && audioReceived) {
              const animal = animals[commandCount % animals.length];
              commandCount++;

              // Send transcript
              ws.send(JSON.stringify({ type: 'transcript', text: `show me a ${animal}` }));

              // Send media response after short delay
              setTimeout(() => {
                ws.send(JSON.stringify({
                  type: 'media',
                  subject: animal,
                  photo_url: `/fixtures/mock-${animal}-photo.jpg`,
                  audio_url: `/fixtures/mock-${animal}-audio.mp3`,
                }));
              }, 50);

              // Reset for next command
              audioReceived = false;
            } else if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            }
          } catch {
            // Not valid JSON, might be audio data encoded as string
            audioReceived = true;
          }
        } else {
          // Binary audio data
          audioReceived = true;
        }
      });
    });

    // Serve test fixtures
    await page.route('**/fixtures/**', async route => {
      const url = new URL(route.request().url());
      const filePath = path.join(fixturesDir, url.pathname.replace('/fixtures/', ''));
      await route.fulfill({ path: filePath });
    });

    // Navigate with WebSocket mode enabled
    await page.goto('/?useWebSocket=true');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const img = page.locator('[data-testid="media-image"]');

    // First command: cat
    await micButton.click();
    await page.waitForTimeout(600); // Allow time for audio chunks to send
    await micButton.click();

    // Wait for media to display
    await expect(img).toBeVisible({ timeout: 10000 });
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    // Second command: dog (from displaying state, demonstrating continuous listening)
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    // Wait for dog media
    await expect(img).toHaveAttribute('src', '/fixtures/mock-dog-photo.jpg', { timeout: 10000 });

    // Verify both commands were processed
    expect(commandCount).toBe(2);
  });

  test('handles server sending invalid JSON gracefully', async ({ page }) => {
    let invalidJsonSent = false;
    let commandProcessed = false;

    // Mock MediaRecorder for WebSocket mode
    await page.addInitScript(() => {
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
                this.ondataavailable({ data: new Blob(['audio chunk'], { type: this.mimeType }) });
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
            this.ondataavailable({ data: new Blob(['final audio'], { type: this.mimeType }) });
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

      navigator.mediaDevices.getUserMedia = () => Promise.resolve(mockStream as unknown as MediaStream);
      (window as any).MediaRecorder = MockMediaRecorder;
    });

    // Mock WebSocket - send invalid JSON first, then valid response
    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioReceived = false;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'stop_recording' && audioReceived) {
              // First, send invalid JSON - this should be gracefully ignored
              if (!invalidJsonSent) {
                ws.send('this is { not valid json [[[');
                ws.send('{incomplete json');
                invalidJsonSent = true;
              }

              // Then send valid response
              setTimeout(() => {
                ws.send(JSON.stringify({ type: 'transcript', text: 'show me a cat' }));
                ws.send(JSON.stringify({
                  type: 'media',
                  subject: 'cat',
                  photo_url: '/fixtures/mock-cat-photo.jpg',
                  audio_url: '/fixtures/mock-cat-audio.mp3',
                }));
                commandProcessed = true;
              }, 50);

              audioReceived = false;
            } else if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            }
          } catch {
            audioReceived = true;
          }
        } else {
          audioReceived = true;
        }
      });
    });

    // Serve test fixtures
    await page.route('**/fixtures/**', async route => {
      const url = new URL(route.request().url());
      const filePath = path.join(fixturesDir, url.pathname.replace('/fixtures/', ''));
      await route.fulfill({ path: filePath });
    });

    await page.goto('/?useWebSocket=true');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const img = page.locator('[data-testid="media-image"]');

    // Record a command
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    // Despite invalid JSON being sent, the valid response should be processed
    await expect(img).toBeVisible({ timeout: 10000 });
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    expect(invalidJsonSent).toBe(true);
    expect(commandProcessed).toBe(true);
  });

  test('handles server closing connection mid-recording', async ({ page }) => {
    let closeConnectionOnFirstCommand = true;

    // Mock MediaRecorder for WebSocket mode
    await page.addInitScript(() => {
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
                this.ondataavailable({ data: new Blob(['audio chunk'], { type: this.mimeType }) });
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
            this.ondataavailable({ data: new Blob(['final audio'], { type: this.mimeType }) });
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

      navigator.mediaDevices.getUserMedia = () => Promise.resolve(mockStream as unknown as MediaStream);
      (window as any).MediaRecorder = MockMediaRecorder;
    });

    // Mock WebSocket - close connection on first recording
    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioChunkCount = 0;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            }
          } catch {
            // Not JSON
          }
        } else {
          // Binary audio data
          audioChunkCount++;
          // Close connection after receiving some audio chunks
          if (closeConnectionOnFirstCommand && audioChunkCount >= 2) {
            ws.close();
            closeConnectionOnFirstCommand = false;
          }
        }
      });
    });

    await page.goto('/?useWebSocket=true');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const mediaDisplay = page.locator('[data-testid="media-display"]');

    // Start recording - connection will be closed mid-recording
    await micButton.click();
    await page.waitForTimeout(1000); // Wait for connection to be closed

    // Wait for error state to be displayed
    await page.waitForTimeout(1000);

    // Verify error message is shown
    await expect(mediaDisplay).toContainText(/connection|lost|error|try again/i);
  });

  test('client can retry after server error message', async ({ page }) => {
    let attemptCount = 0;
    let commandProcessed = false;

    // Mock MediaRecorder for WebSocket mode
    await page.addInitScript(() => {
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
                this.ondataavailable({ data: new Blob(['audio chunk'], { type: this.mimeType }) });
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
            this.ondataavailable({ data: new Blob(['final audio'], { type: this.mimeType }) });
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

      navigator.mediaDevices.getUserMedia = () => Promise.resolve(mockStream as unknown as MediaStream);
      (window as any).MediaRecorder = MockMediaRecorder;
    });

    // Mock WebSocket - first stop_recording returns error, subsequent succeed
    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioReceived = false;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'stop_recording' && audioReceived) {
              attemptCount++;
              if (attemptCount === 1) {
                // First stop_recording: send error
                ws.send(JSON.stringify({ type: 'error', message: 'Server temporarily unavailable' }));
              } else {
                // Subsequent stop_recording: work normally
                ws.send(JSON.stringify({ type: 'transcript', text: 'show me a cat' }));
                setTimeout(() => {
                  ws.send(JSON.stringify({
                    type: 'media',
                    subject: 'cat',
                    photo_url: '/fixtures/mock-cat-photo.jpg',
                    audio_url: '/fixtures/mock-cat-audio.mp3',
                  }));
                  commandProcessed = true;
                }, 50);
              }
              audioReceived = false;
            } else if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            } else if (parsed.type === 'start_recording') {
              // Reset for new recording session
              audioReceived = false;
            }
          } catch {
            audioReceived = true;
          }
        } else {
          audioReceived = true;
        }
      });
    });

    // Serve test fixtures
    await page.route('**/fixtures/**', async route => {
      const url = new URL(route.request().url());
      const filePath = path.join(fixturesDir, url.pathname.replace('/fixtures/', ''));
      await route.fulfill({ path: filePath });
    });

    await page.goto('/?useWebSocket=true');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const mediaDisplay = page.locator('[data-testid="media-display"]');
    const img = page.locator('[data-testid="media-image"]');

    // First attempt - should fail with error
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    // Wait for error to be displayed
    await page.waitForTimeout(1000);
    await expect(mediaDisplay).toContainText(/unavailable|something went wrong|error|try again/i);

    // Wait for error state to reset to idle (3 seconds timeout)
    await page.waitForTimeout(3500);

    // Second attempt - should succeed
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    // Verify the command was processed successfully
    await expect(img).toBeVisible({ timeout: 10000 });
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    expect(attemptCount).toBe(2);
    expect(commandProcessed).toBe(true);
  });
});
