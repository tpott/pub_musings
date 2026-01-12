import { join } from 'path';
import { existsSync, mkdirSync } from 'fs';

// Generate safe filename from job title
export function generateFilename(title: string): string {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  const safeTitle = title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 50);
  return `${safeTitle}-${timestamp}`;
}

// Ensure output directories exist
export function ensureDirectories(baseDir: string): void {
  const dataDir = join(baseDir, 'data');
  const harDir = join(dataDir, 'har');
  const jobsDir = join(dataDir, 'jobs');

  if (!existsSync(dataDir)) mkdirSync(dataDir);
  if (!existsSync(harDir)) mkdirSync(harDir);
  if (!existsSync(jobsDir)) mkdirSync(jobsDir);
}
