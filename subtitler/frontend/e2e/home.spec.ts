import { test, expect } from '@playwright/test';

test.describe('Homepage', () => {
  test('should display the main page with correct title', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle('Subtitler');
  });

  test('should show hero section with tagline', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('h1')).toContainText('Subtitler');
    await expect(page.locator('text=Generate accurate subtitles')).toBeVisible();
  });

  test('should have upload and videos navigation buttons', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Upload a Video')).toBeVisible();
    await expect(page.locator('text=My Videos')).toBeVisible();
  });

  test('should navigate to upload page', async ({ page }) => {
    await page.goto('/');
    await page.click('text=Upload a Video');
    await expect(page).toHaveURL(/\/upload/);
    await expect(page).toHaveTitle('Upload - Subtitler');
  });

  test('should navigate to videos page', async ({ page }) => {
    await page.goto('/');
    await page.click('text=My Videos');
    await expect(page).toHaveURL(/\/videos/);
    await expect(page).toHaveTitle('My Videos - Subtitler');
  });

  test('should show login and signup links for unauthenticated users', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('text=Log in')).toBeVisible();
    await expect(page.locator('text=Sign up')).toBeVisible();
  });
});
