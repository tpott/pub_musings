import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import matter from 'gray-matter';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const BLOG_DIR = path.resolve(__dirname, '../../src/content/blog');

interface PostMeta {
  slug: string;
  title: string;
  draft: boolean;
}

function loadPosts(): PostMeta[] {
  const files = fs.readdirSync(BLOG_DIR).filter(f => f.endsWith('.md'));
  return files.map(file => {
    const raw = fs.readFileSync(path.join(BLOG_DIR, file), 'utf-8');
    const { data } = matter(raw);
    return {
      slug: file.replace(/\.md$/, ''),
      title: data.title,
      draft: data.draft ?? false,
    };
  });
}

const allPosts = loadPosts();
const published = allPosts.filter(p => !p.draft);
const drafts = allPosts.filter(p => p.draft);

test.describe('Blog index', () => {
  test('lists all published posts', async ({ page }) => {
    await page.goto('/blog');
    for (const post of published) {
      await expect(page.locator(`a[href="/blog/${post.slug}"]`)).toBeVisible();
    }
  });

  if (drafts.length > 0) {
    test('does not list draft posts', async ({ page }) => {
      await page.goto('/blog');
      for (const post of drafts) {
        await expect(page.locator(`a[href="/blog/${post.slug}"]`)).not.toBeVisible();
      }
    });
  }
});

test.describe('Published posts', () => {
  for (const post of published) {
    test.describe(post.slug, () => {
      test('renders with correct title', async ({ page }) => {
        await page.goto(`/blog/${post.slug}`);
        await expect(page.locator('h1')).toContainText(post.title);
      });

      test('shows date', async ({ page }) => {
        await page.goto(`/blog/${post.slug}`);
        await expect(page.locator('time')).toBeVisible();
      });

      test('has back link to blog index', async ({ page }) => {
        await page.goto(`/blog/${post.slug}`);
        await expect(page.locator('article footer a[href="/blog"]')).toBeVisible();
      });
    });
  }
});

if (drafts.length > 0) {
  test.describe('Draft posts', () => {
    for (const post of drafts) {
      test(`${post.slug} returns 404`, async ({ page }) => {
        const response = await page.goto(`/blog/${post.slug}`);
        expect(response?.status()).toBe(404);
      });
    }
  });
}
