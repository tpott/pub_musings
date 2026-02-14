/**
 * PBT Runner — orchestrates Piper TTS synthesis, ScriptedMediaRecorder
 * injection, and assertion methods for property-based voice tests.
 */
import { expect, type Page } from '@playwright/test';
import { synthesizeAndChunk, fixtureToChunks } from './audio-pipeline';
import { getScriptedRecorderScript } from './scripted-recorder';
import { getWSObserverScript } from './ws-observer';
import type { PBTConfig } from './types';

const DEFAULT_TIMEOUT = 30_000;

/**
 * Create a PBT runner for a Playwright page.
 * Reads PIPER_SERVER_URL from env (required for say(), not for sayFixture()).
 */
export async function createPBT(
  page: Page,
  opts?: { timeout?: number },
): Promise<PBTRunner> {
  const piperUrl = process.env.PIPER_SERVER_URL || '';
  const config: PBTConfig = {
    piperUrl,
    timeout: opts?.timeout ?? DEFAULT_TIMEOUT,
  };
  return new PBTRunner(page, config);
}

class PBTRunner {
  private page: Page;
  private config: PBTConfig;
  private initialized = false;
  private sessions: string[][] = [];

  constructor(page: Page, config: PBTConfig) {
    this.page = page;
    this.config = config;
  }

  /** Synthesize text via Piper, inject audio, click mic. */
  async say(text: string): Promise<void> {
    if (!this.config.piperUrl) {
      throw new Error(
        'PIPER_SERVER_URL not set — say() requires Piper. Use sayFixture() instead.',
      );
    }
    const audio = await synthesizeAndChunk(text, this.config.piperUrl);
    await this.injectAndPlay(audio.chunks);
  }

  /** Use a pre-recorded .webm fixture file. */
  async sayFixture(filename: string): Promise<void> {
    const chunks = fixtureToChunks(filename);
    await this.injectAndPlay(chunks);
  }

  /** Assert that a media image is visible with src containing subject. */
  async assertMedia(subject: string): Promise<void> {
    const img = this.page.locator('[data-testid="media-image"]');
    await expect(img).toBeVisible({ timeout: this.config.timeout });
    const src = await img.getAttribute('src');
    expect(src).toContain(`/data/media/${subject}/`);
  }

  /** Assert that no media image is visible. */
  async assertNoMedia(): Promise<void> {
    await expect(
      this.page.locator('[data-testid="media-image"]'),
    ).not.toBeVisible({ timeout: this.config.timeout });
  }

  /** Assert that a tts_audio WS message was received. */
  async assertTTS(contains?: string): Promise<void> {
    await expect
      .poll(
        () => this.getWSMessages().then((msgs) => {
          const tts = msgs.filter((m: any) => m.type === 'tts_audio');
          if (tts.length === 0) return null;
          if (contains) {
            return tts.some((m: any) =>
              m.text?.toLowerCase().includes(contains.toLowerCase()),
            )
              ? true
              : null;
          }
          return true;
        }),
        { timeout: this.config.timeout },
      )
      .toBeTruthy();
  }

  /** Assert that no tts_audio WS message is in the buffer. */
  async assertNoTTS(): Promise<void> {
    // Short wait to ensure no late TTS arrives
    await this.page.waitForTimeout(2000);
    const msgs = await this.getWSMessages();
    const tts = msgs.filter((m: any) => m.type === 'tts_audio');
    expect(tts).toHaveLength(0);
  }

  /** Assert that transcript display contains text. */
  async assertTranscript(text: string): Promise<void> {
    const el = this.page.locator('[data-testid="transcript-display"]');
    await expect(el).toContainText(text, { timeout: this.config.timeout });
  }

  /** Assert that an error WS message was received. */
  async assertError(text?: string): Promise<void> {
    await expect
      .poll(
        () => this.getWSMessages().then((msgs) => {
          const errors = msgs.filter((m: any) => m.type === 'error');
          if (errors.length === 0) return null;
          if (text) {
            return errors.some((m: any) =>
              m.message?.toLowerCase().includes(text.toLowerCase()),
            )
              ? true
              : null;
          }
          return true;
        }),
        { timeout: this.config.timeout },
      )
      .toBeTruthy();
  }

  /** Reset the captured WS message buffer (for multi-command tests). */
  async clear(): Promise<void> {
    await this.page.evaluate(() => {
      (window as any).__PBT_CLEAR_MESSAGES__?.();
    });
  }

  // --- Private ---

  private async injectAndPlay(chunks: string[]): Promise<void> {
    this.sessions.push(chunks);

    if (!this.initialized) {
      // First call: inject init scripts, set sessions, navigate
      await this.page.addInitScript(getScriptedRecorderScript());
      await this.page.addInitScript(getWSObserverScript());
      await this.page.addInitScript((sessions: string[][]) => {
        (window as any).__PBT_SESSIONS__ = sessions;
      }, this.sessions);
      await this.page.goto('/');
      await this.page.waitForLoadState('domcontentloaded');
      this.initialized = true;
    } else {
      // Subsequent call: push new session via evaluate
      await this.page.evaluate((newChunks: string[]) => {
        (window as any).__PBT_SESSIONS__.push(newChunks);
      }, chunks);

      // If recorder is currently active, stop it first
      const micButton = this.page.locator('[data-testid="mic-button"]');
      const isRecording = await micButton.getAttribute('aria-pressed');
      if (isRecording === 'true') {
        await micButton.click();
        // Brief wait for stop to complete
        await this.page.waitForTimeout(500);
      }
    }

    // Click mic to start recording
    const micButton = this.page.locator('[data-testid="mic-button"]');
    await expect(micButton).toBeVisible();
    await micButton.click();
  }

  private async getWSMessages(): Promise<any[]> {
    return this.page.evaluate(() => {
      return (window as any).__PBT_WS_MESSAGES__ || [];
    });
  }
}
