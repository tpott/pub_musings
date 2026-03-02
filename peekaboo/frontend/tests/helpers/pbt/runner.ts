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
const MIC_SELECTOR = '[data-testid="mic-button"]';

/** Create a PBT runner. Reads PIPER_SERVER_URL from env for say(). */
export async function createPBT(
  page: Page,
  opts?: { timeout?: number },
): Promise<PBTRunner> {
  const config: PBTConfig = {
    piperUrl: process.env.PIPER_SERVER_URL || '',
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
      throw new Error('PIPER_SERVER_URL not set — use sayFixture() instead.');
    }
    const audio = await synthesizeAndChunk(text, this.config.piperUrl);
    await this.injectAndPlay(audio.chunks);
  }

  /** Use a pre-recorded .webm fixture file. */
  async sayFixture(filename: string): Promise<void> {
    await this.injectAndPlay(fixtureToChunks(filename));
  }

  /** Assert media image visible with src containing subject. */
  async assertMedia(subject: string): Promise<void> {
    const img = this.page.locator('[data-testid="media-image"]');
    await expect(img).toBeVisible({ timeout: this.config.timeout });
    const src = await img.getAttribute('src');
    expect(src).toContain(`/data/media/${subject}/`);
  }

  /** Assert no media image visible. */
  async assertNoMedia(): Promise<void> {
    await expect(
      this.page.locator('[data-testid="media-image"]'),
    ).not.toBeVisible({ timeout: this.config.timeout });
  }

  /** Assert tts_audio WS message received. */
  async assertTTS(contains?: string): Promise<void> {
    await this.pollWSMessage('tts_audio', 'text', contains);
  }

  /** Assert no tts_audio WS message in buffer. Polls to catch late arrivals. */
  async assertNoTTS(): Promise<void> {
    // Poll over 2s to catch TTS arriving at any point, failing immediately
    // if a tts_audio message appears instead of only checking at the end
    await expect
      .poll(
        () => this.getWSMessages().then(
          (msgs) => msgs.filter((m: any) => m.type === 'tts_audio').length,
        ),
        { timeout: 2000, intervals: [200, 400, 400, 500, 500] },
      )
      .toBe(0);
  }

  /** Assert transcript display contains text. */
  async assertTranscript(text: string): Promise<void> {
    const el = this.page.locator('[data-testid="transcript-display"]');
    await expect(el).toContainText(text, { timeout: this.config.timeout });
  }

  /** Assert error WS message received. */
  async assertError(text?: string): Promise<void> {
    await this.pollWSMessage('error', 'message', text);
  }

  /** Reset captured WS message buffer (for multi-command tests). */
  async clear(): Promise<void> {
    await this.page.evaluate(() => (window as any).__PBT_CLEAR_MESSAGES__?.());
  }

  // --- Private helpers ---

  /** Poll WS messages for a specific type, optionally matching a field. */
  private async pollWSMessage(
    type: string, field: string, contains?: string,
  ): Promise<void> {
    await expect
      .poll(
        () => this.getWSMessages().then((msgs) => {
          const matched = msgs.filter((m: any) => m.type === type);
          if (matched.length === 0) return null;
          if (!contains) return true;
          return matched.some((m: any) =>
            m[field]?.toLowerCase().includes(contains.toLowerCase()),
          ) ? true : null;
        }),
        { timeout: this.config.timeout },
      )
      .toBeTruthy();
  }

  private async injectAndPlay(chunks: string[]): Promise<void> {
    this.sessions.push(chunks);
    if (!this.initialized) {
      await this.page.addInitScript(getScriptedRecorderScript());
      await this.page.addInitScript(getWSObserverScript());
      await this.page.addInitScript((sessions: string[][]) => {
        (window as any).__PBT_SESSIONS__ = sessions;
      }, this.sessions);
      await this.page.goto('/');
      await this.page.waitForLoadState('domcontentloaded');
      this.initialized = true;
    } else {
      await this.page.evaluate((newChunks: string[]) => {
        (window as any).__PBT_SESSIONS__.push(newChunks);
      }, chunks);
      // Stop active recorder before starting new session — wait for
      // WebSocket close handshake and backend state transition
      const mic = this.page.locator(MIC_SELECTOR);
      if ((await mic.getAttribute('aria-pressed')) === 'true') {
        await mic.click();
        await this.page.waitForTimeout(1000);
      }
    }
    const mic = this.page.locator(MIC_SELECTOR);
    await expect(mic).toBeVisible();
    await mic.click();
  }

  private async getWSMessages(): Promise<any[]> {
    return this.page.evaluate(() => (window as any).__PBT_WS_MESSAGES__ || []);
  }
}
