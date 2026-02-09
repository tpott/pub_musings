import { test, expect, Page } from '@playwright/test';

/**
 * Mock the /api/auth/me endpoint.
 */
async function mockMeEndpoint(
  page: Page,
  authenticated: boolean,
  user = {
    id: 'user-1',
    email: 'test@example.com',
    totp_enabled: false,
    email_verified: true,
    created_at: '2026-01-15T10:30:00Z',
  },
): Promise<void> {
  await page.route('**/api/auth/me', route => {
    if (authenticated) {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ user }),
      });
    } else {
      route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'not authenticated' }),
      });
    }
  });
}

/**
 * Mock the CSRF token endpoint.
 */
async function mockCSRFEndpoint(page: Page): Promise<void> {
  await page.route('**/api/auth/csrf', route => {
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ token: 'csrf-token-123' }),
    });
  });
}

test.describe('Settings: Page Access', () => {
  test('redirects to login when not authenticated', async ({ page }) => {
    await mockMeEndpoint(page, false);
    await page.goto('/settings');
    await page.waitForURL('**/login', { timeout: 5000 });
  });

  test('shows account info when authenticated', async ({ page }) => {
    await mockMeEndpoint(page, true);
    await mockCSRFEndpoint(page);
    await page.goto('/settings');

    const accountInfo = page.locator('#account-info');
    await expect(accountInfo).toBeVisible({ timeout: 5000 });

    await expect(page.locator('#user-email')).toContainText('test@example.com');
    await expect(page.locator('#email-verified')).toContainText('Yes');
    await expect(page.locator('#totp-status')).toContainText('Disabled');
  });

  test('shows TOTP enabled status when user has 2FA', async ({ page }) => {
    await mockMeEndpoint(page, true, {
      id: 'user-1',
      email: 'secure@example.com',
      totp_enabled: true,
      email_verified: true,
      created_at: '2026-01-15T10:30:00Z',
    });
    await mockCSRFEndpoint(page);
    await page.goto('/settings');

    await expect(page.locator('#account-info')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('#totp-status')).toContainText('Enabled');

    // Setup button should be hidden, disable button should be visible
    await expect(page.locator('#totp-setup-area')).toBeHidden();
    await expect(page.locator('#totp-disable-area')).toBeVisible();
  });
});

test.describe('Settings: TOTP Setup Flow', () => {
  test('setup button shows secret and verify form', async ({ page }) => {
    await mockMeEndpoint(page, true);
    await mockCSRFEndpoint(page);

    await page.route('**/api/auth/totp/setup', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          secret: 'JBSWY3DPEBLW64TMMQ',
          uri: 'otpauth://totp/Peekaboo:test@example.com?secret=JBSWY3DPEBLW64TMMQ&issuer=Peekaboo',
        }),
      });
    });

    await page.goto('/settings');
    await expect(page.locator('#account-info')).toBeVisible({ timeout: 5000 });

    // Click setup button
    await page.click('#setup-totp-btn');

    // Setup flow should appear with secret
    await expect(page.locator('#totp-setup-flow')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('#totp-secret-display')).toContainText('JBSWY3DPEBLW64TMMQ');

    // Verify form should be visible
    await expect(page.locator('#totp-verify-form')).toBeVisible();
  });

  test('enable TOTP with valid code succeeds', async ({ page }) => {
    await mockMeEndpoint(page, true);
    await mockCSRFEndpoint(page);

    await page.route('**/api/auth/totp/setup', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          secret: 'JBSWY3DPEBLW64TMMQ',
          uri: 'otpauth://totp/Peekaboo:test@example.com?secret=JBSWY3DPEBLW64TMMQ',
        }),
      });
    });

    await page.route('**/api/auth/totp/enable', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'TOTP enabled' }),
      });
    });

    await page.goto('/settings');
    await expect(page.locator('#account-info')).toBeVisible({ timeout: 5000 });

    // Start setup
    await page.click('#setup-totp-btn');
    await expect(page.locator('#totp-setup-flow')).toBeVisible({ timeout: 5000 });

    // Fill code and password, submit
    await page.fill('#totp-code', '123456');
    await page.fill('#setup-password', 'mypassword');
    await page.click('#verify-totp-btn');

    // Success message should appear and TOTP status should change
    await expect(page.locator('#setup-success')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('#setup-success')).toContainText('enabled successfully');
    await expect(page.locator('#totp-status')).toContainText('Enabled');
  });

  test('enable TOTP with invalid code shows error', async ({ page }) => {
    await mockMeEndpoint(page, true);
    await mockCSRFEndpoint(page);

    await page.route('**/api/auth/totp/setup', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          secret: 'JBSWY3DPEBLW64TMMQ',
          uri: 'otpauth://totp/Peekaboo:test@example.com?secret=JBSWY3DPEBLW64TMMQ',
        }),
      });
    });

    await page.route('**/api/auth/totp/enable', route => {
      route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'invalid TOTP code' }),
      });
    });

    await page.goto('/settings');
    await expect(page.locator('#account-info')).toBeVisible({ timeout: 5000 });

    await page.click('#setup-totp-btn');
    await expect(page.locator('#totp-setup-flow')).toBeVisible({ timeout: 5000 });

    await page.fill('#totp-code', '000000');
    await page.fill('#setup-password', 'mypassword');
    await page.click('#verify-totp-btn');

    await expect(page.locator('#setup-error')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('#setup-error')).toContainText('invalid TOTP code');
  });

  test('cancel setup hides the flow', async ({ page }) => {
    await mockMeEndpoint(page, true);
    await mockCSRFEndpoint(page);

    await page.route('**/api/auth/totp/setup', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          secret: 'JBSWY3DPEBLW64TMMQ',
          uri: 'otpauth://totp/Peekaboo:test@example.com?secret=JBSWY3DPEBLW64TMMQ',
        }),
      });
    });

    await page.goto('/settings');
    await expect(page.locator('#account-info')).toBeVisible({ timeout: 5000 });

    await page.click('#setup-totp-btn');
    await expect(page.locator('#totp-setup-flow')).toBeVisible({ timeout: 5000 });

    // Cancel
    await page.click('#cancel-setup-btn');
    await expect(page.locator('#totp-setup-flow')).toBeHidden();
  });
});

