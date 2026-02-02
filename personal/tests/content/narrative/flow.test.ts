import { describe, it, expect } from 'vitest';
import { getAllPosts, getPublishedPosts } from '../helpers/parse-posts';

const allPosts = getAllPosts();
const publishedPosts = getPublishedPosts();

const HEADING_RE = /^(#{1,6})\s+(.+)$/gm;

function extractHeadings(content: string): { level: number; text: string }[] {
  const headings: { level: number; text: string }[] = [];
  let match: RegExpExecArray | null;
  const re = new RegExp(HEADING_RE.source, 'gm');
  while ((match = re.exec(content)) !== null) {
    headings.push({ level: match[1].length, text: match[2] });
  }
  return headings;
}

function getLastSection(content: string): string {
  const lines = content.split('\n');
  let lastHeadingIdx = -1;
  for (let i = lines.length - 1; i >= 0; i--) {
    if (/^#{1,6}\s+/.test(lines[i])) {
      lastHeadingIdx = i;
      break;
    }
  }
  if (lastHeadingIdx === -1) return content;
  return lines.slice(lastHeadingIdx + 1).join('\n');
}

describe('Narrative flow', () => {
  describe.each(allPosts.map(p => [p.slug, p]))('%s', (_slug, post) => {
    const headings = extractHeadings(post.content);

    it('heading hierarchy does not skip levels', () => {
      for (let i = 1; i < headings.length; i++) {
        const jump = headings[i].level - headings[i - 1].level;
        expect(
          jump,
          `Heading "${headings[i].text}" (h${headings[i].level}) skips from h${headings[i - 1].level}`,
        ).toBeLessThanOrEqual(1);
      }
    });

    it('description keywords appear in body', () => {
      if (!post.frontmatter.description) return;

      const stopWords = new Set([
        'a', 'an', 'the', 'and', 'or', 'but', 'in', 'on', 'at', 'to', 'for',
        'of', 'with', 'by', 'from', 'is', 'was', 'are', 'were', 'be', 'been',
        'being', 'have', 'has', 'had', 'do', 'does', 'did', 'will', 'would',
        'could', 'should', 'may', 'might', 'can', 'this', 'that', 'these',
        'those', 'i', 'my', 'what', 'how', 'they', 'them', 'it', 'its',
        'not', 'no', 'so', 'if', 'about', 'up', 'out', 'just', 'than',
      ]);

      const keywords = post.frontmatter.description
        .toLowerCase()
        .replace(/[^\w\s]/g, '')
        .split(/\s+/)
        .filter(w => w.length > 2 && !stopWords.has(w));

      const bodyLower = post.content.toLowerCase();
      const found = keywords.filter(kw => bodyLower.includes(kw));
      const ratio = keywords.length > 0 ? found.length / keywords.length : 1;

      expect(
        ratio,
        `Only ${found.length}/${keywords.length} description keywords found in body`,
      ).toBeGreaterThanOrEqual(0.3);
    });

    it('final section has substance (not abrupt)', () => {
      const lastSection = getLastSection(post.content);
      const words = lastSection.trim().split(/\s+/).filter(Boolean).length;
      expect(words, 'Final section is too short').toBeGreaterThanOrEqual(3);
    });
  });

  describe('published posts have no SUGGESTED comments', () => {
    it.each(publishedPosts.map(p => [p.slug, p]))('%s', (_slug, post) => {
      expect(
        post.rawContent.includes('<!-- SUGGESTED'),
        'Published post contains <!-- SUGGESTED comments',
      ).toBe(false);
    });
  });
});
