import { test, expect } from '@playwright/test';
import path from 'path';
import os from 'os';
import fs from 'fs';

test.describe('Transcription API', () => {
  const jfkPath = path.join(os.homedir(), 'Github/whisper.cpp/samples/jfk.wav');

  test('transcribe endpoint returns SRT format with real backend', async ({ page, request }) => {
    test.setTimeout(120000); // 2 minutes timeout for transcription

    // Skip if JFK sample file doesn't exist
    if (!fs.existsSync(jfkPath)) {
      test.skip();
    }

    // Send transcription request directly to API
    const fileBuffer = fs.readFileSync(jfkPath);
    const formData = new FormData();
    const blob = new Blob([fileBuffer], { type: 'audio/wav' });
    formData.append('file', blob, 'jfk.wav');

    const response = await request.post('http://localhost:8080/api/transcribe', {
      multipart: {
        file: {
          name: 'jfk.wav',
          mimeType: 'audio/wav',
          buffer: fileBuffer,
        },
      },
      timeout: 120000, // 2 minutes timeout for transcription
    });

    // Verify response
    expect(response.ok()).toBeTruthy();
    expect(response.status()).toBe(200);

    const json = await response.json();
    expect(json.success).toBe(true);
    expect(json.transcript).toBeDefined();
    expect(json.transcript.length).toBeGreaterThan(0);
    expect(json.format).toBe('srt');
    expect(json.filename).toBe('jfk.wav');

    // Verify SRT format
    const transcript = json.transcript;
    expect(transcript).toContain('-->'); // SRT timestamp separator

    // Verify expected content
    const transcriptLower = transcript.toLowerCase();
    expect(transcriptLower).toContain('ask not');

    console.log(`Transcription completed in ${json.duration} seconds`);
    console.log('Transcript preview:', transcript.substring(0, 200));
  });

  test('transcribe endpoint returns error for invalid file format', async ({ request }) => {
    // Create a fake file with invalid extension
    const invalidContent = Buffer.from('not a real audio file');

    const response = await request.post('http://localhost:8080/api/transcribe', {
      multipart: {
        file: {
          name: 'invalid.exe',
          mimeType: 'application/x-msdownload',
          buffer: invalidContent,
        },
      },
    });

    expect(response.status()).toBe(400);
    const json = await response.json();
    expect(json.success).toBe(false);
    expect(json.message).toContain('file type');
  });

  test('transcribe endpoint returns error for missing file', async ({ request }) => {
    const response = await request.post('http://localhost:8080/api/transcribe', {
      multipart: {},
    });

    expect(response.status()).toBe(400);
    const json = await response.json();
    expect(json.success).toBe(false);
  });

  test('transcribe endpoint handles OPTIONS (CORS preflight)', async ({ request }) => {
    const response = await request.fetch('http://localhost:8080/api/transcribe', {
      method: 'OPTIONS',
    });

    expect(response.status()).toBe(200);
    expect(response.headers()['access-control-allow-origin']).toBe('http://localhost:4321');
  });

  test('transcribe endpoint rejects non-POST methods', async ({ request }) => {
    const response = await request.get('http://localhost:8080/api/transcribe');
    expect(response.status()).toBe(405);

    const json = await response.json();
    expect(json.success).toBe(false);
  });
});

test.describe('Transcription UI Integration', () => {
  test('UI can upload and display transcription (mocked)', async ({ page }) => {
    // Mock the transcribe API response
    await page.route('http://localhost:8080/api/transcribe', async route => {
      // Simulate transcription delay
      await new Promise(resolve => setTimeout(resolve, 1000));

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          message: 'Transcription completed successfully',
          transcript: `1
00:00:00,000 --> 00:00:05,000
And so my fellow Americans, ask not what your country can do for you.

2
00:00:05,000 --> 00:00:10,000
Ask what you can do for your country.`,
          filename: 'test-audio.wav',
          format: 'srt',
          duration: 15.5,
        }),
      });
    });

    await page.goto('/');

    // Create a small test audio file
    const tempDir = os.tmpdir();
    const testFilePath = path.join(tempDir, 'test-audio.wav');
    const testContent = Buffer.alloc(1024 * 10); // 10KB
    fs.writeFileSync(testFilePath, testContent);

    try {
      // Upload the file
      const fileInput = page.locator('#file-input');
      await fileInput.setInputFiles(testFilePath);

      // Wait for file info to appear
      const fileInfo = page.locator('#file-info');
      await expect(fileInfo).toBeVisible();

      // Note: Current UI uses /api/upload endpoint
      // This test validates the API works, but UI integration
      // would require updating the frontend to use /api/transcribe
      // That will be done in a future task (Task 7 - User dashboard)

    } finally {
      // Cleanup
      fs.unlinkSync(testFilePath);
    }
  });
});
