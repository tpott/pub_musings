import { test, expect } from '@playwright/test';

test.describe('Contact form', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/contact');
    // Wait for React hydration
    await page.waitForSelector('[data-hydrated="true"]');
  });

  test('form hydrates correctly', async ({ page }) => {
    await expect(page.locator('form.contact-form')).toBeVisible();
    await expect(page.locator('#name')).toBeVisible();
    await expect(page.locator('#email')).toBeVisible();
    await expect(page.locator('#message')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toBeVisible();
  });

  test('shows validation errors for empty fields', async ({ page }) => {
    await page.click('button[type="submit"]');

    await expect(page.locator('#name-error')).toContainText('Name is required');
    await expect(page.locator('#email-error')).toContainText('Email is required');
    await expect(page.locator('#message-error')).toContainText('Message is required');
  });

  test('validates email format', async ({ page }) => {
    await page.fill('#name', 'Test User');
    await page.fill('#email', 'invalid-email');
    await page.fill('#message', 'This is a test message');
    await page.click('button[type="submit"]');

    await expect(page.locator('#email-error')).toContainText('valid email');
  });

  test('validates message length', async ({ page }) => {
    await page.fill('#name', 'Test User');
    await page.fill('#email', 'test@example.com');
    await page.fill('#message', 'Short');
    await page.click('button[type="submit"]');

    await expect(page.locator('#message-error')).toContainText('at least 10 characters');
  });

  test('clears errors when typing', async ({ page }) => {
    // Trigger validation error
    await page.click('button[type="submit"]');
    await expect(page.locator('#name-error')).toBeVisible();

    // Start typing - error should clear
    await page.fill('#name', 'T');
    await expect(page.locator('#name-error')).not.toBeVisible();
  });

  test('honeypot field is hidden', async ({ page }) => {
    const honeypot = page.locator('#website');
    // Field should exist but not be visible
    await expect(honeypot).toHaveCount(1);
    await expect(honeypot).not.toBeInViewport();
  });
});
