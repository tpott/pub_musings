import { test, expect } from '@playwright/test';
import { getWebSocketMockScript } from '../helpers/mock-media-recorder';
import { routeFixtures } from '../helpers/e2e-helpers';

test.describe('WebSocket error handling', () => {
  test('handles server sending invalid JSON gracefully', async ({ page }) => {
    let invalidJsonSent = false;
    let commandProcessed = false;

    await page.addInitScript(getWebSocketMockScript());

    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioReceived = false;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'stop_recording' && audioReceived) {
              if (!invalidJsonSent) {
                ws.send('this is { not valid json [[[');
                ws.send('{incomplete json');
                invalidJsonSent = true;
              }

              setTimeout(() => {
                ws.send(JSON.stringify({ type: 'transcript', text: 'show me a cat' }));
                ws.send(JSON.stringify({
                  type: 'media',
                  subject: 'cat',
                  photo_url: '/fixtures/mock-cat-photo.jpg',
                  audio_url: '/fixtures/mock-cat-audio.mp3',
                }));
                commandProcessed = true;
              }, 50);

              audioReceived = false;
            } else if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            }
          } catch {
            audioReceived = true;
          }
        } else {
          audioReceived = true;
        }
      });
    });

    await routeFixtures(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const img = page.locator('[data-testid="media-image"]');

    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    await expect(img).toBeVisible({ timeout: 10000 });
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    expect(invalidJsonSent).toBe(true);
    expect(commandProcessed).toBe(true);
  });

  test('handles server closing connection mid-recording', async ({ page }) => {
    let closeConnectionOnFirstCommand = true;

    await page.addInitScript(getWebSocketMockScript());

    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioChunkCount = 0;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            }
          } catch {
            // Not JSON
          }
        } else {
          audioChunkCount++;
          if (closeConnectionOnFirstCommand && audioChunkCount >= 2) {
            ws.close();
            closeConnectionOnFirstCommand = false;
          }
        }
      });
    });

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const mediaDisplay = page.locator('[data-testid="media-display"]');

    await micButton.click();
    await page.waitForTimeout(1000);
    await page.waitForTimeout(1000);

    await expect(mediaDisplay).toContainText(/connection|lost|error|try again/i);
  });

  test('client can retry after server error message', async ({ page }) => {
    let attemptCount = 0;
    let commandProcessed = false;

    await page.addInitScript(getWebSocketMockScript());

    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioReceived = false;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'stop_recording' && audioReceived) {
              attemptCount++;
              if (attemptCount === 1) {
                ws.send(JSON.stringify({ type: 'error', message: 'Server temporarily unavailable' }));
              } else {
                ws.send(JSON.stringify({ type: 'transcript', text: 'show me a cat' }));
                setTimeout(() => {
                  ws.send(JSON.stringify({
                    type: 'media',
                    subject: 'cat',
                    photo_url: '/fixtures/mock-cat-photo.jpg',
                    audio_url: '/fixtures/mock-cat-audio.mp3',
                  }));
                  commandProcessed = true;
                }, 50);
              }
              audioReceived = false;
            } else if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            } else if (parsed.type === 'start_recording') {
              audioReceived = false;
            }
          } catch {
            audioReceived = true;
          }
        } else {
          audioReceived = true;
        }
      });
    });

    await routeFixtures(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const mediaDisplay = page.locator('[data-testid="media-display"]');
    const img = page.locator('[data-testid="media-image"]');

    // First attempt - should fail
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    await page.waitForTimeout(1000);
    await expect(mediaDisplay).toContainText(/unavailable|something went wrong|error|try again/i);

    // Wait for error state to reset (3 seconds)
    await page.waitForTimeout(3500);

    // Second attempt - should succeed
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    await expect(img).toBeVisible({ timeout: 10000 });
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    expect(attemptCount).toBe(2);
    expect(commandProcessed).toBe(true);
  });

  test('handles connection drop mid-recording and resumes gracefully', async ({ page }) => {
    let connectionCount = 0;
    let commandProcessed = false;

    await page.addInitScript(getWebSocketMockScript());

    await page.routeWebSocket('**/ws/audio', async ws => {
      connectionCount++;
      const currentConnection = connectionCount;
      let audioChunkCount = 0;
      let audioReceived = false;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            } else if (parsed.type === 'stop_recording' && audioReceived && currentConnection > 1) {
              ws.send(JSON.stringify({ type: 'transcript', text: 'show me a cat' }));
              setTimeout(() => {
                ws.send(JSON.stringify({
                  type: 'media',
                  subject: 'cat',
                  photo_url: '/fixtures/mock-cat-photo.jpg',
                  audio_url: '/fixtures/mock-cat-audio.mp3',
                }));
                commandProcessed = true;
              }, 50);
            } else if (parsed.type === 'start_recording') {
              audioReceived = false;
            }
          } catch {
            // Not valid JSON
          }
        } else {
          audioChunkCount++;
          audioReceived = true;
          if (currentConnection === 1 && audioChunkCount >= 2) {
            ws.close();
          }
        }
      });
    });

    await routeFixtures(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const mediaDisplay = page.locator('[data-testid="media-display"]');
    const img = page.locator('[data-testid="media-image"]');

    // First attempt - connection will close mid-recording
    await micButton.click();
    await page.waitForTimeout(1000);
    await page.waitForTimeout(1000);

    await expect(mediaDisplay).toContainText(/connection|lost|error|try again/i);

    // Wait for error state to reset (3 seconds)
    await page.waitForTimeout(3500);

    // Second attempt - should work after reconnection
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    await expect(img).toBeVisible({ timeout: 10000 });
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    expect(connectionCount).toBeGreaterThanOrEqual(2);
    expect(commandProcessed).toBe(true);
  });
});
