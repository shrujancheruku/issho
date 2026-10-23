import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// The API allows browser calls only from origins in its ALLOWED_ORIGINS,
// so the dev server keeps a fixed port: add http://localhost:5173 there.
export default defineConfig({
  plugins: [react()],
  server: { port: 5173, strictPort: true },
});
