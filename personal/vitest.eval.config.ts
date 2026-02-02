import { defineConfig } from 'vitest/config';
import { loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  return {
    test: {
      env,
      include: ['tests/content/eval/**/*.test.ts'],
      globals: true,
      testTimeout: 60_000,
      pool: 'forks',
      poolOptions: {
        forks: { singleFork: true },
      },
    },
  };
});
