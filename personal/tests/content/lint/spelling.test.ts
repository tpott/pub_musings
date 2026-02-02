import { describe, it } from 'vitest';
import { execSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { getAllPosts } from '../helpers/parse-posts';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const PROJECT_ROOT = path.resolve(__dirname, '../../..');

const posts = getAllPosts();

describe('Spelling', () => {
  it.each(posts.map(p => [p.slug, p]))('%s', (_slug, post) => {
    try {
      execSync(
        `./node_modules/.bin/cspell --no-progress --no-summary "${post.filePath}"`,
        { cwd: PROJECT_ROOT, encoding: 'utf-8', stdio: 'pipe' },
      );
    } catch (e: any) {
      const output = (e.stdout || '') + (e.stderr || '');
      throw new Error(`Spelling errors:\n${output}`);
    }
  });
});
