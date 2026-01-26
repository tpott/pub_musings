import { test, expect } from '@playwright/test';

test.describe('Dashboard', () => {
  test('redirects to home when not authenticated', async ({ page }) => {
    // Mock the /api/me endpoint to return 401
    await page.route('**/api/me', async route => {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          message: 'Unauthorized'
        })
      });
    });

    // Attempt to navigate to dashboard
    await page.goto('/dashboard');

    // Should redirect to home page
    await page.waitForURL('/');
    await expect(page).toHaveURL('/');
  });

  test('displays user email and logout button when authenticated', async ({ page }) => {
    // Mock the /api/me endpoint
    await page.route('**/api/me', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          user: {
            id: 1,
            email: 'test@example.com'
          }
        })
      });
    });

    // Mock the /api/jobs endpoint with empty list
    await page.route('**/api/jobs', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          jobs: []
        })
      });
    });

    await page.goto('/dashboard');

    // Check that user email is displayed
    const userEmail = page.locator('#user-email');
    await expect(userEmail).toContainText('test@example.com');

    // Check that logout button is present (use the one in user-info section to avoid duplicate ID issue)
    const logoutBtn = page.locator('.user-info #logout-btn');
    await expect(logoutBtn).toBeVisible();
  });

  test('displays empty state when user has no jobs', async ({ page }) => {
    // Mock authentication
    await page.route('**/api/me', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          user: { id: 1, email: 'test@example.com' }
        })
      });
    });

    // Mock empty jobs list
    await page.route('**/api/jobs', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          jobs: []
        })
      });
    });

    await page.goto('/dashboard');

    // Check empty state message
    const emptyState = page.locator('.empty-state');
    await expect(emptyState).toBeVisible();
    await expect(emptyState).toContainText('No transcription jobs yet');

    // Check upload link is present
    const uploadLink = page.locator('.upload-link');
    await expect(uploadLink).toBeVisible();
    await expect(uploadLink).toHaveAttribute('href', '/');
  });

  test('displays list of jobs with correct information', async ({ page }) => {
    // Mock authentication
    await page.route('**/api/me', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          user: { id: 1, email: 'test@example.com' }
        })
      });
    });

    // Mock jobs list with sample data
    await page.route('**/api/jobs', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          jobs: [
            {
              id: 1,
              user_id: 1,
              status: 'completed',
              original_filename: 'test-video.mp4',
              file_path: '/data/files/uploads/1/1/test-video.mp4',
              file_size: 5242880, // 5MB
              output_format: 'srt',
              transcript_path: '/data/files/results/1/1/test-video.srt',
              created_at: '2026-01-18T10:00:00Z',
              updated_at: '2026-01-18T10:05:00Z',
              completed_at: '2026-01-18T10:05:00Z'
            },
            {
              id: 2,
              user_id: 1,
              status: 'pending',
              original_filename: 'audio-clip.mp3',
              file_path: '/data/files/uploads/1/2/audio-clip.mp3',
              file_size: 1048576, // 1MB
              output_format: 'srt',
              created_at: '2026-01-18T11:00:00Z',
              updated_at: '2026-01-18T11:00:00Z'
            },
            {
              id: 3,
              user_id: 1,
              status: 'failed',
              original_filename: 'broken.mp4',
              file_path: '/data/files/uploads/1/3/broken.mp4',
              file_size: 2097152, // 2MB
              output_format: 'srt',
              error_message: 'Failed to process audio stream',
              created_at: '2026-01-18T09:00:00Z',
              updated_at: '2026-01-18T09:05:00Z',
              completed_at: '2026-01-18T09:05:00Z'
            }
          ]
        })
      });
    });

    await page.goto('/dashboard');

    // Wait for jobs to be displayed
    const jobsList = page.locator('.jobs-list');
    await expect(jobsList).toBeVisible();

    // Check that we have 3 job cards
    const jobCards = page.locator('.job-card');
    await expect(jobCards).toHaveCount(3);

    // Check first job (completed)
    const firstJob = jobCards.nth(0);
    await expect(firstJob.locator('.job-title')).toContainText('test-video.mp4');
    await expect(firstJob.locator('.job-status')).toContainText('completed');
    await expect(firstJob.locator('.job-status')).toHaveClass(/status-completed/);

    const downloadBtn = firstJob.locator('.download-btn');
    await expect(downloadBtn).toBeVisible();
    await expect(downloadBtn).not.toBeDisabled();
    await expect(downloadBtn).toHaveAttribute('href', '/api/jobs/1/download');

    // Check second job (pending)
    const secondJob = jobCards.nth(1);
    await expect(secondJob.locator('.job-title')).toContainText('audio-clip.mp3');
    await expect(secondJob.locator('.job-status')).toContainText('pending');
    await expect(secondJob.locator('.job-status')).toHaveClass(/status-pending/);

    const pendingBtn = secondJob.locator('.download-btn');
    await expect(pendingBtn).toBeVisible();
    await expect(pendingBtn).toBeDisabled();

    // Check third job (failed)
    const thirdJob = jobCards.nth(2);
    await expect(thirdJob.locator('.job-title')).toContainText('broken.mp4');
    await expect(thirdJob.locator('.job-status')).toContainText('failed');
    await expect(thirdJob.locator('.job-status')).toHaveClass(/status-failed/);
    await expect(thirdJob.locator('.error-message')).toContainText('Failed to process audio stream');
  });

  test('logout button redirects to home and calls logout API', async ({ page }) => {
    // Mock authentication
    await page.route('**/api/me', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          user: { id: 1, email: 'test@example.com' }
        })
      });
    });

    // Mock jobs list
    await page.route('**/api/jobs', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          jobs: []
        })
      });
    });

    // Mock logout endpoint
    let logoutCalled = false;
    await page.route('**/api/logout', async route => {
      logoutCalled = true;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          message: 'Logged out successfully'
        })
      });
    });

    // Set auth token in localStorage BEFORE navigating so Nav shows logout button
    await page.goto('/');
    await page.evaluate(() => {
      localStorage.setItem('auth_token', 'fake-token');
    });

    await page.goto('/dashboard');

    // Wait for the dashboard to load and show logout button in nav
    const logoutBtn = page.locator('#nav-links #logout-btn');
    await expect(logoutBtn).toBeVisible({ timeout: 5000 });
    await logoutBtn.click();

    // Should redirect to home
    await page.waitForURL('/');
    await expect(page).toHaveURL('/');

    // Verify logout API was called
    expect(logoutCalled).toBe(true);
  });

  test('displays error message when jobs API fails', async ({ page }) => {
    // Mock authentication
    await page.route('**/api/me', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          user: { id: 1, email: 'test@example.com' }
        })
      });
    });

    // Mock jobs API failure
    await page.route('**/api/jobs', async route => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          message: 'Internal server error'
        })
      });
    });

    await page.goto('/dashboard');

    // Check error message is displayed
    const error = page.locator('.error');
    await expect(error).toBeVisible();
    await expect(error).toContainText('Failed to load your jobs');
  });

  test('formats file sizes correctly', async ({ page }) => {
    // Mock authentication
    await page.route('**/api/me', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          user: { id: 1, email: 'test@example.com' }
        })
      });
    });

    // Mock jobs with different file sizes
    await page.route('**/api/jobs', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          jobs: [
            {
              id: 1,
              user_id: 1,
              status: 'completed',
              original_filename: 'small.mp3',
              file_path: '/data/files/uploads/1/1/small.mp3',
              file_size: 1024, // 1KB
              output_format: 'srt',
              created_at: '2026-01-18T10:00:00Z',
              updated_at: '2026-01-18T10:00:00Z'
            },
            {
              id: 2,
              user_id: 1,
              status: 'completed',
              original_filename: 'medium.mp4',
              file_path: '/data/files/uploads/1/2/medium.mp4',
              file_size: 5242880, // 5MB
              output_format: 'srt',
              created_at: '2026-01-18T10:00:00Z',
              updated_at: '2026-01-18T10:00:00Z'
            },
            {
              id: 3,
              user_id: 1,
              status: 'completed',
              original_filename: 'large.mp4',
              file_path: '/data/files/uploads/1/3/large.mp4',
              file_size: 104857600, // 100MB
              output_format: 'srt',
              created_at: '2026-01-18T10:00:00Z',
              updated_at: '2026-01-18T10:00:00Z'
            }
          ]
        })
      });
    });

    await page.goto('/dashboard');

    const jobCards = page.locator('.job-card');

    // Check file sizes are formatted correctly
    await expect(jobCards.nth(0)).toContainText('1 KB');
    await expect(jobCards.nth(1)).toContainText('5 MB');
    await expect(jobCards.nth(2)).toContainText('100 MB');
  });
});
