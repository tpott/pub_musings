import { describe, it, expect, beforeEach, vi } from 'vitest';
import type { TranscriptionSegment } from '../types/transcription';

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] || null),
    setItem: vi.fn((key: string, value: string) => { store[key] = value; }),
    removeItem: vi.fn((key: string) => { delete store[key]; }),
    clear: vi.fn(() => { store = {}; }),
    _setStore: (newStore: Record<string, string>) => { store = { ...newStore }; },
  };
})();

vi.stubGlobal('localStorage', localStorageMock);

import {
  getSavedLayer,
  saveLayer,
  hasWordData,
  renderKaraokeHTML,
  createLayerToggleState,
  updateLayerToggleUI,
  setupLayerToggle,
} from './subtitle-layers';

/** Create a mock button element for testing */
function createMockButton(): HTMLButtonElement {
  const classList = new Set<string>();
  const attrs: Record<string, string> = {};
  const listeners: Record<string, Array<() => void>> = {};

  return {
    classList: {
      add: vi.fn((cls: string) => classList.add(cls)),
      remove: vi.fn((cls: string) => classList.delete(cls)),
      contains: (cls: string) => classList.has(cls),
      toggle: vi.fn((cls: string, force?: boolean) => {
        if (force === true) { classList.add(cls); return true; }
        if (force === false) { classList.delete(cls); return false; }
        if (classList.has(cls)) { classList.delete(cls); return false; }
        classList.add(cls); return true;
      }),
    },
    setAttribute: vi.fn((key: string, value: string) => { attrs[key] = value; }),
    getAttribute: vi.fn((key: string) => attrs[key] || null),
    addEventListener: vi.fn((event: string, handler: () => void) => {
      if (!listeners[event]) listeners[event] = [];
      listeners[event].push(handler);
    }),
    click: () => { (listeners['click'] || []).forEach(h => h()); },
  } as unknown as HTMLButtonElement;
}

