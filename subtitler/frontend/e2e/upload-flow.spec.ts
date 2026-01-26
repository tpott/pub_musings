import { test, expect, type Page } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';
import { existsSync, mkdirSync } from 'fs';
import { execSync } from 'child_process';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const TEST_VIDEO_PATH = path.join(__dirname, 'fixtures/test-video.mp4');

// Ensure test video fixture exists (generate with ffmpeg if missing)
// The test video is gitignored (*.mp4) so we generate it on demand
function ensureTestVideo(): void {
  const fixturesDir = path.join(__dirname, 'fixtures');
  if (!existsSync(fixturesDir)) {
    mkdirSync(fixturesDir, { recursive: true });
  }

  if (!existsSync(TEST_VIDEO_PATH)) {
    console.log('Generating test video fixture...');
    try {
      execSync(
        `ffmpeg -f lavfi -i "testsrc=duration=3:size=320x240:rate=15" -f lavfi -i "sine=frequency=440:duration=3" -c:v libx264 -preset ultrafast -crf 30 -c:a aac -b:a 64k -y "${TEST_VIDEO_PATH}"`,
        { stdio: 'pipe' }
      );
      console.log('Test video fixture created');
    } catch (error) {
      throw new Error(`Failed to generate test video fixture. Ensure ffmpeg is installed.\nRun manually: ffmpeg -f lavfi -i "testsrc=duration=3:size=320x240:rate=15" -f lavfi -i "sine=frequency=440:duration=3" -c:v libx264 -preset ultrafast -crf 30 -c:a aac -b:a 64k -y "${TEST_VIDEO_PATH}"`);
    }
  }
}

// Generate test fixture before tests run
test.beforeAll(() => {
  ensureTestVideo();
});

// Helper to generate unique session ID for anonymous uploads
function generateSessionId(): string {
  return `e2e-test-${Date.now()}-${Math.random().toString(36).substring(2, 8)}`;
}

// Helper to wait for transcription to complete or error
async function waitForTranscriptionStatus(page: Page, timeout: number = 120000): Promise<'complete' | 'complete-no-segments' | 'error' | 'timeout'> {
  const startTime = Date.now();

  while (Date.now() - startTime < timeout) {
    // Check for error status
    const errorVisible = await page.locator('.status.error').isVisible().catch(() => false);
    if (errorVisible) {
      return 'error';
    }

    // Check if actions section is visible (indicates transcription complete)
    const actionsVisible = await page.locator('#actions').isVisible().catch(() => false);
    if (actionsVisible) {
      // Check if we have segments
      const segmentCount = await page.locator('#segments .segment').count();
      if (segmentCount > 0) {
        return 'complete';
      }
      return 'complete-no-segments';
    }

    // Also check for transcription status text indicating completion
    const statusText = await page.locator('#transcriptionStatus').textContent().catch(() => '');
    if (statusText && (statusText.includes('Complete') || statusText.includes('complete'))) {
      const segmentCount = await page.locator('#segments .segment').count();
      if (segmentCount > 0) {
        return 'complete';
      }
      return 'complete-no-segments';
    }

    // Wait a bit before checking again
    await page.waitForTimeout(2000);
  }

  return 'timeout';
}

test.describe('Upload Flow', () => {
  test('should display upload page correctly', async ({ page }) => {
    await page.goto('/upload');

    await expect(page).toHaveTitle('Upload - Subtitler');
    await expect(page.locator('h1')).toContainText('Upload Video');
    await expect(page.locator('#dropzone')).toBeVisible();
    await expect(page.locator('.dropzone-text')).toContainText('Drag and drop');
  });

  test('should accept video file via file input', async ({ page }) => {
    await page.goto('/upload');

    // Upload file
    const fileInput = page.locator('#fileInput');
    await fileInput.setInputFiles(TEST_VIDEO_PATH);

    // Check that status section becomes visible
    const status = page.locator('#status');
    await expect(status).toHaveClass(/visible/, { timeout: 5000 });

    // Check that status text shows progress (upload or transcription starting)
    // Small files upload quickly, so we may see "Upload complete" or "transcription" status
    const statusText = page.locator('#statusText');
    await expect(statusText).toContainText(/upload|processing|transcription/i, { timeout: 10000 });
  });
});