test.describe('Settings: TOTP Disable Flow', () => {
  test('disable TOTP with correct password succeeds', async ({ page }) => {
    await mockMeEndpoint(page, true, {
      id: 'user-1',
      email: 'test@example.com',
      totp_enabled: true,
      email_verified: true,
      created_at: '2026-01-15T10:30:00Z',
    });
    await mockCSRFEndpoint(page);

    await page.route('**/api/auth/totp/disable', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'TOTP disabled' }),
      });
    });

    await page.goto('/settings');
    await expect(page.locator('#account-info')).toBeVisible({ timeout: 5000 });

    // Click disable button
    await page.click('#disable-totp-btn');
    await expect(page.locator('#totp-disable-flow')).toBeVisible({ timeout: 5000 });

    // Enter password and confirm
    await page.fill('#disable-password', 'mypassword');
    await page.click('#confirm-disable-btn');

    // TOTP status should change to Disabled
    await expect(page.locator('#totp-status')).toContainText('Disabled', { timeout: 5000 });
    await expect(page.locator('#totp-disable-flow')).toBeHidden();
  });

  test('disable TOTP with wrong password shows error', async ({ page }) => {
    await mockMeEndpoint(page, true, {
      id: 'user-1',
      email: 'test@example.com',
      totp_enabled: true,
      email_verified: true,
      created_at: '2026-01-15T10:30:00Z',
    });
    await mockCSRFEndpoint(page);

    await page.route('**/api/auth/totp/disable', route => {
      route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'invalid password' }),
      });
    });

    await page.goto('/settings');
    await expect(page.locator('#account-info')).toBeVisible({ timeout: 5000 });

    await page.click('#disable-totp-btn');
    await expect(page.locator('#totp-disable-flow')).toBeVisible({ timeout: 5000 });

    await page.fill('#disable-password', 'wrongpassword');
    await page.click('#confirm-disable-btn');

    await expect(page.locator('#disable-error')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('#disable-error')).toContainText('invalid password');
  });
});

test.describe('Settings: Main Page Integration', () => {
  test('settings button visible when authenticated', async ({ page }) => {
    await mockMeEndpoint(page, true);
    await mockCSRFEndpoint(page);

    await page.goto('/');

    const settingsBtn = page.locator('[data-testid="settings-button"]');
    await expect(settingsBtn).toBeVisible({ timeout: 5000 });

    // Should link to /settings
    await expect(settingsBtn).toHaveAttribute('href', '/settings');
  });

  test('settings button hidden when not authenticated', async ({ page }) => {
    await mockMeEndpoint(page, false);
    await mockCSRFEndpoint(page);

    await page.goto('/');

    const settingsBtn = page.locator('[data-testid="settings-button"]');
    await expect(settingsBtn).toBeHidden();
  });
});
