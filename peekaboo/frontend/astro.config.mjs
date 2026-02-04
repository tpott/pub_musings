import { defineConfig } from 'astro/config';

export default defineConfig({
  server: {
    port: 4321,
  },
  vite: {
    server: {
      proxy: {
        // Proxy API requests to Go backend during development
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
        // Proxy media files to Go backend
        '/data/media': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
        // Proxy fixtures for e2e tests
        '/fixtures': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
  },
});
