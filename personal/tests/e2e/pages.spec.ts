import { test, expect } from '@playwright/test';

test.describe('Page rendering', () => {
  test('home page loads correctly', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/Trevor Pottinger/);
    await expect(page.locator('h1')).toContainText("Hi, I'm Trevor");
  });

  test('blog page loads correctly', async ({ page }) => {
    await page.goto('/blog');
    await expect(page).toHaveTitle(/Blog/);
    await expect(page.locator('h1')).toContainText('Blog');
  });

  test('contact page loads correctly', async ({ page }) => {
    await page.goto('/contact');
    await expect(page).toHaveTitle(/Contact/);
    await expect(page.locator('h1')).toContainText('Contact');
  });

  test('navigation works correctly', async ({ page }) => {
    await page.goto('/');

    // Navigate to blog
    await page.click('nav a[href="/blog"]');
    await expect(page).toHaveURL('/blog');
    await expect(page.locator('h1')).toContainText('Blog');

    // Navigate to contact
    await page.click('nav a[href="/contact"]');
    await expect(page).toHaveURL('/contact');
    await expect(page.locator('h1')).toContainText('Contact');

    // Navigate back home via logo
    await page.click('nav a.logo');
    await expect(page).toHaveURL('/');
  });

  test('blog post renders from markdown', async ({ page }) => {
    await page.goto('/blog');

    // Click on the first blog post
    const firstPost = page.locator('article a').first();
    await firstPost.click();

    // Should be on a blog post page
    await expect(page.locator('article')).toBeVisible();
    await expect(page.getByRole('link', { name: '← Back to blog' })).toBeVisible();
  });
});
