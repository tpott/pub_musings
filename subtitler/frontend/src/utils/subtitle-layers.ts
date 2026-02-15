/**
 * Subtitle layer management: toggle between sentence-level and word-level display.
 *
 * Sentence mode (default): shows one subtitle per segment (existing behavior).
 * Word mode: karaoke-style display where individual words highlight as spoken.
 */

import type { TranscriptionSegment } from '../types/transcription';
import { escapeHtml } from './html';

export type SubtitleLayer = 'sentences' | 'words';

const LAYER_STORAGE_KEY = 'subtitler-layer';

/** Get saved layer preference from localStorage */
export function getSavedLayer(): SubtitleLayer {
  try {
    const saved = localStorage.getItem(LAYER_STORAGE_KEY);
    if (saved === 'words') return 'words';
  } catch {
    // localStorage unavailable
  }
  return 'sentences';
}

/** Save layer preference to localStorage */
export function saveLayer(layer: SubtitleLayer): void {
  try {
    localStorage.setItem(LAYER_STORAGE_KEY, layer);
  } catch {
    // localStorage unavailable
  }
}

/** Check whether any segment in the array has word-level data */
export function hasWordData(segments: TranscriptionSegment[]): boolean {
  return segments.some(s => s.words && s.words.length > 0);
}

/**
 * Render karaoke-style word highlighting for a segment at the given time.
 * Returns an HTML string with span elements for each word.
 */
export function renderKaraokeHTML(segment: TranscriptionSegment, currentTime: number): string {
  if (!segment.words || segment.words.length === 0) {
    return escapeHtml(segment.text.trim());
  }

  return segment.words.map(w => {
    const isCurrent = currentTime >= w.start && currentTime <= w.end;
    const isPast = currentTime > w.end;
    const cls = isCurrent ? 'word-current' : isPast ? 'word-past' : 'word-future';
    return `<span class="${cls}">${escapeHtml(w.text)}</span>`;
  }).join(' ');
}

/** State for the layer toggle UI */
export interface LayerToggleState {
  currentLayer: SubtitleLayer;
}

/** Create initial layer toggle state */
export function createLayerToggleState(): LayerToggleState {
  return { currentLayer: getSavedLayer() };
}

/** Update toggle button appearance to reflect the active layer */
export function updateLayerToggleUI(
  sentencesBtn: HTMLButtonElement,
  wordsBtn: HTMLButtonElement,
  layer: SubtitleLayer
): void {
  sentencesBtn.classList.toggle('active', layer === 'sentences');
  wordsBtn.classList.toggle('active', layer === 'words');
  sentencesBtn.setAttribute('aria-pressed', String(layer === 'sentences'));
  wordsBtn.setAttribute('aria-pressed', String(layer === 'words'));
}

/** Set up layer toggle event listeners */
export function setupLayerToggle(
  sentencesBtn: HTMLButtonElement,
  wordsBtn: HTMLButtonElement,
  layerState: LayerToggleState,
  onChange: (layer: SubtitleLayer) => void
): void {
  sentencesBtn.addEventListener('click', () => {
    if (layerState.currentLayer !== 'sentences') {
      layerState.currentLayer = 'sentences';
      saveLayer('sentences');
      updateLayerToggleUI(sentencesBtn, wordsBtn, 'sentences');
      onChange('sentences');
    }
  });

  wordsBtn.addEventListener('click', () => {
    if (layerState.currentLayer !== 'words') {
      layerState.currentLayer = 'words';
      saveLayer('words');
      updateLayerToggleUI(sentencesBtn, wordsBtn, 'words');
      onChange('words');
    }
  });

  updateLayerToggleUI(sentencesBtn, wordsBtn, layerState.currentLayer);
}
