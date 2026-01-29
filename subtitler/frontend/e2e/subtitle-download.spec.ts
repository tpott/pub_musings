import { test, expect, type Page } from '@playwright/test';

// Accept cookie consent before tests to prevent the banner from blocking interactions
async function acceptCookies(page: Page): Promise<void> {
  await page.addInitScript(() => {
    localStorage.setItem('subtitler:cookie_consent', 'accepted');
  });
}

/**
 * E2E tests for client-side subtitle viewing/download functionality.
 * Download buttons open a viewer page in a new tab (not a file download).
 * These tests verify that the buttons work and generate correct content.
 */

test.describe('Subtitle viewing - client-side generation', () => {
  // Track network requests to ensure subtitle content doesn't fetch from server
  test('upload page subtitle buttons do not make network requests for subtitle files', async ({ page }) => {
    await acceptCookies(page);

    // Track API calls to subtitle endpoints
    const subtitleRequests: string[] = [];
    page.on('request', (request) => {
      const url = request.url();
      if (url.includes('/subtitles.srt') || url.includes('/subtitles.vtt') || url.includes('/subtitles.json')) {
        subtitleRequests.push(url);
      }
    });

    // Mock the transcription endpoint to return test data
    const videoId = 'test-video-123';
    await page.route('**/api/transcribe/' + videoId, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'complete',
          result: {
            language: 'en',
            duration: 10,
            text: 'Hello world. This is a test.',
            segments: [
              { id: 0, start: 0, end: 2.5, text: 'Hello world.' },
              { id: 1, start: 2.5, end: 5, text: 'This is a test.' },
            ],
          },
        }),
      });
    });

    // Mock video endpoint
    await page.route('**/api/videos/' + videoId + '/video', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'video/mp4',
        body: Buffer.from([0, 0, 0, 0]),
      });
    });

    // Visit upload page with existing video
    await page.goto(`/upload?id=${videoId}`);

    // Wait for transcription to load
    await page.waitForSelector('#segments', { state: 'visible', timeout: 15000 });

    // Find download buttons
    const srtButton = page.locator('#downloadSrt');
    const vttButton = page.locator('#downloadVtt');
    const jsonButton = page.locator('#downloadJson');

    // Verify buttons are visible
    await expect(srtButton).toBeVisible();
    await expect(vttButton).toBeVisible();
    await expect(jsonButton).toBeVisible();

    // Click SRT button - it opens a viewer in a new tab/popup
    const [popup] = await Promise.all([
      page.waitForEvent('popup'),
      srtButton.click(),
    ]);

    // Verify the popup/new tab has the SRT content
    await popup.waitForLoadState();
    const content = await popup.content();
    expect(content).toContain('Hello world.');
    await popup.close();

    // Verify NO network requests were made to subtitle endpoints
    expect(subtitleRequests.length).toBe(0);
  });

  test('videos page modal download buttons use loaded data', async ({ page }) => {
    await acceptCookies(page);

    // Track API calls
    const subtitleRequests: string[] = [];
    page.on('request', (request) => {
      const url = request.url();
      if (url.includes('/subtitles.srt') || url.includes('/subtitles.vtt') || url.includes('/subtitles.json')) {
        subtitleRequests.push(url);
      }
    });

    // Mock auth check (not authenticated)
    await page.route('**/api/auth/me', async (route) => {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Not authenticated' }),
      });
    });

    // Mock videos list
    await page.route('**/api/videos*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          videos: [
            {
              id: 'video-1',
              filename: 'test-video.mp4',
              size: 1000000,
              content_type: 'video/mp4',
              created_at: new Date().toISOString(),
              transcription_status: 'complete',
              thumbnail_path: null,
            },
          ],
          total_count: 1,
          has_more: false,
        }),
      });
    });

    // Mock transcription for the video
    await page.route('**/api/transcribe/video-1', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'complete',
          result: {
            language: 'en',
            duration: 10,
            text: 'Test subtitle content.',
            segments: [
              { id: 0, start: 0, end: 5, text: 'Test subtitle content.' },
            ],
          },
        }),
      });
    });

    // Mock video endpoint
    await page.route('**/api/videos/video-1/video', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'video/mp4',
        body: Buffer.from([0, 0, 0, 0]),
      });
    });

    // Visit videos page
    await page.goto('/videos');

    // Wait for video list to load
    await page.waitForSelector('.video-card', { state: 'visible', timeout: 10000 });

    // Click View button to open modal
    await page.click('.btn-view');

    // Wait for modal to open and load subtitles
    await page.waitForSelector('#videoModal.visible', { state: 'visible', timeout: 10000 });
    await page.waitForSelector('.modal-segment', { state: 'visible', timeout: 10000 });

    // Find modal download buttons
    const modalSrtButton = page.locator('#modalDownloadSrt');

    // Click SRT button - opens viewer in new tab
    const [popup] = await Promise.all([
      page.waitForEvent('popup'),
      modalSrtButton.click(),
    ]);

    await popup.waitForLoadState();
    const content = await popup.content();
    expect(content).toContain('Test subtitle content.');
    await popup.close();

    // Verify NO additional requests to subtitle endpoints were made
    expect(subtitleRequests.length).toBe(0);
  });

  test('inline download buttons on video cards work correctly', async ({ page }) => {
    await acceptCookies(page);

    let transcriptionCalls = 0;
    const subtitleRequests: string[] = [];

    page.on('request', (request) => {
      const url = request.url();
      if (url.includes('/subtitles.srt') || url.includes('/subtitles.vtt') || url.includes('/subtitles.json')) {
        subtitleRequests.push(url);
      }
      if (url.includes('/api/transcribe/video-1') && !url.includes('subtitles')) {
        transcriptionCalls++;
      }
    });

    // Mock auth
    await page.route('**/api/auth/me', async (route) => {
      await route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'Not authenticated' }) });
    });

    // Mock videos list
    await page.route('**/api/videos*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          videos: [
            {
              id: 'video-1',
              filename: 'my-video.mp4',
              size: 500000,
              content_type: 'video/mp4',
              created_at: new Date().toISOString(),
              transcription_status: 'complete',
            },
          ],
          total_count: 1,
          has_more: false,
        }),
      });
    });

    // Mock transcription
    await page.route('**/api/transcribe/video-1', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'complete',
          result: {
            language: 'en',
            duration: 5,
            text: 'Inline test.',
            segments: [{ id: 0, start: 0, end: 3, text: 'Inline test.' }],
          },
        }),
      });
    });

    await page.goto('/videos');
    await page.waitForSelector('.video-card', { state: 'visible', timeout: 10000 });

    // Click the inline SRT button (not in modal)
    const inlineSrtButton = page.locator('.btn-download-srt').first();
    await expect(inlineSrtButton).toBeVisible();

    // Inline buttons trigger a file download (not a popup viewer)
    const [download] = await Promise.all([
      page.waitForEvent('download'),
      inlineSrtButton.click(),
    ]);

    // Verify the download has the correct filename
    expect(download.suggestedFilename()).toMatch(/\.srt$/);

    // Subtitle endpoints should NOT be called
    expect(subtitleRequests.length).toBe(0);
    // Transcription endpoint should be called once (to get the data for client-side generation)
    expect(transcriptionCalls).toBe(1);
  });

  test('SRT viewer shows correct format', async ({ page }) => {
    await acceptCookies(page);
    const videoId = 'format-test';

    await page.route('**/api/transcribe/' + videoId, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'complete',
          result: {
            language: 'en',
            segments: [
              { id: 0, start: 0, end: 2.5, text: 'First line.' },
              { id: 1, start: 3, end: 5.5, text: 'Second line.' },
            ],
          },
        }),
      });
    });

    await page.route('**/api/videos/' + videoId + '/video', async (route) => {
      await route.fulfill({ status: 200, contentType: 'video/mp4', body: Buffer.from([0]) });
    });

    await page.goto(`/upload?id=${videoId}`);
    await page.waitForSelector('#segments', { state: 'visible', timeout: 15000 });

    // Click SRT button to open viewer
    const [popup] = await Promise.all([
      page.waitForEvent('popup'),
      page.click('#downloadSrt'),
    ]);

    await popup.waitForLoadState();
    const content = await popup.content();

    // Verify SRT format in the viewer page (HTML-encoded arrows)
    expect(content).toContain('First line.');
    expect(content).toContain('Second line.');
    // Check time format exists (may be HTML-encoded)
    expect(content).toContain('00:00:00,000');
    expect(content).toContain('00:00:02,500');
    await popup.close();
  });

  test('VTT viewer shows correct format', async ({ page }) => {
    await acceptCookies(page);
    const videoId = 'vtt-format-test';

    await page.route('**/api/transcribe/' + videoId, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'complete',
          result: {
            language: 'en',
            segments: [{ id: 0, start: 1.5, end: 4, text: 'VTT test.' }],
          },
        }),
      });
    });

    await page.route('**/api/videos/' + videoId + '/video', async (route) => {
      await route.fulfill({ status: 200, contentType: 'video/mp4', body: Buffer.from([0]) });
    });

    await page.goto(`/upload?id=${videoId}`);
    await page.waitForSelector('#segments', { state: 'visible', timeout: 15000 });

    const [popup] = await Promise.all([
      page.waitForEvent('popup'),
      page.click('#downloadVtt'),
    ]);

    await popup.waitForLoadState();
    const content = await popup.content();

    // Verify VTT format in viewer
    expect(content).toContain('WEBVTT');
    expect(content).toContain('VTT test.');
    await popup.close();
  });

  test('JSON viewer shows correct format', async ({ page }) => {
    await acceptCookies(page);
    const videoId = 'json-format-test';

    await page.route('**/api/transcribe/' + videoId, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'complete',
          result: {
            language: 'en',
            segments: [{ id: 0, start: 0, end: 2, text: 'JSON test.' }],
          },
        }),
      });
    });

    await page.route('**/api/videos/' + videoId + '/video', async (route) => {
      await route.fulfill({ status: 200, contentType: 'video/mp4', body: Buffer.from([0]) });
    });

    await page.goto(`/upload?id=${videoId}`);
    await page.waitForSelector('#segments', { state: 'visible', timeout: 15000 });

    const [popup] = await Promise.all([
      page.waitForEvent('popup'),
      page.click('#downloadJson'),
    ]);

    await popup.waitForLoadState();
    const content = await popup.content();

    // Verify JSON content in viewer
    expect(content).toContain('JSON test.');
    await popup.close();
  });
});
