import { describe, it, expect } from 'vitest';
import { getAllPosts, getPublishedPosts } from '../helpers/parse-posts';

const allPosts = getAllPosts();
const publishedPosts = getPublishedPosts();

describe('Frontmatter', () => {
  describe.each(allPosts.map(p => [p.slug, p]))('%s', (_slug, post) => {
    it('has a title', () => {
      expect(post.frontmatter.title).toBeTruthy();
    });

    it('title is ≤ 80 characters', () => {
      expect(post.frontmatter.title.length).toBeLessThanOrEqual(80);
    });

    it('has a valid pubDate', () => {
      const date = new Date(post.frontmatter.pubDate);
      expect(date.toString()).not.toBe('Invalid Date');
    });
  });

  describe('published posts', () => {
    describe.each(publishedPosts.map(p => [p.slug, p]))('%s', (_slug, post) => {
      it('has a description', () => {
        expect(post.frontmatter.description).toBeTruthy();
      });

      it('has at least one tag', () => {
        expect(post.frontmatter.tags).toBeDefined();
        expect(post.frontmatter.tags!.length).toBeGreaterThanOrEqual(1);
      });
    });
  });
});
