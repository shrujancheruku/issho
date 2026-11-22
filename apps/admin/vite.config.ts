import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  server: {
    // Dev runs on its own port, so the API's paths go through this proxy to
    // the local API instead of a cross-origin call the API does not allow.
    // API_PROXY_TARGET (read here, never bundled) points it at an API on
    // another port, e.g. one started with API_ADDR=127.0.0.1:0.
    port: 5173,
    proxy: {
      '/issho.v1.': process.env.API_PROXY_TARGET ?? 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
  },
});
