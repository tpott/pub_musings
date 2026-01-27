import { test, expect } from '@playwright/test';

/**
 * E2E tests for client-side subtitle download functionality.
 * These tests verify that download buttons generate files client-side
 * without making server requests for the subtitle data.
 */

test.describe('Subtitle downloads - client-side generation', () => {
  // Track network requests to ensure downloads don't fetch from server
  test('upload page download buttons do not make network requests for subtitle files', async ({ page }) => {
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
        body: Buffer.from([0, 0, 0, 0]), // Minimal content
      });
    });

    // Visit upload page with existing video
    await page.goto(`/upload?id=${videoId}`);

    // Wait for transcription to load
    await page.waitForSelector('#segments', { state: 'visible' });

    // Find download buttons
    const srtButton = page.locator('#downloadSrt');
    const vttButton = page.locator('#downloadVtt');
    const jsonButton = page.locator('#downloadJson');

    // Verify buttons are visible
    await expect(srtButton).toBeVisible();
    await expect(vttButton).toBeVisible();
    await expect(jsonButton).toBeVisible();

    // Set up download listener - we expect downloads to be created via Blob URLs
    const downloadPromise = page.waitForEvent('download');

    // Click SRT download
    await srtButton.click();

    // Wait for download to start
    const download = await downloadPromise;

    // Verify download was triggered
    expect(download.suggestedFilename()).toContain('.srt');

    // Verify NO network requests were made to subtitle endpoints
    expect(subtitleRequests.length).toBe(0);
  });

  test('videos page modal download buttons use loaded data', async ({ page }) => {
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
    await page.waitForSelector('.video-card', { state: 'visible' });

    // Click View button to open modal
    await page.click('.btn-view');

    // Wait for modal to open and load subtitles
    await page.waitForSelector('#videoModal.visible', { state: 'visible' });
    await page.waitForSelector('.modal-segment', { state: 'visible' });

    // Find modal download buttons
    const modalSrtButton = page.locator('#modalDownloadSrt');

    // Set up download listener
    const downloadPromise = page.waitForEvent('download');

    // Click SRT download in modal
    await modalSrtButton.click();

    // Wait for download
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toContain('.srt');

    // Verify NO additional requests to subtitle endpoints were made
    // (The transcription endpoint was called once to populate the modal - that's expected)
    expect(subtitleRequests.length).toBe(0);
  });

  test('inline download buttons on video cards work correctly', async ({ page }) => {
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
    await page.waitForSelector('.video-card', { state: 'visible' });

    // Click the inline SRT button (not in modal)
    const inlineSrtButton = page.locator('.btn-download-srt').first();
    await expect(inlineSrtButton).toBeVisible();

    // Set up download listener
    const downloadPromise = page.waitForEvent('download');

    await inlineSrtButton.click();

    // Wait for download
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toContain('.srt');

    // Transcription endpoint should be called once (to get the data for generation)
    // but subtitle endpoints should NOT be called
    expect(subtitleRequests.length).toBe(0);
    expect(transcriptionCalls).toBe(1); // Data fetched once for client-side generation
  });

  test('download generates correct SRT format', async ({ page }) => {
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
    await page.waitForSelector('#segments', { state: 'visible' });

    // Set up download with content capture
    const [download] = await Promise.all([
      page.waitForEvent('download'),
      page.click('#downloadSrt'),
    ]);

    // Read the downloaded content
    const stream = await download.createReadStream();
    const chunks: Buffer[] = [];
    for await (const chunk of stream!) {
      chunks.push(Buffer.from(chunk));
    }
    const content = Buffer.concat(chunks).toString('utf-8');

    // Verify SRT format
    expect(content).toContain('1\n');
    expect(content).toContain('00:00:00,000 --> 00:00:02,500');
    expect(content).toContain('First line.');
    expect(content).toContain('2\n');
    expect(content).toContain('00:00:03,000 --> 00:00:05,500');
    expect(content).toContain('Second line.');
  });

  test('download generates correct VTT format', async ({ page }) => {
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
    await page.waitForSelector('#segments', { state: 'visible' });

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      page.click('#downloadVtt'),
    ]);

    const stream = await download.createReadStream();
    const chunks: Buffer[] = [];
    for await (const chunk of stream!) {
      chunks.push(Buffer.from(chunk));
    }
    const content = Buffer.concat(chunks).toString('utf-8');

    // Verify VTT format
    expect(content).toContain('WEBVTT');
    expect(content).toContain('00:00:01.500 --> 00:00:04.000'); // VTT uses periods
    expect(content).toContain('VTT test.');
  });

  test('download generates correct JSON format', async ({ page }) => {
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
    await page.waitForSelector('#segments', { state: 'visible' });

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      page.click('#downloadJson'),
    ]);

    const stream = await download.createReadStream();
    const chunks: Buffer[] = [];
    for await (const chunk of stream!) {
      chunks.push(Buffer.from(chunk));
    }
    const content = Buffer.concat(chunks).toString('utf-8');

    // Verify JSON format
    const parsed = JSON.parse(content);
    expect(parsed.segments).toBeDefined();
    expect(parsed.segments[0].id).toBe(0);
    expect(parsed.segments[0].start).toBe(0);
    expect(parsed.segments[0].end).toBe(2);
    expect(parsed.segments[0].text).toBe('JSON test.');
  });
});