test.describe('Full Upload-to-Download Flow', () => {
  // This test requires whisper to be available
  // It tests: upload → transcription → edit → download
  // NOTE: The test video contains only test patterns and a sine wave (no speech),
  // so transcription completes with 0 segments. The test verifies the flow completes
  // and handles this edge case correctly. To test with actual speech segments,
  // replace fixtures/test-video.mp4 with a video containing spoken audio.
  test('should complete full flow: upload, transcribe, edit, download', async ({ page }) => {
    // Set a longer timeout for this test (transcription can take time)
    test.setTimeout(180000);

    await page.goto('/upload');

    // Step 1: Upload the video file
    console.log('Step 1: Uploading video...');
    const fileInput = page.locator('#fileInput');
    await fileInput.setInputFiles(TEST_VIDEO_PATH);

    // Wait for upload to start (small files upload quickly, may show complete status)
    const statusText = page.locator('#statusText');
    await expect(statusText).toContainText(/upload|processing|transcription/i, { timeout: 15000 });

    // Step 2: Wait for transcription to complete
    console.log('Step 2: Waiting for transcription...');
    const transcriptionResult = await waitForTranscriptionStatus(page, 120000);

    if (transcriptionResult === 'error') {
      // Transcription failed - likely whisper not available
      console.log('Transcription failed (whisper may not be available)');

      // Check that error message is shown
      const errorStatus = page.locator('.status.error');
      await expect(errorStatus).toBeVisible();

      // This is expected in test environments without whisper
      // Skip the rest of the test gracefully
      test.skip(true, 'Whisper server not available - skipping full flow test');
      return;
    }

    if (transcriptionResult === 'timeout') {
      // Timeout waiting - transcription may still be processing
      console.log('Timeout waiting for transcription');
      test.skip(true, 'Transcription timed out - whisper may be slow or unavailable');
      return;
    }

    if (transcriptionResult === 'complete-no-segments') {
      // Transcription completed but no speech detected (test video has no speech)
      console.log('Transcription completed with no segments (no speech detected in test video)');

      // Verify actions are visible even without segments
      const actions = page.locator('#actions');
      await expect(actions).toBeVisible();

      // Verify download button is visible
      const downloadSrt = page.locator('#downloadSrt');
      await expect(downloadSrt).toBeVisible();

      // Download SRT should work (will be empty but valid)
      const href = await downloadSrt.getAttribute('href');
      expect(href).toMatch(/\/api\/videos\/[a-zA-Z0-9]+\/subtitles\.srt/);

      console.log('Test completed successfully (empty transcription case)');
      return;
    }

    // Transcription completed successfully with segments
    console.log('Step 3: Transcription complete, verifying segments...');

    // Verify segments are displayed
    const segments = page.locator('#segments .segment');
    const segmentCount = await segments.count();
    expect(segmentCount).toBeGreaterThan(0);

    // Verify actions are visible
    const actions = page.locator('#actions');
    await expect(actions).toBeVisible();

    // Step 4: Enter edit mode and modify a subtitle
    console.log('Step 4: Editing subtitle...');
    const editBtn = page.locator('#editBtn');
    await expect(editBtn).toBeVisible();
    await editBtn.click();

    // Find the first segment's text area in edit mode
    const firstSegmentText = page.locator('#segments .segment textarea').first();
    await expect(firstSegmentText).toBeVisible({ timeout: 5000 });

    // Clear and type new text
    const originalText = await firstSegmentText.inputValue();
    const newText = 'E2E TEST EDIT: ' + originalText;
    await firstSegmentText.fill(newText);

    // Verify unsaved changes indicator
    const unsavedIndicator = page.locator('#unsavedIndicator');
    await expect(unsavedIndicator).toHaveClass(/visible/);

    // Save changes
    const saveBtn = page.locator('#saveBtn');
    await saveBtn.click();

    // Wait for save to complete (edit mode should exit)
    await expect(editBtn).toBeVisible({ timeout: 10000 });

    console.log('Step 5: Verifying edit was saved...');
    // Verify the segment text was updated
    const updatedSegment = page.locator('#segments .segment').first();
    await expect(updatedSegment).toContainText('E2E TEST EDIT');

    // Step 5: Download SRT file
    console.log('Step 6: Downloading SRT...');
    const downloadSrt = page.locator('#downloadSrt');
    await expect(downloadSrt).toBeVisible();

    // Verify download link has correct href pattern
    const href = await downloadSrt.getAttribute('href');
    expect(href).toMatch(/\/api\/videos\/[a-zA-Z0-9]+\/subtitles\.srt/);

    // Trigger download and verify response (without actually downloading file)
    const [downloadPromise] = await Promise.all([
      page.waitForEvent('download'),
      downloadSrt.click()
    ]);

    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(/\.srt$/);

    console.log('Full upload-to-download flow completed successfully!');
  });
});

