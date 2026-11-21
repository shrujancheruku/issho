import { defineConfig, devices } from '@playwright/test';

// `make e2e` (scripts/e2e.sh) picks two free ports, builds the API with a
// version of its own, and sets these. Nothing here has a default, so running
// `playwright test` by hand fails loudly rather than finding another stack.
function env(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is not set; run the end-to-end tests with \`make e2e\``);
  return value;
}

const apiPort = env('E2E_API_PORT');
const webPort = env('E2E_WEB_PORT');
const apiBinary = env('E2E_API_BINARY');

export default defineConfig({
  testDir: './tests',
  // CI has no retries, with one exception: this one. A test that fails and then
  // passes is reported as flaky (the list and GitHub reporters both say so), so
  // the retry is never silent (docs/spec/quality.md, "What CI requires").
  retries: 1,
  forbidOnly: true,
  reporter: process.env.CI ? [['list'], ['github']] : 'list',
  use: {
    baseURL: `http://127.0.0.1:${webPort}`,
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      command: apiBinary,
      env: { API_ADDR: `127.0.0.1:${apiPort}` },
      // The API answers every path it doesn't serve with 404, which Playwright
      // doesn't take for ready, so wait for the port to accept connections.
      port: Number(apiPort),
      reuseExistingServer: false,
    },
    {
      // The admin's production build served as static files; its proxy sends
      // the API's paths to the API, so the browser makes no cross-origin call.
      command: `pnpm --filter @issho/admin exec vite preview --host 127.0.0.1 --port ${webPort} --strictPort`,
      env: { API_PROXY_TARGET: `http://127.0.0.1:${apiPort}` },
      url: `http://127.0.0.1:${webPort}/`,
      reuseExistingServer: false,
    },
  ],
});
