import { describe, it, expect } from 'vitest';
import { getAllPosts } from '../helpers/parse-posts';

const posts = getAllPosts();
const allSlugs = new Set(posts.map(p => p.slug));

const LINK_RE = /\[([^\]]*)\]\(([^)]+)\)/g;
const CHECK_EXTERNAL = process.env.CHECK_EXTERNAL_LINKS === 'true';

interface Link {
  text: string;
  url: string;
  line: number;
}

function extractLinks(content: string): Link[] {
  const links: Link[] = [];
  const lines = content.split('\n');
  for (let i = 0; i < lines.length; i++) {
    let match: RegExpExecArray | null;
    const re = new RegExp(LINK_RE.source, 'g');
    while ((match = re.exec(lines[i])) !== null) {
      links.push({ text: match[1], url: match[2], line: i + 1 });
    }
  }
  return links;
}

function isInternalLink(url: string): boolean {
  return url.startsWith('/') && !url.startsWith('//');
}

function isValidUrl(url: string): boolean {
  if (url.startsWith('#') || url.startsWith('/')) return true;
  try {
    new URL(url);
    return true;
  } catch {
    return false;
  }
}

describe('Links', () => {
  describe.each(posts.map(p => [p.slug, p]))('%s', (_slug, post) => {
    const links = extractLinks(post.content);

    if (links.length === 0) return;

    it('all links have non-empty text', () => {
      const empty = links.filter(l => l.text.trim() === '');
      expect(empty, `Empty link text at lines: ${empty.map(l => l.line).join(', ')}`).toHaveLength(0);
    });

    it('all links have valid URL format', () => {
      const invalid = links.filter(l => !isValidUrl(l.url));
      expect(invalid, `Invalid URLs: ${invalid.map(l => `line ${l.line}: ${l.url}`).join(', ')}`).toHaveLength(0);
    });

    it('internal links resolve to known slugs', () => {
      const internal = links.filter(l => isInternalLink(l.url));
      const broken = internal.filter(l => {
        const match = l.url.match(/^\/blog\/([^/#?]+)/);
        return match && !allSlugs.has(match[1]);
      });
      expect(broken, `Broken internal links: ${broken.map(l => `line ${l.line}: ${l.url}`).join(', ')}`).toHaveLength(0);
    });

    if (CHECK_EXTERNAL) {
      it('external links are reachable', async () => {
        const external = links.filter(l => !isInternalLink(l.url) && !l.url.startsWith('#'));
        for (const link of external) {
          const res = await fetch(link.url, { method: 'HEAD', redirect: 'follow' });
          expect(res.ok, `Unreachable: ${link.url} (line ${link.line})`).toBe(true);
        }
      }, 30_000);
    }
  });
});
