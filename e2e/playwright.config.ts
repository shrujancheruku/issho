import { defineConfig, devices } from '@playwright/test';

// `make e2e` (scripts/e2e.sh) starts the throwaway stack (Postgres, Auth, a mail
// catcher), picks free ports, builds the API for end-to-end with a version of
// its own, and sets these. Nothing here has a default, so running
// `playwright test` by hand fails loudly rather than finding another stack.
function env(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is not set; run the end-to-end tests with \`make e2e\``);
  return value;
}

const apiPort = env('E2E_API_PORT');
const webPort = env('E2E_WEB_PORT');
const appPort = env('E2E_APP_PORT');
const appDist = env('E2E_APP_DIST');
const apiBinary = env('E2E_API_BINARY');
const databaseUrl = env('E2E_DATABASE_URL');
const authUrl = env('E2E_AUTH_URL');

export default defineConfig({
  testDir: './tests',
  // CI has no retries, with one exception: this one. A test that fails and then
  // passes is reported as flaky (the list and GitHub reporters both say so), so
  // the retry is never silent (docs/spec/quality.md, "What CI requires").
  retries: 1,
  // A journey drives three browsers through a day of the app's life.
  // The journeys share one stack and one clock, so they run one at a time.
  workers: 1,
  timeout: 180_000,
  expect: { timeout: 10_000 },
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
      // The API as `make e2e` built it (a fake clock, the worker in-process, the
      // test-only RPCs), on the throwaway database and Auth.
      env: { API_ADDR: `127.0.0.1:${apiPort}`, DATABASE_URL: databaseUrl, SUPABASE_AUTH_URL: authUrl },
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
    {
      // The member app as deployed: the Expo server (apps/web/server.js) serving
      // the app's web export, forwarding the API's paths to the API.
      command: 'node server.js',
      cwd: '../apps/web',
      env: { PORT: appPort, DIST: appDist, API_ORIGIN: `http://127.0.0.1:${apiPort}` },
      url: `http://127.0.0.1:${appPort}/up`,
      reuseExistingServer: false,
    },
  ],
});
