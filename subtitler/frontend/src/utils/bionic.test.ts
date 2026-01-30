import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
	toBionicSegments,
	toBionicHTML,
	renderBionicText,
	isBionicEnabled,
	getBionicFixation,
	setBionicEnabled,
	setBionicFixation,
	BIONIC_READING_KEY,
	BIONIC_FIXATION_KEY,
} from './bionic';

// Mock localStorage
const localStorageMock = (() => {
	let store: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => store[key] || null),
		setItem: vi.fn((key: string, value: string) => {
			store[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete store[key];
		}),
		clear: vi.fn(() => {
			store = {};
		}),
		_setStore: (newStore: Record<string, string>) => {
			store = newStore;
		},
		_getStore: () => store,
	};
})();

Object.defineProperty(globalThis, 'localStorage', {
	value: localStorageMock,
	writable: true,
});

describe('Bionic Reading utility', () => {
	beforeEach(() => {
		localStorageMock.clear();
		vi.clearAllMocks();
	});

	describe('toBionicSegments', () => {
		it('should return empty array for empty string', () => {
			expect(toBionicSegments('')).toEqual([]);
		});

		it('should return empty array for null/undefined', () => {
			expect(toBionicSegments(null as unknown as string)).toEqual([]);
			expect(toBionicSegments(undefined as unknown as string)).toEqual([]);
		});

		it('should not bold short words (less than minWordLength)', () => {
			const result = toBionicSegments('a is to me');
			// "a", "is", "to" are too short (< 3), "me" is exactly 2 chars
			expect(result).toEqual([
				{ text: 'a', bold: false },
				{ text: ' ', bold: false },
				{ text: 'is', bold: false },
				{ text: ' ', bold: false },
				{ text: 'to', bold: false },
				{ text: ' ', bold: false },
				{ text: 'me', bold: false },
			]);
		});

		it('should bold first portion of words meeting minWordLength', () => {
			const result = toBionicSegments('the cat');
			// "the" = 3 chars, 40% = 1.2, ceil = 2 chars bold
			// "cat" = 3 chars, 40% = 1.2, ceil = 2 chars bold
			expect(result).toEqual([
				{ text: 'th', bold: true },
				{ text: 'e', bold: false },
				{ text: ' ', bold: false },
				{ text: 'ca', bold: true },
				{ text: 't', bold: false },
			]);
		});

		it('should preserve whitespace', () => {
			const result = toBionicSegments('hello   world');
			expect(result).toContainEqual({ text: '   ', bold: false });
		});

		it('should preserve multiple types of whitespace', () => {
			const result = toBionicSegments('hello\t\nworld');
			expect(result).toContainEqual({ text: '\t\n', bold: false });
		});

		it('should handle punctuation attached to words', () => {
			const result = toBionicSegments('Hello, world!');
			// Punctuation should not be bolded
			// "Hello" = 5 chars, 5 * 0.4 = 2.0, ceil = 2 chars
			// "world" = 5 chars, 5 * 0.4 = 2.0, ceil = 2 chars
			expect(result).toEqual([
				{ text: 'He', bold: true },
				{ text: 'llo', bold: false },
				{ text: ',', bold: false },
				{ text: ' ', bold: false },
				{ text: 'wo', bold: true },
				{ text: 'rld', bold: false },
				{ text: '!', bold: false },
			]);
		});

		it('should handle leading punctuation', () => {
			const result = toBionicSegments('"Hello"');
			// "Hello" = 5 chars, 5 * 0.4 = 2.0, ceil = 2 chars
			expect(result).toEqual([
				{ text: '"', bold: false },
				{ text: 'He', bold: true },
				{ text: 'llo', bold: false },
				{ text: '"', bold: false },
			]);
		});

		it('should handle different fixation percentages', () => {
			// 30% fixation
			const result30 = toBionicSegments('hello', { fixationPercent: 0.3 });
			// 5 * 0.3 = 1.5, ceil = 2
			expect(result30[0]).toEqual({ text: 'he', bold: true });

			// 50% fixation
			const result50 = toBionicSegments('hello', { fixationPercent: 0.5 });
			// 5 * 0.5 = 2.5, ceil = 3
			expect(result50[0]).toEqual({ text: 'hel', bold: true });
		});

		it('should respect custom minWordLength', () => {
			const result = toBionicSegments('the cat sat', { minWordLength: 4 });
			// All words are 3 chars, so none should be bolded
			expect(result.every((s) => !s.bold)).toBe(true);
		});

		it('should handle Unicode characters', () => {
			const result = toBionicSegments('こんにちは');
			// 5 chars, 40% = 2 chars bold
			expect(result).toEqual([
				{ text: 'こん', bold: true },
				{ text: 'にちは', bold: false },
			]);
		});

		it('should handle Devanagari script', () => {
			const result = toBionicSegments('नमस्ते');
			// Devanagari characters with combining marks - the character count may be 6
			// Even if matching is complex, it should produce output
			expect(result.length).toBeGreaterThan(0);
			// Just verify it handles the input without crashing
			// Bolding behavior depends on how the regex matches combining characters
		});

		it('should handle mixed script text', () => {
			const result = toBionicSegments('Hello 世界');
			// "Hello" = 5 chars, 5 * 0.4 = 2.0, ceil = 2 chars
			expect(result).toContainEqual({ text: 'He', bold: true });
			expect(result.some((s) => s.text.includes('世'))).toBe(true);
		});

		it('should handle numbers in words', () => {
			const result = toBionicSegments('test123');
			// "test123" = 7 chars, 40% = 2.8, ceil = 3
			expect(result[0]).toEqual({ text: 'tes', bold: true });
		});

		it('should handle pure punctuation', () => {
			const result = toBionicSegments('...');
			expect(result).toEqual([{ text: '...', bold: false }]);
		});

		it('should handle hyphenated words', () => {
			const result = toBionicSegments('well-known');
			// The hyphen creates separate word segments
			expect(result.length).toBeGreaterThan(0);
		});

		it('should ensure at least 1 character is bolded for eligible words', () => {
			const result = toBionicSegments('cat', { fixationPercent: 0.1 });
			// 3 * 0.1 = 0.3, ceil = 1 (minimum)
			expect(result[0]).toEqual({ text: 'c', bold: true });
		});
	});

	describe('toBionicHTML', () => {
		it('should return empty string for empty input', () => {
			expect(toBionicHTML('')).toBe('');
		});

		it('should wrap bold segments in <strong> tags', () => {
			const result = toBionicHTML('hello');
			// "hello" = 5 chars, 5 * 0.4 = 2.0, ceil = 2 chars
			expect(result).toBe('<strong>he</strong>llo');
		});

		it('should escape HTML special characters', () => {
			const result = toBionicHTML('<script>alert("xss")</script>');
			expect(result).not.toContain('<script>');
			expect(result).toContain('&lt;script&gt;');
		});

		it('should escape ampersands', () => {
			const result = toBionicHTML('rock & roll');
			expect(result).toContain('&amp;');
		});

		it('should handle quotes', () => {
			const result = toBionicHTML('"hello"');
			expect(result).toContain('&quot;');
		});

		it('should preserve whitespace in output', () => {
			const result = toBionicHTML('hello   world');
			expect(result).toContain('   ');
		});

		it('should produce valid HTML structure', () => {
			const result = toBionicHTML('The quick brown fox');
			// All <strong> tags should be properly closed
			const openTags = (result.match(/<strong>/g) || []).length;
			const closeTags = (result.match(/<\/strong>/g) || []).length;
			expect(openTags).toBe(closeTags);
		});
	});

	describe('localStorage functions', () => {
		describe('isBionicEnabled', () => {
			it('should return false when not set', () => {
				expect(isBionicEnabled()).toBe(false);
			});

			it('should return true when set to "true"', () => {
				localStorageMock._setStore({ [BIONIC_READING_KEY]: 'true' });
				expect(isBionicEnabled()).toBe(true);
			});

			it('should return false when set to "false"', () => {
				localStorageMock._setStore({ [BIONIC_READING_KEY]: 'false' });
				expect(isBionicEnabled()).toBe(false);
			});

			it('should return false for invalid values', () => {
				localStorageMock._setStore({ [BIONIC_READING_KEY]: 'yes' });
				expect(isBionicEnabled()).toBe(false);
			});
		});

		describe('getBionicFixation', () => {
			it('should return default (0.4) when not set', () => {
				expect(getBionicFixation()).toBe(0.4);
			});

			it('should return stored value when valid', () => {
				localStorageMock._setStore({ [BIONIC_FIXATION_KEY]: '0.3' });
				expect(getBionicFixation()).toBe(0.3);
			});

			it('should return default for invalid values', () => {
				localStorageMock._setStore({ [BIONIC_FIXATION_KEY]: 'invalid' });
				expect(getBionicFixation()).toBe(0.4);
			});

			it('should return default for out-of-range values', () => {
				localStorageMock._setStore({ [BIONIC_FIXATION_KEY]: '0.1' });
				expect(getBionicFixation()).toBe(0.4);

				localStorageMock._setStore({ [BIONIC_FIXATION_KEY]: '0.9' });
				expect(getBionicFixation()).toBe(0.4);
			});
		});

		describe('setBionicEnabled', () => {
			it('should store "true" when enabled', () => {
				setBionicEnabled(true);
				expect(localStorageMock.setItem).toHaveBeenCalledWith(
					BIONIC_READING_KEY,
					'true'
				);
			});

			it('should store "false" when disabled', () => {
				setBionicEnabled(false);
				expect(localStorageMock.setItem).toHaveBeenCalledWith(
					BIONIC_READING_KEY,
					'false'
				);
			});
		});

		describe('setBionicFixation', () => {
			it('should store the value', () => {
				setBionicFixation(0.3);
				expect(localStorageMock.setItem).toHaveBeenCalledWith(
					BIONIC_FIXATION_KEY,
					'0.3'
				);
			});

			it('should clamp values below minimum', () => {
				setBionicFixation(0.1);
				expect(localStorageMock.setItem).toHaveBeenCalledWith(
					BIONIC_FIXATION_KEY,
					'0.3'
				);
			});

			it('should clamp values above maximum', () => {
				setBionicFixation(0.9);
				expect(localStorageMock.setItem).toHaveBeenCalledWith(
					BIONIC_FIXATION_KEY,
					'0.5'
				);
			});
		});
	});

	describe('renderBionicText', () => {
		it('should return escaped HTML when bionic is disabled', () => {
			localStorageMock._setStore({ [BIONIC_READING_KEY]: 'false' });
			const result = renderBionicText('<script>');
			expect(result).toBe('&lt;script&gt;');
			expect(result).not.toContain('<strong>');
		});

		it('should return bionic HTML when enabled', () => {
			localStorageMock._setStore({ [BIONIC_READING_KEY]: 'true' });
			const result = renderBionicText('hello');
			expect(result).toContain('<strong>');
		});

		it('should use localStorage fixation when enabled', () => {
			localStorageMock._setStore({
				[BIONIC_READING_KEY]: 'true',
				[BIONIC_FIXATION_KEY]: '0.5',
			});
			const result = renderBionicText('hello');
			// 50% of 5 = 2.5, ceil = 3
			expect(result).toBe('<strong>hel</strong>lo');
		});

		it('should respect explicit enabled option', () => {
			localStorageMock._setStore({ [BIONIC_READING_KEY]: 'false' });
			const result = renderBionicText('hello', { enabled: true });
			expect(result).toContain('<strong>');
		});

		it('should respect explicit disabled option', () => {
			localStorageMock._setStore({ [BIONIC_READING_KEY]: 'true' });
			const result = renderBionicText('hello', { enabled: false });
			expect(result).not.toContain('<strong>');
		});

		it('should respect explicit fixationPercent option', () => {
			localStorageMock._setStore({
				[BIONIC_READING_KEY]: 'true',
				[BIONIC_FIXATION_KEY]: '0.3',
			});
			const result = renderBionicText('hello', { fixationPercent: 0.5 });
			// Explicit option should override localStorage
			// 50% of 5 = 2.5, ceil = 3
			expect(result).toBe('<strong>hel</strong>lo');
		});
	});

	describe('edge cases', () => {
		it('should handle very long words', () => {
			const longWord = 'supercalifragilisticexpialidocious';
			const result = toBionicSegments(longWord);
			const boldPart = result.find((s) => s.bold);
			expect(boldPart).toBeDefined();
			// 34 chars * 0.4 = 13.6, ceil = 14
			expect(boldPart?.text.length).toBe(14);
		});

		it('should handle single character input', () => {
			const result = toBionicSegments('a');
			expect(result).toEqual([{ text: 'a', bold: false }]);
		});

		it('should handle emoji', () => {
			// Emoji are not letters, so they should not be bolded
			const result = toBionicSegments('hello 👋 world');
			expect(result.some((s) => s.text === '👋')).toBe(true);
		});

		it('should handle mixed content', () => {
			const result = toBionicSegments('Price: $100.00!');
			// Should not crash, punctuation handled gracefully
			expect(result.length).toBeGreaterThan(0);
		});

		it('should handle contractions', () => {
			const result = toBionicSegments("don't");
			// Apostrophe creates word boundary
			expect(result.length).toBeGreaterThan(0);
		});

		it('should handle newlines in text', () => {
			const result = toBionicSegments('line1\nline2');
			expect(result.some((s) => s.text === '\n')).toBe(true);
		});
	});
});
