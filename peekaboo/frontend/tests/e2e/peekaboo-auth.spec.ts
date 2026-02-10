import { test, expect, Page } from '@playwright/test';

/**
 * Mock the /api/auth/me endpoint. Returns user if authenticated, 401 otherwise.
 */
async function mockMeEndpoint(page: Page, authenticated: boolean, user = { id: 'user-1', email: 'test@example.com', totp_enabled: false, email_verified: true }): Promise<void> {
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

test.describe('Auth: Registration', () => {
  test('register new account shows success message', async ({ page }) => {
    await page.route('**/api/auth/register', route => {
      route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          message: 'Account created. Please check your email to verify your account.',
          email_verification: true,
          user: { id: 'user-1', email: 'new@example.com' },
        }),
      });
    });

    await page.goto('/register');

    await page.fill('#email', 'new@example.com');
    await page.fill('#password', 'securepassword123');
    await page.click('#submit-btn');

    // Form should be hidden, success message shown
    const formSuccess = page.locator('#form-success');
    await expect(formSuccess).toBeVisible({ timeout: 5000 });
    await expect(formSuccess).toContainText('check your email');

    const form = page.locator('#register-form');
    await expect(form).toBeHidden();
  });

  test('register with duplicate email shows error', async ({ page }) => {
    await page.route('**/api/auth/register', route => {
      route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'email already registered' }),
      });
    });

    await page.goto('/register');

    await page.fill('#email', 'existing@example.com');
    await page.fill('#password', 'securepassword123');
    await page.click('#submit-btn');

    const formError = page.locator('#form-error');
    await expect(formError).toBeVisible({ timeout: 5000 });
    await expect(formError).toContainText('email already registered');

    // Form should remain visible for retry
    await expect(page.locator('#register-form')).toBeVisible();
  });

  test('register shows client-side validation errors', async ({ page }) => {
    await page.goto('/register');

    // Submit empty form
    await page.click('#submit-btn');

    // Email error should appear
    await expect(page.locator('#email-error')).toBeVisible();
    await expect(page.locator('#email-error')).toContainText('Email is required');

    // Fill invalid email, submit again
    await page.fill('#email', 'notanemail');
    await page.click('#submit-btn');
    await expect(page.locator('#email-error')).toContainText('valid email');

    // Fill valid email but short password
    await page.fill('#email', 'user@example.com');
    await page.click('#submit-btn');
    await expect(page.locator('#password-error')).toBeVisible();
    await expect(page.locator('#password-error')).toContainText('Password is required');

    await page.fill('#password', 'short');
    await page.click('#submit-btn');
    await expect(page.locator('#password-error')).toContainText('at least 8 characters');
  });
});

test.describe('Auth: Login', () => {
  test('login with valid credentials redirects to home', async ({ page }) => {
    await page.route('**/api/auth/login', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          user: { id: 'user-1', email: 'test@example.com', totp_enabled: false },
          token: 'session-token-abc',
        }),
      });
    });

    await page.goto('/login');

    await page.fill('#email', 'test@example.com');
    await page.fill('#password', 'correctpassword');

    // Listen for navigation
    const navigationPromise = page.waitForURL('/', { timeout: 5000 });
    await page.click('#submit-btn');
    await navigationPromise;
  });

  test('login with wrong password shows error', async ({ page }) => {
    await page.route('**/api/auth/login', route => {
      route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'invalid credentials' }),
      });
    });

    await page.goto('/login');

    await page.fill('#email', 'test@example.com');
    await page.fill('#password', 'wrongpassword');
    await page.click('#submit-btn');

    const formError = page.locator('#form-error');
    await expect(formError).toBeVisible({ timeout: 5000 });
    await expect(formError).toContainText('invalid credentials');

    // Should stay on login page
    expect(page.url()).toContain('/login');
  });

  test('login with unverified email shows verification prompt', async ({ page }) => {
    await page.route('**/api/auth/login', route => {
      route.fulfill({
        status: 403,
        contentType: 'application/json',
        body: JSON.stringify({
          error: 'Please verify your email',
          email_not_verified: true,
          can_resend_verification: true,
        }),
      });
    });

    await page.goto('/login');

    await page.fill('#email', 'unverified@example.com');
    await page.fill('#password', 'correctpassword');
    await page.click('#submit-btn');

    const formError = page.locator('#form-error');
    await expect(formError).toBeVisible({ timeout: 5000 });
    await expect(formError).toContainText('verify your email');

    // Should stay on login page
    expect(page.url()).toContain('/login');
  });

  test('login with locked account shows lockout message', async ({ page }) => {
    await page.route('**/api/auth/login', route => {
      route.fulfill({
        status: 429,
        contentType: 'application/json',
        body: JSON.stringify({
          error: 'too many failed login attempts',
          retry_after_min: 15,
        }),
      });
    });

    await page.goto('/login');

    await page.fill('#email', 'locked@example.com');
    await page.fill('#password', 'anypassword');
    await page.click('#submit-btn');

    const formError = page.locator('#form-error');
    await expect(formError).toBeVisible({ timeout: 5000 });
    await expect(formError).toContainText('Account locked');
    await expect(formError).toContainText('15 minutes');
  });

  test('login requiring TOTP shows code field', async ({ page }) => {
    let loginAttempt = 0;

    await page.route('**/api/auth/login', route => {
      loginAttempt++;
      if (loginAttempt === 1) {
        // First attempt: TOTP required
        route.fulfill({
          status: 401,
          contentType: 'application/json',
          body: JSON.stringify({ totp_required: true }),
        });
      } else {
        // Second attempt with TOTP: success
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            user: { id: 'user-1', email: 'secure@example.com', totp_enabled: true },
            token: 'session-token',
          }),
        });
      }
    });

    await page.goto('/login');

    // First login attempt
    await page.fill('#email', 'secure@example.com');
    await page.fill('#password', 'correctpassword');
    await page.click('#submit-btn');

    // TOTP group should become visible
    const totpGroup = page.locator('#totp-group');
    await expect(totpGroup).toBeVisible({ timeout: 5000 });

    // Fill TOTP and submit again
    await page.fill('#totp', '123456');

    const navigationPromise = page.waitForURL('/', { timeout: 5000 });
    await page.click('#submit-btn');
    await navigationPromise;
  });

  test('login shows client-side validation errors', async ({ page }) => {
    await page.goto('/login');

    // Submit empty form
    await page.click('#submit-btn');
    await expect(page.locator('#email-error')).toBeVisible();
    await expect(page.locator('#email-error')).toContainText('Email is required');
  });
});

