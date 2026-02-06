import { test, expect } from '@playwright/test';
import { getSimpleMockScript } from '../helpers/mock-media-recorder';
import { setupHttpMocks } from '../helpers/e2e-helpers';

test.describe('Feedback submission', () => {
  test('feedback form submission works end-to-end', async ({ page }) => {
    let feedbackRequest: { type: string; message: string; rating?: number } | null = null;
    await page.route('**/api/feedback', async route => {
      const body = await route.request().postDataJSON();
      feedbackRequest = body;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'test-feedback-123' }),
      });
    });

    await page.goto('/');

    const feedbackBtn = page.locator('[data-testid="feedback-btn"]');
    await expect(feedbackBtn).toBeVisible();
    await feedbackBtn.click();

    const modal = page.locator('[data-testid="feedback-modal"]');
    await expect(modal).not.toHaveClass(/hidden/);

    await page.selectOption('[data-testid="feedback-type"]', 'bug');
    await page.fill('[data-testid="feedback-message"]', 'Test feedback message for E2E');

    const starBtns = page.locator('.star-btn');
    await starBtns.nth(3).click();

    await page.click('[data-testid="btn-submit"]');

    const successMessage = page.locator('[data-testid="success-message"]');
    await expect(successMessage).not.toHaveClass(/hidden/, { timeout: 5000 });
    await expect(successMessage).toContainText('Thanks for your feedback');

    expect(feedbackRequest).not.toBeNull();
    expect(feedbackRequest!.type).toBe('bug');
    expect(feedbackRequest!.message).toBe('Test feedback message for E2E');
    expect(feedbackRequest!.rating).toBe(4);
  });

  test('feedback modal closes on cancel', async ({ page }) => {
    await page.goto('/');

    await page.click('[data-testid="feedback-btn"]');
    const modal = page.locator('[data-testid="feedback-modal"]');
    await expect(modal).not.toHaveClass(/hidden/);

    await page.click('[data-testid="btn-cancel"]');

    await expect(modal).toHaveClass(/hidden/);
  });

  test('feedback modal closes on Escape key', async ({ page }) => {
    await page.goto('/');

    await page.click('[data-testid="feedback-btn"]');
    const modal = page.locator('[data-testid="feedback-modal"]');
    await expect(modal).not.toHaveClass(/hidden/);

    await page.keyboard.press('Escape');

    await expect(modal).toHaveClass(/hidden/);
  });

  test('feedback button hides during recording and reappears after', async ({ page }) => {
    // Use HTTP mode so we can control the flow simply
    await page.addInitScript(() => {
      (window as any).__PEEKABOO_USE_WEBSOCKET__ = false;
    });
    await page.addInitScript(getSimpleMockScript());
    await setupHttpMocks(page, 'cat', 'show me a cat');
    await page.goto('/');

    const feedbackBtn = page.locator('[data-testid="feedback-btn"]');
    const micButton = page.locator('[data-testid="mic-button"]');

    // Feedback button should be visible initially
    await expect(feedbackBtn).toBeVisible();

    // Start recording
    await micButton.click();
    await expect(micButton).toHaveAttribute('aria-pressed', 'true');

    // Feedback button should be hidden during recording
    await expect(feedbackBtn).not.toBeVisible();

    // Stop recording (triggers transcribe -> intent -> media flow)
    await micButton.click();

    // Feedback button should reappear after recording stops
    await expect(feedbackBtn).toBeVisible({ timeout: 5000 });
  });
});
