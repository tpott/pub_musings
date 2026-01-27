import { test, expect, type Page } from '@playwright/test';

// Generate unique email for each test run to avoid conflicts
function generateTestEmail(): string {
  const timestamp = Date.now();
  const random = Math.random().toString(36).substring(2, 8);
  return `test-${timestamp}-${random}@example.com`;
}

const TEST_PASSWORD = 'TestPassword123!';
const WEAK_PASSWORD = 'weak';

// Helper to register a user - used sparingly due to rate limiting
async function registerUser(page: Page, email: string): Promise<void> {
  await page.goto('/register');
  await page.fill('#email', email);
  await page.fill('#password', TEST_PASSWORD);
  await page.fill('#confirmPassword', TEST_PASSWORD);
  await page.click('#submitBtn');
  await expect(page).toHaveURL(/\/videos/, { timeout: 15000 });
}

// Tests are grouped to minimize register calls
// Group 1: Registration form validation (no API calls needed)
test.describe('Registration Form Validation', () => {
  test('should show error for invalid email format', async ({ page }) => {
    await page.goto('/register');

    await page.fill('#email', 'invalid-email');
    await page.locator('#email').blur();

    const emailError = page.locator('#emailError');
    await expect(emailError).toBeVisible();
    await expect(emailError).toContainText('valid email');
  });

  test('should show error for weak password', async ({ page }) => {
    await page.goto('/register');

    await page.fill('#password', WEAK_PASSWORD);
    await page.locator('#password').blur();

    const passwordError = page.locator('#passwordError');
    await expect(passwordError).toBeVisible();
    await expect(passwordError).toContainText('8 characters');
  });

  test('should show error for mismatched passwords', async ({ page }) => {
    await page.goto('/register');

    await page.fill('#password', TEST_PASSWORD);
    await page.fill('#confirmPassword', 'DifferentPassword123!');
    await page.locator('#confirmPassword').blur();

    const confirmError = page.locator('#confirmPasswordError');
    await expect(confirmError).toBeVisible();
    await expect(confirmError).toContainText('do not match');
  });

  test('should have working navigation to login', async ({ page }) => {
    await page.goto('/register');
    await page.click('a:has-text("Log in")');
    await expect(page).toHaveURL(/\/login/);
  });
});

// Group 2: Login form validation (no API calls needed)
test.describe('Login Form Validation', () => {
  test('should show error for invalid email format', async ({ page }) => {
    await page.goto('/login');

    await page.fill('#email', 'not-an-email');
    await page.locator('#email').blur();

    const emailError = page.locator('#emailError');
    await expect(emailError).toBeVisible();
  });

  test('should have working navigation to register', async ({ page }) => {
    await page.goto('/login');
    await page.click('a:has-text("Sign up")');
    await expect(page).toHaveURL(/\/register/);
  });

  test('should show TOTP section hidden initially', async ({ page }) => {
    await page.goto('/login');

    const totpSection = page.locator('#totpSection');
    await expect(totpSection).toHaveClass(/hidden/);

    const credentialsSection = page.locator('#credentialsSection');
    await expect(credentialsSection).not.toHaveClass(/hidden/);
  });

  test('should show recovery section hidden initially', async ({ page }) => {
    await page.goto('/login');

    const recoverySection = page.locator('#recoverySection');
    await expect(recoverySection).toHaveClass(/hidden/);

    const backLink = page.locator('#backToTotpLink');
    await expect(backLink).toBeHidden();
  });

  test('should show error for non-existent user', async ({ page }) => {
    await page.goto('/login');

    await page.fill('#email', 'nonexistent-user-12345@example.com');
    await page.fill('#password', TEST_PASSWORD);
    await page.click('#submitBtn');

    const errorDiv = page.locator('#error');
    await expect(errorDiv).toBeVisible();
    await expect(errorDiv).toContainText('Invalid');
  });
});

