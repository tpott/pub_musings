import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    include: ['tests/content/**/*.test.ts'],
    exclude: ['tests/content/eval/**'],
    globals: true,
  },
});