describe('Subtitle layers', () => {
  beforeEach(() => {
    localStorageMock.clear();
    vi.clearAllMocks();
  });

  describe('getSavedLayer', () => {
    it('returns sentences by default', () => {
      expect(getSavedLayer()).toBe('sentences');
    });

    it('returns words when saved as words', () => {
      localStorageMock._setStore({ 'subtitler-layer': 'words' });
      expect(getSavedLayer()).toBe('words');
    });

    it('returns sentences for invalid value', () => {
      localStorageMock._setStore({ 'subtitler-layer': 'invalid' });
      expect(getSavedLayer()).toBe('sentences');
    });
  });

  describe('saveLayer', () => {
    it('saves words to localStorage', () => {
      saveLayer('words');
      expect(localStorageMock.setItem).toHaveBeenCalledWith('subtitler-layer', 'words');
    });

    it('saves sentences to localStorage', () => {
      saveLayer('sentences');
      expect(localStorageMock.setItem).toHaveBeenCalledWith('subtitler-layer', 'sentences');
    });
  });

  describe('hasWordData', () => {
    it('returns false for empty segments', () => {
      expect(hasWordData([])).toBe(false);
    });

    it('returns false when no segments have words', () => {
      const segments: TranscriptionSegment[] = [
        { id: 0, start: 0, end: 2.5, text: 'Hello' },
        { id: 1, start: 3.0, end: 5.5, text: 'World' },
      ];
      expect(hasWordData(segments)).toBe(false);
    });

    it('returns true when at least one segment has words', () => {
      const segments: TranscriptionSegment[] = [
        { id: 0, start: 0, end: 2.5, text: 'Hello world', words: [
          { text: 'Hello', start: 0, end: 0.4 },
          { text: 'world', start: 0.5, end: 0.9 },
        ]},
        { id: 1, start: 3.0, end: 5.5, text: 'No words' },
      ];
      expect(hasWordData(segments)).toBe(true);
    });

    it('returns false when words array is empty', () => {
      const segments: TranscriptionSegment[] = [
        { id: 0, start: 0, end: 2.5, text: 'Hello', words: [] },
      ];
      expect(hasWordData(segments)).toBe(false);
    });
  });

  describe('renderKaraokeHTML', () => {
    const segment: TranscriptionSegment = {
      id: 0, start: 0, end: 4.5, text: 'Hello world test',
      words: [
        { text: 'Hello', start: 0.0, end: 0.4 },
        { text: 'world', start: 0.5, end: 0.9 },
        { text: 'test', start: 1.0, end: 1.5 },
      ],
    };

    it('highlights the current word', () => {
      const html = renderKaraokeHTML(segment, 0.2);
      expect(html).toContain('class="word-current">Hello</span>');
      expect(html).toContain('class="word-future">world</span>');
      expect(html).toContain('class="word-future">test</span>');
    });

    it('marks past words', () => {
      const html = renderKaraokeHTML(segment, 1.2);
      expect(html).toContain('class="word-past">Hello</span>');
      expect(html).toContain('class="word-past">world</span>');
      expect(html).toContain('class="word-current">test</span>');
    });

    it('all words are past after last word ends', () => {
      const html = renderKaraokeHTML(segment, 2.0);
      expect(html).toContain('class="word-past">Hello</span>');
      expect(html).toContain('class="word-past">world</span>');
      expect(html).toContain('class="word-past">test</span>');
    });

    it('all words are future before first word starts', () => {
      const earlySegment: TranscriptionSegment = {
        id: 0, start: 1.0, end: 3.0, text: 'A B',
        words: [
          { text: 'A', start: 1.0, end: 1.5 },
          { text: 'B', start: 2.0, end: 2.5 },
        ],
      };
      const html = renderKaraokeHTML(earlySegment, 0.5);
      expect(html).toContain('class="word-future">A</span>');
      expect(html).toContain('class="word-future">B</span>');
    });

    it('falls back to plain text for segment without words', () => {
      const noWordsSeg: TranscriptionSegment = { id: 0, start: 0, end: 2, text: 'Plain text' };
      const html = renderKaraokeHTML(noWordsSeg, 1.0);
      expect(html).toBe('Plain text');
      expect(html).not.toContain('span');
    });

    it('escapes HTML in word text', () => {
      const xssSeg: TranscriptionSegment = {
        id: 0, start: 0, end: 2, text: '<script>',
        words: [{ text: '<script>', start: 0, end: 1 }],
      };
      const html = renderKaraokeHTML(xssSeg, 0.5);
      expect(html).toContain('&lt;script&gt;');
      expect(html).not.toContain('<script>');
    });
  });

  describe('createLayerToggleState', () => {
    it('initializes with saved layer', () => {
      localStorageMock._setStore({ 'subtitler-layer': 'words' });
      const state = createLayerToggleState();
      expect(state.currentLayer).toBe('words');
    });

    it('initializes with sentences by default', () => {
      const state = createLayerToggleState();
      expect(state.currentLayer).toBe('sentences');
    });
  });

  describe('updateLayerToggleUI', () => {
    it('sets active class on sentences button', () => {
      const sentencesBtn = createMockButton();
      const wordsBtn = createMockButton();

      updateLayerToggleUI(sentencesBtn, wordsBtn, 'sentences');

      expect(sentencesBtn.classList.contains('active')).toBe(true);
      expect(wordsBtn.classList.contains('active')).toBe(false);
      expect(sentencesBtn.getAttribute('aria-pressed')).toBe('true');
      expect(wordsBtn.getAttribute('aria-pressed')).toBe('false');
    });

    it('sets active class on words button', () => {
      const sentencesBtn = createMockButton();
      const wordsBtn = createMockButton();

      updateLayerToggleUI(sentencesBtn, wordsBtn, 'words');

      expect(sentencesBtn.classList.contains('active')).toBe(false);
      expect(wordsBtn.classList.contains('active')).toBe(true);
      expect(sentencesBtn.getAttribute('aria-pressed')).toBe('false');
      expect(wordsBtn.getAttribute('aria-pressed')).toBe('true');
    });
  });

  describe('setupLayerToggle', () => {
    it('calls onChange when switching to words', () => {
      const sentencesBtn = createMockButton();
      const wordsBtn = createMockButton();
      const state = createLayerToggleState();
      const onChange = vi.fn();

      setupLayerToggle(sentencesBtn, wordsBtn, state, onChange);

      wordsBtn.click();
      expect(onChange).toHaveBeenCalledWith('words');
      expect(state.currentLayer).toBe('words');
    });

    it('calls onChange when switching to sentences', () => {
      localStorageMock._setStore({ 'subtitler-layer': 'words' });
      const sentencesBtn = createMockButton();
      const wordsBtn = createMockButton();
      const state = createLayerToggleState();
      const onChange = vi.fn();

      setupLayerToggle(sentencesBtn, wordsBtn, state, onChange);

      sentencesBtn.click();
      expect(onChange).toHaveBeenCalledWith('sentences');
      expect(state.currentLayer).toBe('sentences');
    });

    it('does not call onChange when clicking already active layer', () => {
      const sentencesBtn = createMockButton();
      const wordsBtn = createMockButton();
      const state = createLayerToggleState();
      const onChange = vi.fn();

      setupLayerToggle(sentencesBtn, wordsBtn, state, onChange);

      sentencesBtn.click();
      expect(onChange).not.toHaveBeenCalled();
    });

    it('saves layer preference to localStorage', () => {
      const sentencesBtn = createMockButton();
      const wordsBtn = createMockButton();
      const state = createLayerToggleState();

      setupLayerToggle(sentencesBtn, wordsBtn, state, vi.fn());

      wordsBtn.click();
      expect(localStorageMock.setItem).toHaveBeenCalledWith('subtitler-layer', 'words');
    });
  });
});
