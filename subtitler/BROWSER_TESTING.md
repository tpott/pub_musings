# Browser Testing Guide

This guide covers setting up and running browser automation tests for the Subtitler application.

## Overview

Browser automation testing (also called end-to-end or E2E testing) simulates real user interactions with the application in a browser. This complements unit tests by testing the full user experience.

## Recommended Tool: Playwright

[Playwright](https://playwright.dev/) is recommended for browser testing due to its cross-browser support, reliability, and excellent TypeScript integration.

### Installation

```bash
cd frontend
npm install -D @playwright/test
npx playwright install
```

This installs Playwright and downloads browser binaries.

### Configuration

The project uses `playwright.config.ts` in the frontend directory. Currently only Chromium is configured for testing (to reduce test execution time and CI complexity):

```typescript
// frontend/playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',

  use: {
    baseURL: 'http://localhost:4321',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    // Firefox and WebKit can be enabled by adding:
    // { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    // { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],

  // Start dev server before running tests
  webServer: [
    {
      command: 'cd ../backend && go run main.go',
      url: 'http://localhost:8080/api/health',
      reuseExistingServer: !process.env.CI,
      timeout: 120000,
    },
    {
      command: 'npm run dev',
      url: 'http://localhost:4321',
      reuseExistingServer: !process.env.CI,
      timeout: 120000,
    },
  ],
});
```

**Note:** To enable cross-browser testing, add Firefox and/or WebKit projects to the `projects` array and run `npx playwright install` to download additional browser binaries.

### Add Scripts

Add to `frontend/package.json`:

```json
{
  "scripts": {
    "test:e2e": "playwright test",
    "test:e2e:ui": "playwright test --ui",
    "test:e2e:headed": "playwright test --headed",
    "test:e2e:debug": "playwright test --debug"
  }
}
```

### Directory Structure

```
frontend/
├── e2e/
│   ├── home.spec.ts
│   ├── upload.spec.ts
│   ├── auth.spec.ts
│   └── fixtures/
│       └── test-video.mp4
├── playwright.config.ts
└── package.json
```

## Writing Tests

### Example: Homepage Test

```typescript
// frontend/e2e/home.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Homepage', () => {
  test('should display the main page', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/Subtitler/);
  });

  test('should navigate to upload page', async ({ page }) => {
    await page.goto('/');
    await page.click('text=Upload');
    await expect(page).toHaveURL(/\/upload/);
  });
});
```

### Example: Upload Flow Test

```typescript
// frontend/e2e/upload.spec.ts
import { test, expect } from '@playwright/test';
import path from 'path';

test.describe('Video Upload', () => {
  test('should upload a video file', async ({ page }) => {
    await page.goto('/upload');

    // Upload file
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles(path.join(__dirname, 'fixtures/test-video.mp4'));

    // Check progress indicator
    await expect(page.locator('.upload-progress')).toBeVisible();

    // Wait for upload complete
    await expect(page.locator('.upload-complete')).toBeVisible({ timeout: 30000 });
  });

  test('should reject files over 500MB', async ({ page }) => {
    await page.goto('/upload');

    // Try uploading oversized file
    const fileInput = page.locator('input[type="file"]');
    // Create mock file > 500MB would fail with error

    await expect(page.locator('.error-message')).toContainText('500MB');
  });
});
```

### Example: Authentication Test

```typescript
// frontend/e2e/auth.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Authentication', () => {
  test('should register a new user', async ({ page }) => {
    await page.goto('/register');

    await page.fill('input[name="email"]', 'test@example.com');
    await page.fill('input[name="password"]', 'SecurePassword123!');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL(/\/videos/);
  });

  test('should login existing user', async ({ page }) => {
    await page.goto('/login');

    await page.fill('input[name="email"]', 'test@example.com');
    await page.fill('input[name="password"]', 'SecurePassword123!');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL(/\/videos/);
  });

  test('should show error for invalid credentials', async ({ page }) => {
    await page.goto('/login');

    await page.fill('input[name="email"]', 'wrong@example.com');
    await page.fill('input[name="password"]', 'wrongpassword');
    await page.click('button[type="submit"]');

    await expect(page.locator('.error-message')).toBeVisible();
  });
});
```

## Running Tests

### Run All Tests
```bash
cd frontend
npm run test:e2e
```

### Run Specific Test File
```bash
npx playwright test e2e/upload.spec.ts
```

### Run in Headed Mode (See Browser)
```bash
npm run test:e2e:headed
```

### Run with Debug UI
```bash
npm run test:e2e:debug
```

### Run Interactive UI Mode
```bash
npm run test:e2e:ui
```

### Run on Specific Browser
```bash
npx playwright test --project=chromium
# Firefox and WebKit require enabling in playwright.config.ts first:
# npx playwright test --project=firefox
# npx playwright test --project=webkit
```

## Test Fixtures

### Test Video File

For upload tests, create a small test video:

```bash
# Create a 5-second test video with ffmpeg
ffmpeg -f lavfi -i testsrc=duration=5:size=320x240:rate=30 \
       -f lavfi -i sine=frequency=1000:duration=5 \
       -c:v libx264 -c:a aac \
       frontend/e2e/fixtures/test-video.mp4
```

### Database Reset

For consistent tests, reset the database before each test suite:

```typescript
// frontend/e2e/global-setup.ts
import { execSync } from 'child_process';

export default async function globalSetup() {
  // Reset test database
  execSync('rm -f ../backend/data/subtitler.db');
}
```

Add to `playwright.config.ts`:
```typescript
export default defineConfig({
  globalSetup: './e2e/global-setup.ts',
  // ...
});
```

## CI/CD Integration

### GitHub Actions Example

```yaml
# .github/workflows/e2e.yml
name: E2E Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: 20

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install ffmpeg
        run: sudo apt-get install -y ffmpeg

      - name: Install dependencies
        run: |
          cd frontend
          npm ci
          npx playwright install --with-deps

      - name: Run E2E tests
        run: |
          cd frontend
          npm run test:e2e

      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: playwright-report
          path: frontend/playwright-report/
```

## Debugging Failed Tests

### View Test Report
```bash
npx playwright show-report
```

### Trace Viewer
Failed tests generate traces. View them with:
```bash
npx playwright show-trace trace.zip
```

### Screenshots
Screenshots are captured on failure in `test-results/` directory.

## Alternative: Cypress

[Cypress](https://www.cypress.io/) is another popular option for browser testing.

### Installation
```bash
cd frontend
npm install -D cypress
npx cypress open
```

### Configuration
```javascript
// frontend/cypress.config.js
const { defineConfig } = require('cypress');

module.exports = defineConfig({
  e2e: {
    baseUrl: 'http://localhost:4321',
    supportFile: 'cypress/support/e2e.js',
  },
});
```

## Quick Reference

| Command | Description |
|---------|-------------|
| `npm run test:e2e` | Run all E2E tests |
| `npm run test:e2e:ui` | Run with interactive UI |
| `npm run test:e2e:headed` | Run with visible browser |
| `npm run test:e2e:debug` | Run in debug mode |
| `npx playwright show-report` | View test report |
| `npx playwright codegen` | Generate tests by recording |

## See Also

- [README.md](README.md) - Project overview
- [TESTING.md](TESTING.md) - Unit and integration tests
- [INSTALL.md](INSTALL.md) - Installing dependencies
- [Playwright Documentation](https://playwright.dev/docs/intro)