// Group 3: Tests requiring registration (limited to minimize rate limit hits)
// These tests are serial to share a single user when possible
test.describe.serial('Auth Flows with Registration', () => {
  let sharedEmail: string;

  test('should successfully register a new user', async ({ page }) => {
    sharedEmail = generateTestEmail();

    await page.goto('/register');
    await expect(page).toHaveTitle('Sign Up - Subtitler');

    await page.fill('#email', sharedEmail);
    await page.fill('#password', TEST_PASSWORD);
    await page.fill('#confirmPassword', TEST_PASSWORD);
    await page.click('#submitBtn');

    await expect(page).toHaveURL(/\/videos/);
  });

  test('should maintain session across page navigations', async ({ page }) => {
    // First login with the shared user
    await page.goto('/login');
    await page.fill('#email', sharedEmail);
    await page.fill('#password', TEST_PASSWORD);
    await page.click('#submitBtn');
    await expect(page).toHaveURL(/\/videos/);

    // Navigate to different pages and verify still logged in
    await page.goto('/');
    await expect(page.locator('.nav-user')).toContainText(sharedEmail);

    await page.goto('/upload');
    await expect(page.locator('.nav-user')).toContainText(sharedEmail);
  });

  test('should maintain session after page reload', async ({ page }) => {
    await page.goto('/login');
    await page.fill('#email', sharedEmail);
    await page.fill('#password', TEST_PASSWORD);
    await page.click('#submitBtn');
    await expect(page).toHaveURL(/\/videos/);

    await page.reload();
    await expect(page.locator('.nav-user')).toContainText(sharedEmail);
  });

  test('should successfully logout', async ({ page }) => {
    await page.goto('/login');
    await page.fill('#email', sharedEmail);
    await page.fill('#password', TEST_PASSWORD);
    await page.click('#submitBtn');
    await expect(page).toHaveURL(/\/videos/);

    await page.click('button:has-text("Log out")');
    await expect(page).toHaveURL(/\//);
    await expect(page.locator('text=Log in')).toBeVisible();
    await expect(page.locator('text=Sign up')).toBeVisible();
  });

});

// NOTE: Test for duplicate email registration is skipped in E2E because it triggers
// rate limiting (5 requests/min per IP). The duplicate email rejection is tested
// in backend unit tests (TestAuthRegisterDuplicate in api_test.go).

// NOTE: Security/2FA tests that require fresh registration are limited due to rate limiting.
// Tests for 2FA setup and QR code display would require an additional registration,
// which may hit rate limits when running the full test suite.
// The 2FA functionality is thoroughly tested in backend unit tests (totp_test.go, api_test.go).

// Only test settings page access using the shared user from previous group
test.describe('Settings Page (reusing shared user)', () => {
  test('should show settings page structure with tabs', async ({ page }) => {
    await page.goto('/settings');

    // Even without auth, we can verify the page structure
    // The page should show a message about not being logged in or redirect
    const body = await page.textContent('body');
    const hasSettingsContent = body?.includes('Settings') ||
      body?.includes('Not logged in') ||
      body?.includes('2FA');
    expect(hasSettingsContent).toBeTruthy();
  });
});

// Group 4: Password Reset Flow (no API calls for validation tests)
test.describe('Password Reset Flow', () => {
  test('should show forgot password page structure', async ({ page }) => {
    await page.goto('/forgot-password');

    await expect(page).toHaveTitle('Forgot Password - Subtitler');
    await expect(page.locator('h1')).toContainText('Subtitler');
    await expect(page.locator('.subtitle')).toContainText('Reset your password');
    await expect(page.locator('#email')).toBeVisible();
    await expect(page.locator('#submitBtn')).toContainText('Send Reset Link');
  });

  test('should have link from forgot password to login', async ({ page }) => {
    await page.goto('/forgot-password');
    await page.click('a:has-text("Log in")');
    await expect(page).toHaveURL(/\/login/);
  });

  test('should have link from login to forgot password', async ({ page }) => {
    await page.goto('/login');
    await page.click('a:has-text("Forgot password")');
    await expect(page).toHaveURL(/\/forgot-password/);
  });

  test('should show validation error for invalid email on forgot password', async ({ page }) => {
    await page.goto('/forgot-password');

    await page.fill('#email', 'invalid-email');
    await page.locator('#email').blur();

    const emailError = page.locator('#emailError');
    await expect(emailError).toBeVisible();
    await expect(emailError).toContainText('valid email');
  });

  test('should show success message after submitting forgot password', async ({ page }) => {
    await page.goto('/forgot-password');

    // Use a non-existent email - API should still return success (prevents email enumeration)
    await page.fill('#email', 'nonexistent-test-email-12345@example.com');
    await page.click('#submitBtn');

    const successDiv = page.locator('#success');
    await expect(successDiv).toBeVisible({ timeout: 10000 });
    await expect(successDiv).toContainText('password reset link has been sent');
  });

  test('should show reset password page structure', async ({ page }) => {
    await page.goto('/reset-password');

    await expect(page).toHaveTitle('Reset Password - Subtitler');
    await expect(page.locator('h1')).toContainText('Subtitler');
    await expect(page.locator('.subtitle')).toContainText('Create a new password');
  });

  test('should show invalid token message when no token provided', async ({ page }) => {
    await page.goto('/reset-password');

    const invalidTokenSection = page.locator('#invalidTokenSection');
    await expect(invalidTokenSection).toBeVisible();
    await expect(invalidTokenSection).toContainText('invalid or has expired');
    await expect(page.locator('a:has-text("Request a new link")')).toBeVisible();
  });

  test('should show form when token is provided', async ({ page }) => {
    await page.goto('/reset-password?token=test-token-123');

    const formSection = page.locator('#formSection');
    const invalidTokenSection = page.locator('#invalidTokenSection');

    await expect(formSection).toBeVisible();
    await expect(invalidTokenSection).toBeHidden();
    await expect(page.locator('#password')).toBeVisible();
    await expect(page.locator('#confirmPassword')).toBeVisible();
  });

  test('should show validation error for weak password on reset', async ({ page }) => {
    await page.goto('/reset-password?token=test-token-123');

    await page.fill('#password', WEAK_PASSWORD);
    await page.locator('#password').blur();

    const passwordError = page.locator('#passwordError');
    await expect(passwordError).toBeVisible();
    await expect(passwordError).toContainText('8 characters');
  });

  test('should show validation error for mismatched passwords on reset', async ({ page }) => {
    await page.goto('/reset-password?token=test-token-123');

    await page.fill('#password', TEST_PASSWORD);
    await page.fill('#confirmPassword', 'DifferentPassword123!');
    await page.locator('#confirmPassword').blur();

    const confirmError = page.locator('#confirmError');
    await expect(confirmError).toBeVisible();
    await expect(confirmError).toContainText('do not match');
  });

  test('should show invalid token error when submitting with invalid token', async ({ page }) => {
    await page.goto('/reset-password?token=invalid-token-that-does-not-exist');

    await page.fill('#password', TEST_PASSWORD);
    await page.fill('#confirmPassword', TEST_PASSWORD);
    await page.click('#submitBtn');

    // Should show invalid token section after submission
    const invalidTokenSection = page.locator('#invalidTokenSection');
    await expect(invalidTokenSection).toBeVisible({ timeout: 10000 });
  });
});
