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

  test('displays file list after selecting files', async ({ page }) => {
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

      // Wait for file list to appear (batch upload UI)
      const fileList = page.locator('#file-list');
      await expect(fileList).toBeVisible();

      // Check file name is displayed in the file list
      const fileItem = page.locator('.file-item');
      await expect(fileItem).toBeVisible();
      await expect(fileItem.locator('.file-item-name')).toContainText('test-audio.mp3');

      // Check file size is displayed
      const fileSize = fileItem.locator('.file-item-size');
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

    // Wait for file list to appear
    const fileList = page.locator('#file-list');
    await expect(fileList).toBeVisible();

    // File should show error styling
    const fileItem = page.locator('.file-item-error');
    await expect(fileItem).toBeVisible();
    await expect(fileItem.locator('.file-item-error-msg')).toContainText('200MB');

    // Submit button should be disabled since no valid files
    const submitBtn = page.locator('#submit-btn');
    await expect(submitBtn).toBeDisabled();
  });

  test('uploads file successfully with mock backend', async ({ page }) => {
    // Intercept the upload request (relative path via proxy)
    await page.route('**/api/upload', async route => {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          message: 'Files uploaded successfully',
          jobs: [
            { job_id: 1, filename: 'test-audio.mp3' }
          ],
          failed_files: []
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
    await page.route('**/api/upload', async route => {
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
    await page.route('**/api/upload', async route => {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          message: 'Files uploaded successfully',
          jobs: [
            { job_id: 1, filename: 'test.mp3' }
          ],
          failed_files: []
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

      // Verify form is reset - file list should be hidden
      const fileList = page.locator('#file-list');
      await expect(fileList).not.toBeVisible();

      // Submit button should be hidden again (no files selected)
      await expect(submitBtn).not.toBeVisible();
    } finally {
      // Cleanup
      fs.unlinkSync(testFilePath);
    }
  });

  test('shows network error when backend is unreachable', async ({ page }) => {
    // Intercept and abort the request to simulate network error
    await page.route('**/api/upload', route => route.abort('failed'));

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

  test('can remove files from selection before upload', async ({ page }) => {
    await page.goto('/');

    // Create test files
    const tempDir = os.tmpdir();
    const testFilePath1 = path.join(tempDir, 'test1.mp3');
    const testFilePath2 = path.join(tempDir, 'test2.mp3');
    fs.writeFileSync(testFilePath1, Buffer.alloc(1024));
    fs.writeFileSync(testFilePath2, Buffer.alloc(1024));

    try {
      // Upload multiple files
      const fileInput = page.locator('#file-input');
      await fileInput.setInputFiles([testFilePath1, testFilePath2]);

      // Verify both files are shown
      const fileItems = page.locator('.file-item');
      await expect(fileItems).toHaveCount(2);

      // Click the remove button on the first file
      const removeBtn = fileItems.first().locator('.file-item-remove');
      await removeBtn.click();

      // Verify only one file remains
      await expect(fileItems).toHaveCount(1);
      await expect(fileItems.first().locator('.file-item-name')).toContainText('test2.mp3');
    } finally {
      // Cleanup
      fs.unlinkSync(testFilePath1);
      fs.unlinkSync(testFilePath2);
    }
  });

  test('language selection is shown when files are selected', async ({ page }) => {
    await page.goto('/');

    // Create a test file
    const tempDir = os.tmpdir();
    const testFilePath = path.join(tempDir, 'test.mp3');
    fs.writeFileSync(testFilePath, Buffer.alloc(1024));

    try {
      // Initially, language selection should be hidden
      const optionsSection = page.locator('#options-section');
      await expect(optionsSection).not.toBeVisible();

      // Upload the file
      const fileInput = page.locator('#file-input');
      await fileInput.setInputFiles(testFilePath);

      // Language selection should now be visible
      await expect(optionsSection).toBeVisible();
      const languageSelect = page.locator('#language-select');
      await expect(languageSelect).toBeVisible();

      // Select a language
      await languageSelect.selectOption('es');
      await expect(languageSelect).toHaveValue('es');
    } finally {
      // Cleanup
      fs.unlinkSync(testFilePath);
    }
  });
});
