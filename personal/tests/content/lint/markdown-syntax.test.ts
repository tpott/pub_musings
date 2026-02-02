import { describe, it } from 'vitest';
import { lint } from 'markdownlint/sync';
import { getAllPosts } from '../helpers/parse-posts';

const posts = getAllPosts();

const config = {
  default: true,
  'line-length': false,      // MD013 — too noisy for blog prose
  'no-inline-html': false,   // MD033 — we use HTML comments and entities
  'first-line-heading': false, // MD041 — frontmatter comes first, not a heading
};

describe('Markdown syntax', () => {
  it.each(posts.map(p => [p.slug, p]))('%s', (_slug, post) => {
    const result = lint({
      strings: { [post.slug]: post.content },
      config,
    });

    const errors = result[post.slug] || [];
    if (errors.length > 0) {
      const messages = errors.map(
        (e: any) => `  line ${e.lineNumber}: ${e.ruleNames.join('/')} — ${e.ruleDescription}`,
      );
      throw new Error(`Markdown lint errors:\n${messages.join('\n')}`);
    }
  });
});