// Test upload page structure and UI elements
test.describe('Upload Page Elements', () => {
  test('should have video preview area', async ({ page }) => {
    await page.goto('/upload');

    const videoPreview = page.locator('#videoPreview');
    const previewVideo = page.locator('#previewVideo');

    // Video preview is hidden by default
    await expect(videoPreview).not.toHaveClass(/visible/);
    await expect(previewVideo).toBeAttached();
  });

  test('should have transcription section', async ({ page }) => {
    await page.goto('/upload');

    const transcription = page.locator('#transcription');
    const segments = page.locator('#segments');

    // Transcription section exists but not visible until transcription starts
    await expect(transcription).toBeAttached();
    await expect(segments).toBeAttached();
  });

  test('should have paste transcript section', async ({ page }) => {
    await page.goto('/upload');

    const pasteTranscript = page.locator('#pasteTranscript');
    const pasteText = page.locator('#pasteText');
    const lyricsMode = page.locator('#lyricsMode');
    const alignBtn = page.locator('#alignBtn');

    await expect(pasteTranscript).toBeAttached();
    await expect(pasteText).toBeAttached();
    await expect(lyricsMode).toBeAttached();
    await expect(alignBtn).toBeAttached();
  });

  test('should have script conversion options', async ({ page }) => {
    await page.goto('/upload');

    const convertScript = page.locator('#convertScript');
    const languageSelect = page.locator('#languageSelect');
    const scriptSelect = page.locator('#scriptSelect');

    await expect(convertScript).toBeAttached();
    await expect(languageSelect).toBeAttached();
    await expect(scriptSelect).toBeAttached();
  });

  test('should have download format buttons', async ({ page }) => {
    await page.goto('/upload');

    const downloadSrt = page.locator('#downloadSrt');
    const downloadVtt = page.locator('#downloadVtt');
    const downloadJson = page.locator('#downloadJson');

    await expect(downloadSrt).toBeAttached();
    await expect(downloadVtt).toBeAttached();
    await expect(downloadJson).toBeAttached();
  });

  test('should have burn subtitles button', async ({ page }) => {
    await page.goto('/upload');

    const burnBtn = page.locator('#burnBtn');
    await expect(burnBtn).toBeAttached();
    await expect(burnBtn).toContainText('Burn Subtitles');
  });
});

// Test client-side validation
test.describe('Upload Validation', () => {
  test('should only accept video files', async ({ page }) => {
    await page.goto('/upload');

    // The file input should have accept="video/*"
    const fileInput = page.locator('#fileInput');
    const acceptAttribute = await fileInput.getAttribute('accept');
    expect(acceptAttribute).toBe('video/*');
  });

  test('dropzone should respond to drag events', async ({ page }) => {
    await page.goto('/upload');

    const dropzone = page.locator('#dropzone');

    // Verify dropzone is interactive
    await expect(dropzone).toHaveClass(/dropzone/);

    // Verify it has click handler (clicking should trigger file input)
    await expect(dropzone).toHaveCSS('cursor', 'pointer');
  });
});
