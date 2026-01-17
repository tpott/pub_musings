import { test, expect } from '@playwright/test';
import path from 'path';
import fs from 'fs';
import os from 'os';

test.describe('File Upload Form', () => {
  test('renders upload form correctly', async ({ page }) => {
    await page.goto('/');

    // Check page title
    await expect(page).toHaveTitle(/Subtitler/);

    // Check main heading
    await expect(page.locator('h1')).toHaveText('Subtitler');

    // Check upload area exists
    const uploadArea = page.locator('#upload-area');
    await expect(uploadArea).toBeVisible();

    // Check file input exists
    const fileInput = page.locator('#file-input');
    await expect(fileInput).toBeAttached();

    // Submit button should not be visible initially
    const submitBtn = page.locator('#submit-btn');
    await expect(submitBtn).not.toBeVisible();
  });

  test('displays file info after selecting a file', async ({ page }) => {
    await page.goto('/');

    // Create a small test audio file
    const tempDir = os.tmpdir();
    const testFilePath = path.join(tempDir, 'test-audio.mp3');
    const testContent = Buffer.alloc(1024 * 10); // 10KB
    fs.writeFileSync(testFilePath, testContent);

    try {
      // Upload the file
      const fileInput = page.locator('#file-input');
      await fileInput.setInputFiles(testFilePath);

      // Wait for file info to appear
      const fileInfo = page.locator('#file-info');
      await expect(fileInfo).toBeVisible();

      // Check file name is displayed
      const fileName = page.locator('#file-name');
      await expect(fileName).toContainText('test-audio.mp3');

      // Check file size is displayed
      const fileSize = page.locator('#file-size');
      await expect(fileSize).toBeVisible();

      // Submit button should now be visible
      const submitBtn = page.locator('#submit-btn');
      await expect(submitBtn).toBeVisible();
    } finally {
      // Cleanup
      fs.unlinkSync(testFilePath);
    }
  });

  test('shows error for oversized file', async ({ page }) => {
    await page.goto('/');

    // Create a mock large file by setting the File object directly
    await page.evaluate(() => {
      const fileInput = document.getElementById('file-input') as HTMLInputElement;
      const largeMockFile = new File([''], 'large.mp4', { type: 'video/mp4' });
      Object.defineProperty(largeMockFile, 'size', {
        value: 250 * 1024 * 1024, // 250MB
        writable: false
      });

      const dataTransfer = new DataTransfer();
      dataTransfer.items.add(largeMockFile);
      fileInput.files = dataTransfer.files;
      fileInput.dispatchEvent(new Event('change', { bubbles: true }));
    });

    // Wait for error message
    const message = page.locator('#message');
    await expect(message).toBeVisible();
    await expect(message).toContainText('200MB');

    // Submit button should be disabled
    const submitBtn = page.locator('#submit-btn');
    await expect(submitBtn).toBeDisabled();
  });

  test('uploads file successfully with mock backend', async ({ page, context }) => {
    // Intercept the upload request
    await page.route('http://localhost:8080/api/upload', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          message: 'File uploaded successfully',
          filename: 'test-audio.mp3',
          size: 10240,
          type: 'audio/mpeg'
        })
      });
    });

    await page.goto('/');

    // Create a small test audio file
    const tempDir = os.tmpdir();
    const testFilePath = path.join(tempDir, 'test-audio.mp3');
    const testContent = Buffer.alloc(1024 * 10); // 10KB
    fs.writeFileSync(testFilePath, testContent);

    try {
      // Upload the file
      const fileInput = page.locator('#file-input');
      await fileInput.setInputFiles(testFilePath);

      // Submit the form
      const submitBtn = page.locator('#submit-btn');
      await submitBtn.click();

      // Wait for success message
      const message = page.locator('#message');
      await expect(message).toBeVisible();
      await expect(message).toContainText('uploaded successfully');

      // Reset button should be visible
      const resetBtn = page.locator('#reset-btn');
      await expect(resetBtn).toBeVisible();
    } finally {
      // Cleanup
      fs.unlinkSync(testFilePath);
    }
  });

  test('shows error on failed upload', async ({ page }) => {
    // Intercept the upload request with error response
    await page.route('http://localhost:8080/api/upload', async route => {
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          message: 'Invalid file type'
        })
      });
    });

    await page.goto('/');

    // Create a test file
    const tempDir = os.tmpdir();
    const testFilePath = path.join(tempDir, 'test.mp3');
    fs.writeFileSync(testFilePath, Buffer.alloc(1024));

    try {
      // Upload the file
      const fileInput = page.locator('#file-input');
      await fileInput.setInputFiles(testFilePath);

      // Submit the form
      const submitBtn = page.locator('#submit-btn');
      await submitBtn.click();

      // Wait for error message
      const message = page.locator('#message');
      await expect(message).toBeVisible();
      await expect(message).toContainText('Invalid file type');
    } finally {
      // Cleanup
      fs.unlinkSync(testFilePath);
    }
  });

  test('reset button works correctly', async ({ page }) => {
    // Intercept the upload request
    await page.route('http://localhost:8080/api/upload', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          message: 'File uploaded successfully',
          filename: 'test.mp3',
          size: 1024,
          type: 'audio/mpeg'
        })
      });
    });

    await page.goto('/');

    // Create and upload a test file
    const tempDir = os.tmpdir();
    const testFilePath = path.join(tempDir, 'test.mp3');
    fs.writeFileSync(testFilePath, Buffer.alloc(1024));

    try {
      const fileInput = page.locator('#file-input');
      await fileInput.setInputFiles(testFilePath);

      const submitBtn = page.locator('#submit-btn');
      await submitBtn.click();

      // Wait for reset button to appear
      const resetBtn = page.locator('#reset-btn');
      await expect(resetBtn).toBeVisible();

      // Click reset button
      await resetBtn.click();

      // Verify form is reset
      const fileInfo = page.locator('#file-info');
      await expect(fileInfo).not.toBeVisible();

      await expect(resetBtn).not.toBeVisible();
      await expect(submitBtn).toBeVisible();
    } finally {
      // Cleanup
      fs.unlinkSync(testFilePath);
    }
  });

  test('shows network error when backend is unreachable', async ({ page }) => {
    // Intercept and abort the request to simulate network error
    await page.route('http://localhost:8080/api/upload', route => route.abort('failed'));

    await page.goto('/');

    // Create a test file
    const tempDir = os.tmpdir();
    const testFilePath = path.join(tempDir, 'test.mp3');
    fs.writeFileSync(testFilePath, Buffer.alloc(1024));

    try {
      // Upload the file
      const fileInput = page.locator('#file-input');
      await fileInput.setInputFiles(testFilePath);

      // Submit the form
      const submitBtn = page.locator('#submit-btn');
      await submitBtn.click();

      // Wait for network error message
      const message = page.locator('#message');
      await expect(message).toBeVisible();
      await expect(message).toContainText('Network error');
    } finally {
      // Cleanup
      fs.unlinkSync(testFilePath);
    }
  });
});
