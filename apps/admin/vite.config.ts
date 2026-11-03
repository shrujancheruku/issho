import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

// The API allows browser calls only from origins in its ALLOWED_ORIGINS,
// so the dev server keeps a fixed port: add http://localhost:5173 there.
export default defineConfig({
  plugins: [react()],
  server: { port: 5173, strictPort: true },
  // Page tests render into jsdom. src/lib.ts needs env vars and a Supabase
  // client, so they replace it with the fake API (src/test/fakeApi.ts).
  test: { environment: 'jsdom', setupFiles: ['src/test/setup.ts'] },
});
