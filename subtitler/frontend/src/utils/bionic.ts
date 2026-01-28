/**
 * Bionic Reading utility
 *
 * Transforms text by bolding the first portion of each word to create
 * "artificial fixation points" for reading.
 *
 * Note: Scientific studies have not found measurable improvements in reading
 * speed or comprehension for most people. This is offered as an optional
 * accessibility feature for those who find it subjectively helpful.
 */

import { escapeHtml } from './html';

export interface BionicOptions {
	/** Percentage of word to bold (0.3-0.5, default 0.4) */
	fixationPercent?: number;
	/** Minimum word length to apply bolding (default 3) */
	minWordLength?: number;
}

export interface BionicSegment {
	text: string;
	bold: boolean;
}

const DEFAULT_OPTIONS: Required<BionicOptions> = {
	fixationPercent: 0.4,
	minWordLength: 3,
};

/** localStorage key for bionic reading preference */
export const BIONIC_READING_KEY = 'subtitler:bionic-reading';

/** localStorage key for bionic fixation percentage */
export const BIONIC_FIXATION_KEY = 'subtitler:bionic-fixation';

/**
 * Check if bionic reading is enabled in localStorage
 */
export function isBionicEnabled(): boolean {
	if (typeof localStorage === 'undefined') return false;
	return localStorage.getItem(BIONIC_READING_KEY) === 'true';
}

/**
 * Get the bionic fixation percentage from localStorage
 */
export function getBionicFixation(): number {
	if (typeof localStorage === 'undefined') return DEFAULT_OPTIONS.fixationPercent;
	const value = localStorage.getItem(BIONIC_FIXATION_KEY);
	if (!value) return DEFAULT_OPTIONS.fixationPercent;
	const parsed = parseFloat(value);
	if (isNaN(parsed) || parsed < 0.3 || parsed > 0.5) {
		return DEFAULT_OPTIONS.fixationPercent;
	}
	return parsed;
}

/**
 * Set bionic reading preference in localStorage
 */
export function setBionicEnabled(enabled: boolean): void {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(BIONIC_READING_KEY, enabled ? 'true' : 'false');
}

/**
 * Set bionic fixation percentage in localStorage
 */
export function setBionicFixation(percent: number): void {
	if (typeof localStorage === 'undefined') return;
	// Clamp to valid range
	const clamped = Math.max(0.3, Math.min(0.5, percent));
	localStorage.setItem(BIONIC_FIXATION_KEY, clamped.toString());
}

/**
 * Convert a single word into bionic segments.
 * Returns an array with one or two segments depending on whether bolding applies.
 */
function wordToSegments(
	word: string,
	options: Required<BionicOptions>
): BionicSegment[] {
	// Skip short words
	if (word.length < options.minWordLength) {
		return [{ text: word, bold: false }];
	}

	// Calculate fixation length (at least 1 character)
	const fixationLength = Math.max(
		1,
		Math.ceil(word.length * options.fixationPercent)
	);

	const fixation = word.slice(0, fixationLength);
	const rest = word.slice(fixationLength);

	const segments: BionicSegment[] = [{ text: fixation, bold: true }];
	if (rest.length > 0) {
		segments.push({ text: rest, bold: false });
	}

	return segments;
}

/**
 * Converts plain text to an array of segments for rendering.
 * Each segment has { text: string, bold: boolean }.
 *
 * Preserves whitespace and handles punctuation attached to words.
 */
export function toBionicSegments(
	text: string,
	options?: BionicOptions
): BionicSegment[] {
	if (!text) return [];

	const opts: Required<BionicOptions> = {
		...DEFAULT_OPTIONS,
		...options,
	};

	const segments: BionicSegment[] = [];

	// Split by whitespace while preserving whitespace segments
	// This regex captures both word segments and whitespace segments
	const parts = text.split(/(\s+)/);

	for (const part of parts) {
		if (!part) continue;

		// If it's whitespace, add as non-bold
		if (/^\s+$/.test(part)) {
			segments.push({ text: part, bold: false });
			continue;
		}

		// For words, we need to handle punctuation carefully
		// Match leading punctuation, word content, trailing punctuation
		const match = part.match(/^([^\p{L}\p{N}]*)([\p{L}\p{N}]+)([^\p{L}\p{N}]*)$/u);

		if (match) {
			const [, leadingPunct, word, trailingPunct] = match;

			// Add leading punctuation as non-bold
			if (leadingPunct) {
				segments.push({ text: leadingPunct, bold: false });
			}

			// Process the word itself
			const wordSegments = wordToSegments(word, opts);
			segments.push(...wordSegments);

			// Add trailing punctuation as non-bold
			if (trailingPunct) {
				segments.push({ text: trailingPunct, bold: false });
			}
		} else {
			// If it doesn't match the word pattern (e.g., pure punctuation),
			// add as non-bold
			segments.push({ text: part, bold: false });
		}
	}

	return segments;
}


/**
 * Converts plain text to HTML with bionic reading formatting.
 * Returns HTML string with <strong> tags for fixation points.
 *
 * Text is escaped to prevent XSS attacks.
 */
export function toBionicHTML(text: string, options?: BionicOptions): string {
	const segments = toBionicSegments(text, options);

	return segments
		.map((s) =>
			s.bold
				? `<strong>${escapeHtml(s.text)}</strong>`
				: escapeHtml(s.text)
		)
		.join('');
}

/**
 * Renders text with bionic formatting if enabled, otherwise returns escaped HTML.
 * This is the main entry point for applying bionic reading to subtitles.
 *
 * @param text - The plain text to render
 * @param options - Optional bionic options (uses localStorage settings by default)
 * @returns HTML string safe for innerHTML
 */
export function renderBionicText(
	text: string,
	options?: BionicOptions & { enabled?: boolean }
): string {
	const enabled = options?.enabled ?? isBionicEnabled();

	if (!enabled) {
		return escapeHtml(text);
	}

	const fixation = options?.fixationPercent ?? getBionicFixation();
	return toBionicHTML(text, { ...options, fixationPercent: fixation });
}