test.describe('Auth: Email Verification', () => {
  test('verify email with valid token shows success', async ({ page }) => {
    await page.route('**/api/auth/verify?token=valid-token-123', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Email verified successfully. You can now log in.' }),
      });
    });

    await page.goto('/verify-email?token=valid-token-123');

    const statusText = page.locator('#status-text');
    await expect(statusText).toContainText('Email verified', { timeout: 5000 });

    // Login link should appear
    await expect(page.locator('#links')).toBeVisible();
    await expect(page.locator('#login-link')).toBeVisible();
  });

  test('verify email with expired token shows error and resend link', async ({ page }) => {
    await page.route('**/api/auth/verify**', route => {
      route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'token expired' }),
      });
    });

    await page.goto('/verify-email?token=expired-token');

    const statusText = page.locator('#status-text');
    await expect(statusText).toContainText('token expired', { timeout: 5000 });

    // Resend link should appear
    await expect(page.locator('#resend-link')).toBeVisible();
  });

  test('verify email with missing token shows error', async ({ page }) => {
    await page.goto('/verify-email');

    const statusText = page.locator('#status-text');
    await expect(statusText).toContainText('Missing verification token', { timeout: 5000 });
  });
});

test.describe('Auth: Magic Link', () => {
  test('magic link with valid token signs in and redirects', async ({ page }) => {
    await page.route('**/api/auth/magic-link/verify**', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          message: 'Signed in',
          user: { id: 'user-1', email: 'test@example.com', totp_enabled: false },
        }),
      });
    });

    await page.goto('/magic-link?token=magic-token-abc');

    const statusText = page.locator('#status-text');
    await expect(statusText).toContainText('Signed in', { timeout: 5000 });

    // Should redirect to home after ~1 second
    await page.waitForURL('/', { timeout: 5000 });
  });

  test('magic link with expired token shows error and login link', async ({ page }) => {
    await page.route('**/api/auth/magic-link/verify**', route => {
      route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'token expired' }),
      });
    });

    await page.goto('/magic-link?token=expired-magic-token');

    const statusText = page.locator('#status-text');
    await expect(statusText).toContainText('token expired', { timeout: 5000 });

    // Login link should appear
    await expect(page.locator('#login-link')).toBeVisible();
  });

  test('magic link with missing token shows error', async ({ page }) => {
    await page.goto('/magic-link');

    const statusText = page.locator('#status-text');
    await expect(statusText).toContainText('Missing magic link token', { timeout: 5000 });
  });
});

test.describe('Auth: Logout', () => {
  test('logout button is visible when authenticated and logs out', async ({ page }) => {
    await mockMeEndpoint(page, true);

    await page.route('**/api/auth/csrf', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ token: 'csrf-token-123' }),
      });
    });

    await page.route('**/api/auth/logout', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Logged out' }),
      });
    });

    await page.goto('/');

    const logoutBtn = page.locator('[data-testid="logout-button"]');
    await expect(logoutBtn).toBeVisible({ timeout: 5000 });

    // Click logout — should redirect to /login
    const navigationPromise = page.waitForURL('**/login', { timeout: 5000 });
    await logoutBtn.click();
    await navigationPromise;
  });

  test('logout button is hidden when not authenticated', async ({ page }) => {
    await mockMeEndpoint(page, false);

    await page.route('**/api/auth/csrf', route => {
      route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'not authenticated' }),
      });
    });

    await page.goto('/');

    // Button should exist in DOM but be hidden
    const logoutBtn = page.locator('[data-testid="logout-button"]');
    await expect(logoutBtn).toBeHidden();
  });
});

test.describe('Auth: Navigation Links', () => {
  test('login page has link to register', async ({ page }) => {
    await page.goto('/login');
    const registerLink = page.locator('a[href="/register"]');
    await expect(registerLink).toBeVisible();
    await expect(registerLink).toContainText('Create an account');
  });

  test('register page has link to login', async ({ page }) => {
    await page.goto('/register');
    const loginLink = page.locator('a[href="/login"]');
    await expect(loginLink).toBeVisible();
    await expect(loginLink).toContainText('Sign in');
  });
});

test.describe('Auth: Login Button', () => {
  test('login button visible when not authenticated', async ({ page }) => {
    await mockMeEndpoint(page, false);

    await page.route('**/api/auth/csrf', route => {
      route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'not authenticated' }),
      });
    });

    await page.goto('/');

    const loginBtn = page.locator('[data-testid="login-button"]');
    await expect(loginBtn).toBeVisible({ timeout: 5000 });
    await expect(loginBtn).toHaveAttribute('href', '/login');
  });

  test('login button hidden when authenticated', async ({ page }) => {
    await mockMeEndpoint(page, true);

    await page.route('**/api/auth/csrf', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ token: 'csrf-token-123' }),
      });
    });

    await page.goto('/');

    const loginBtn = page.locator('[data-testid="login-button"]');
    await expect(loginBtn).toBeHidden();
  });
});
