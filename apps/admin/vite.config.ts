import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  server: {
    // Dev runs on its own port, so the API's paths go through this proxy to
    // the local API instead of a cross-origin call the API does not allow.
    port: 5173,
    proxy: {
      '/issho.v1.': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
  },
});
