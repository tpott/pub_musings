import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('astro.config.mjs', () => {
  const configPath = resolve(__dirname, '../astro.config.mjs');
  const configContent = readFileSync(configPath, 'utf-8');

  it('has proxy configuration for /api', () => {
    // Check that the config proxies /api to the backend
    expect(configContent).toContain("'/api'");
    expect(configContent).toContain('proxy');
    expect(configContent).toContain('localhost:8080');
  });

  it('has proxy configuration for /data/media', () => {
    // Check that the config proxies /data/media to the backend
    expect(configContent).toContain("'/data/media'");
  });

  it('targets the Go backend at port 8080', () => {
    // Ensure the proxy target is the Go backend
    expect(configContent).toContain('http://localhost:8080');
  });
});
