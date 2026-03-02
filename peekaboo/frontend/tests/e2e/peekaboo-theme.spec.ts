import { test, expect } from '@playwright/test';

test.describe('Theme toggle', () => {
  test('cycles through light → dark → auto modes', async ({ page }) => {
    await page.goto('/');

    const html = page.locator('html');
    const toggle = page.getByTestId('theme-toggle');

    // Initial state should be auto (default when no localStorage)
    await expect(html).toHaveAttribute('data-theme-mode', 'auto');
    await expect(toggle).toHaveAttribute('aria-label', /auto mode/);

    // Click 1: auto → light
    await toggle.click();
    await expect(html).toHaveAttribute('data-theme-mode', 'light');
    await expect(toggle).toHaveAttribute('aria-label', /light mode/);

    // Click 2: light → dark
    await toggle.click();
    await expect(html).toHaveAttribute('data-theme-mode', 'dark');
    await expect(toggle).toHaveAttribute('aria-label', /dark mode/);
    // Dark mode should add 'dark' class
    await expect(html).toHaveClass(/dark/);

    // Click 3: dark → auto
    await toggle.click();
    await expect(html).toHaveAttribute('data-theme-mode', 'auto');
    await expect(toggle).toHaveAttribute('aria-label', /auto mode/);
  });

  test('persists theme choice across page reloads', async ({ page }) => {
    await page.goto('/');

    const html = page.locator('html');
    const toggle = page.getByTestId('theme-toggle');

    // Set to dark mode: auto → light → dark
    await toggle.click(); // auto → light
    await toggle.click(); // light → dark
    await expect(html).toHaveAttribute('data-theme-mode', 'dark');

    // Reload the page
    await page.reload();

    // Theme should persist
    await expect(html).toHaveAttribute('data-theme-mode', 'dark');
    await expect(html).toHaveClass(/dark/);
  });

  test('auto mode removes localStorage entry', async ({ page }) => {
    await page.goto('/');

    const toggle = page.getByTestId('theme-toggle');

    // Set to light: auto → light
    await toggle.click();

    // Verify localStorage has the value
    const savedLight = await page.evaluate(() => localStorage.getItem('peekaboo:theme'));
    expect(savedLight).toBe('light');

    // Cycle to dark: light → dark
    await toggle.click();
    const savedDark = await page.evaluate(() => localStorage.getItem('peekaboo:theme'));
    expect(savedDark).toBe('dark');

    // Cycle to auto: dark → auto
    await toggle.click();
    const savedAuto = await page.evaluate(() => localStorage.getItem('peekaboo:theme'));
    expect(savedAuto).toBeNull();
  });
});
