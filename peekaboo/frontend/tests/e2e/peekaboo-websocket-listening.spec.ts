import { test, expect } from '@playwright/test';
import { getWebSocketMockScript } from '../helpers/mock-media-recorder';
import { routeFixtures } from '../helpers/e2e-helpers';

test.describe('WebSocket continuous listening', () => {
  test('user can issue multiple commands via WebSocket mode', async ({ page }) => {
    let commandCount = 0;
    const animals = ['cat', 'dog'];

    await page.addInitScript(getWebSocketMockScript());

    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioReceived = false;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'audio_data') {
              audioReceived = true;
            } else if (parsed.type === 'stop_recording' && audioReceived) {
              const animal = animals[commandCount % animals.length];
              commandCount++;

              ws.send(JSON.stringify({ type: 'transcript', text: `show me a ${animal}` }));

              setTimeout(() => {
                ws.send(JSON.stringify({
                  type: 'media',
                  subject: animal,
                  photo_url: `/fixtures/mock-${animal}-photo.jpg`,
                  audio_url: `/fixtures/mock-${animal}-audio.mp3`,
                }));
              }, 50);

              audioReceived = false;
            } else if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            }
          } catch {
            // Ignore non-JSON messages
          }
        }
      });
    });

    await routeFixtures(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const img = page.locator('[data-testid="media-image"]');

    // First command: cat
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    await expect(img).toBeVisible({ timeout: 10000 });
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    // Second command: dog
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    await expect(img).toHaveAttribute('src', '/fixtures/mock-dog-photo.jpg', { timeout: 10000 });

    expect(commandCount).toBe(2);
  });

  test('continuous listening - mic stays active after media display', async ({ page }) => {
    let commandCount = 0;
    const animals = ['cat', 'dog'];

    await page.addInitScript(getWebSocketMockScript());

    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioChunkCount = 0;
      const audioThreshold = 4;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'audio_data') {
              audioChunkCount++;
              if (audioChunkCount >= audioThreshold) {
                const animal = animals[commandCount % animals.length];
                commandCount++;

                ws.send(JSON.stringify({ type: 'transcript', text: `show me a ${animal}` }));

                setTimeout(() => {
                  ws.send(JSON.stringify({
                    type: 'media',
                    subject: animal,
                    photo_url: `/fixtures/mock-${animal}-photo.jpg`,
                    audio_url: `/fixtures/mock-${animal}-audio.mp3`,
                  }));
                }, 50);

                audioChunkCount = 0;
              }
            } else if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            } else if (parsed.type === 'start_recording') {
              audioChunkCount = 0;
            }
          } catch {
            // Ignore non-JSON messages
          }
        }
      });
    });

    await routeFixtures(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const img = page.locator('[data-testid="media-image"]');

    // Click mic to start recording
    await micButton.click();

    // Wait for first command (cat)
    await expect(img).toBeVisible({ timeout: 10000 });
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    // Mic should still be recording
    await expect(micButton).toHaveAttribute('aria-pressed', 'true');
    await expect(micButton).toHaveClass(/recording/);

    // Wait for second command (dog) - no click needed
    await expect(img).toHaveAttribute('src', '/fixtures/mock-dog-photo.jpg', { timeout: 10000 });

    expect(commandCount).toBe(2);

    await expect(micButton).toHaveAttribute('aria-pressed', 'true');

    // Stop recording
    await micButton.click();
    await expect(micButton).toHaveAttribute('aria-pressed', 'false');
  });

  test('transcript display shows recognized speech', async ({ page }) => {
    await page.addInitScript(getWebSocketMockScript());

    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioReceived = false;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'audio_data') {
              audioReceived = true;
            } else if (parsed.type === 'stop_recording' && audioReceived) {
              ws.send(JSON.stringify({ type: 'transcript', text: 'show me a cat' }));

              setTimeout(() => {
                ws.send(JSON.stringify({
                  type: 'media',
                  subject: 'cat',
                  photo_url: '/fixtures/mock-cat-photo.jpg',
                  audio_url: '/fixtures/mock-cat-audio.mp3',
                }));
              }, 50);

              audioReceived = false;
            } else if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            }
          } catch {
            // Ignore non-JSON messages
          }
        }
      });
    });

    await routeFixtures(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();
    await expect(page.locator('[data-testid="transcript-display"]')).toBeAttached();

    const micButton = page.locator('[data-testid="mic-button"]');
    const transcriptDisplay = page.locator('[data-testid="transcript-display"]');

    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    await expect(transcriptDisplay).toContainText('show me a cat');

    const transcriptEntry = transcriptDisplay.locator('.transcript-entry');
    await expect(transcriptEntry).toBeVisible();
    await expect(transcriptEntry).toContainText('"show me a cat"');
  });

  test('transcript display accumulates multiple commands', async ({ page }) => {
    let commandCount = 0;
    const transcripts = ['show me a cat', 'show me a dog'];

    await page.addInitScript(getWebSocketMockScript());

    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioReceived = false;

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'audio_data') {
              audioReceived = true;
            } else if (parsed.type === 'stop_recording' && audioReceived) {
              const transcript = transcripts[commandCount % transcripts.length];
              const animal = commandCount === 0 ? 'cat' : 'dog';
              commandCount++;

              ws.send(JSON.stringify({ type: 'transcript', text: transcript }));

              setTimeout(() => {
                ws.send(JSON.stringify({
                  type: 'media',
                  subject: animal,
                  photo_url: `/fixtures/mock-${animal}-photo.jpg`,
                  audio_url: `/fixtures/mock-${animal}-audio.mp3`,
                }));
              }, 50);

              audioReceived = false;
            } else if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            }
          } catch {
            // Ignore non-JSON messages
          }
        }
      });
    });

    await routeFixtures(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const transcriptDisplay = page.locator('[data-testid="transcript-display"]');
    const img = page.locator('[data-testid="media-image"]');

    // First command: cat
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    await expect(img).toBeVisible({ timeout: 10000 });
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    // Second command: dog
    await micButton.click();
    await page.waitForTimeout(600);
    await micButton.click();

    await expect(img).toHaveAttribute('src', '/fixtures/mock-dog-photo.jpg', { timeout: 10000 });

    // Both transcripts visible
    const transcriptEntries = transcriptDisplay.locator('.transcript-entry');
    await expect(transcriptEntries).toHaveCount(2);
    await expect(transcriptEntries.nth(0)).toContainText('"show me a cat"');
    await expect(transcriptEntries.nth(1)).toContainText('"show me a dog"');
  });

  test('sequential voice commands - two utterances in one session with gap', async ({ page }) => {
    let commandCount = 0;
    const commands = [
      { text: 'show me a cat', subject: 'cat' },
      { text: 'show me a dog', subject: 'dog' },
    ];

    await page.addInitScript(getWebSocketMockScript());

    await page.routeWebSocket('**/ws/audio', async ws => {
      let audioChunkCount = 0;
      const audioThreshold = 4;
      let processingRound = 0; // even = real utterance, odd = silence gap

      ws.onMessage(message => {
        if (typeof message === 'string') {
          try {
            const parsed = JSON.parse(message);
            if (parsed.type === 'audio_data') {
              audioChunkCount++;
              if (audioChunkCount >= audioThreshold) {
                if (processingRound % 2 === 0 && commandCount < commands.length) {
                  // Real utterance
                  const cmd = commands[commandCount];
                  commandCount++;

                  ws.send(JSON.stringify({ type: 'transcript', text: cmd.text }));

                  setTimeout(() => {
                    ws.send(JSON.stringify({
                      type: 'media',
                      subject: cmd.subject,
                      photo_url: `/fixtures/mock-${cmd.subject}-photo.jpg`,
                      audio_url: `/fixtures/mock-${cmd.subject}-audio.mp3`,
                    }));
                  }, 50);
                } else {
                  // Silence gap
                  ws.send(JSON.stringify({ type: 'error', message: 'No speech detected. Please try again.' }));
                }

                processingRound++;
                audioChunkCount = 0;
              }
            } else if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong' }));
            } else if (parsed.type === 'start_recording') {
              audioChunkCount = 0;
              processingRound = 0;
            }
          } catch {
            // Ignore non-JSON messages
          }
        }
      });
    });

    await routeFixtures(page);

    await page.goto('/');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const img = page.locator('[data-testid="media-image"]');
    const transcriptDisplay = page.locator('[data-testid="transcript-display"]');

    // Click mic once to start - this is the ONLY click
    await micButton.click();

    // Wait for first command (cat)
    await expect(img).toBeVisible({ timeout: 15000 });
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    await expect(micButton).toHaveAttribute('aria-pressed', 'true');

    // Wait for silence gap, then second command (dog)
    await expect(img).toHaveAttribute('src', '/fixtures/mock-dog-photo.jpg', { timeout: 15000 });

    expect(commandCount).toBe(2);

    // Both transcripts visible
    const transcriptEntries = transcriptDisplay.locator('.transcript-entry');
    await expect(transcriptEntries).toHaveCount(2);
    await expect(transcriptEntries.nth(0)).toContainText('"show me a cat"');
    await expect(transcriptEntries.nth(1)).toContainText('"show me a dog"');

    // Mic still recording
    await expect(micButton).toHaveAttribute('aria-pressed', 'true');
    await expect(micButton).toHaveClass(/recording/);

    // Stop recording
    await micButton.click();
    await expect(micButton).toHaveAttribute('aria-pressed', 'false');
  });
});
