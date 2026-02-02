import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import matter from 'gray-matter';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const BLOG_DIR = path.resolve(__dirname, '../../../src/content/blog');

export interface PostFrontmatter {
  title: string;
  description?: string;
  pubDate: Date;
  updatedDate?: Date;
  tags?: string[];
  draft?: boolean;
}

export interface Post {
  slug: string;
  filePath: string;
  frontmatter: PostFrontmatter;
  content: string;
  rawContent: string;
}

export function getAllPosts(): Post[] {
  const files = fs.readdirSync(BLOG_DIR).filter(f => f.endsWith('.md'));
  return files.map(file => {
    const filePath = path.join(BLOG_DIR, file);
    const raw = fs.readFileSync(filePath, 'utf-8');
    const { data, content } = matter(raw);
    return {
      slug: file.replace(/\.md$/, ''),
      filePath,
      frontmatter: data as PostFrontmatter,
      content,
      rawContent: raw,
    };
  });
}

export function getPublishedPosts(): Post[] {
  return getAllPosts().filter(p => !p.frontmatter.draft);
}

export function getDraftPosts(): Post[] {
  return getAllPosts().filter(p => p.frontmatter.draft);
}
