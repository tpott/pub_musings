import { test, expect } from '@playwright/test';

test.describe('Cookie consent', () => {
  test.beforeEach(async ({ page }) => {
    // Clear localStorage before each test
    await page.goto('/');
    await page.evaluate(() => localStorage.clear());
  });

  test('shows consent banner after delay', async ({ page }) => {
    await page.goto('/');
    // Banner should not be visible immediately
    await expect(page.locator('.cookie-banner')).not.toBeVisible();

    // Wait for banner to appear (1s delay + animation)
    await page.waitForSelector('.cookie-banner', { timeout: 2000 });
    await expect(page.locator('.cookie-banner')).toBeVisible();
  });

  test('hides banner when accepted', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.cookie-banner');

    await page.click('.cookie-banner .accept');

    await expect(page.locator('.cookie-banner')).not.toBeVisible();

    // Verify localStorage
    const consent = await page.evaluate(() => localStorage.getItem('cookie-consent'));
    expect(consent).toBe('accepted');
  });

  test('hides banner when rejected', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.cookie-banner');

    await page.click('.cookie-banner .reject');

    await expect(page.locator('.cookie-banner')).not.toBeVisible();

    // Verify localStorage
    const consent = await page.evaluate(() => localStorage.getItem('cookie-consent'));
    expect(consent).toBe('rejected');
  });

  test('does not show banner if consent already given', async ({ page }) => {
    // Set consent before navigating
    await page.goto('/');
    await page.evaluate(() => localStorage.setItem('cookie-consent', 'accepted'));

    await page.goto('/');

    // Wait enough time for banner to potentially appear
    await page.waitForTimeout(1500);
    await expect(page.locator('.cookie-banner')).not.toBeVisible();
  });

  test('consent persists across page navigations', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('.cookie-banner');
    await page.click('.cookie-banner .accept');

    // Navigate to another page
    await page.goto('/blog');

    // Banner should not appear
    await page.waitForTimeout(1500);
    await expect(page.locator('.cookie-banner')).not.toBeVisible();
  });
});
