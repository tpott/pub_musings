import { defineConfig } from 'astro/config';
import { loadEnv } from 'vite';
import react from '@astrojs/react';

const { PUBLIC_SITE_URL } = loadEnv('', process.cwd(), '');

export default defineConfig({
  output: 'static',
  integrations: [react()],
  site: PUBLIC_SITE_URL,
  vite: {
    server: {
      watch: {
        ignored: ['**/.*.swp', '**/.*.swo']
      }
    }
  }
});
