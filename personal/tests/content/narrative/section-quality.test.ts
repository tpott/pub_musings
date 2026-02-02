import { describe, it, expect } from 'vitest';
import { getAllPosts } from '../helpers/parse-posts';

const posts = getAllPosts();

const MIN_WORDS = 100;
const MAX_WORDS = 5_000;
const MIN_SECTION_WORDS = 5;
const HEADING_REQUIRED_ABOVE = 500;

function wordCount(text: string): number {
  return text.trim().split(/\s+/).filter(Boolean).length;
}

interface Section {
  heading: string | null;
  level: number;
  body: string;
}

function parseSections(content: string): Section[] {
  const sections: Section[] = [];
  const lines = content.split('\n');
  let currentSection: Section = { heading: null, level: 0, body: '' };

  for (const line of lines) {
    const match = line.match(/^(#{1,6})\s+(.+)$/);
    if (match) {
      sections.push(currentSection);
      currentSection = { heading: match[2], level: match[1].length, body: '' };
    } else {
      currentSection.body += line + '\n';
    }
  }
  sections.push(currentSection);
  return sections;
}

describe('Section quality', () => {
  describe.each(posts.map(p => [p.slug, p]))('%s', (_slug, post) => {
    const sections = parseSections(post.content);
    const headings = sections.filter(s => s.heading !== null);
    const totalWords = wordCount(post.content);

    it('has at least one heading (posts > 500 words)', () => {
      if (totalWords <= HEADING_REQUIRED_ABOVE) return;
      expect(headings.length).toBeGreaterThanOrEqual(1);
    });

    it('no empty sections (< 5 words)', () => {
      const empty = sections
        .filter(s => s.heading !== null && wordCount(s.body) < MIN_SECTION_WORDS);
      const names = empty.map(s => `"${s.heading}" (${wordCount(s.body)} words)`);
      expect(empty, `Empty sections: ${names.join(', ')}`).toHaveLength(0);
    });

    it(`word count between ${MIN_WORDS} and ${MAX_WORDS}`, () => {
      expect(totalWords).toBeGreaterThanOrEqual(MIN_WORDS);
      expect(totalWords).toBeLessThanOrEqual(MAX_WORDS);
    });

    it('has intro text before first heading', () => {
      if (headings.length === 0) return;
      const intro = sections[0];
      expect(
        wordCount(intro.body),
        'No intro text before the first heading',
      ).toBeGreaterThan(0);
    });
  });
});
