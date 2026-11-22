import { expect, test } from '@playwright/test';

// The member app as it is deployed: the Expo server serves the app's web export
// and forwards the API's paths, so the page calls the API on its own origin. The
// You tab shows the API's version (this run's build has a version of its own),
// which no part of the Expo server, its forwarder or the generated client could
// fake: all three are crossed to show it.
test("the member app's You tab shows the API's version through the Expo server", async ({ page }) => {
  const version = process.env.E2E_API_VERSION;
  const appPort = process.env.E2E_APP_PORT;
  if (!version || !appPort) throw new Error('E2E_API_VERSION and E2E_APP_PORT are not set; run this through `make e2e`');

  await page.goto(`http://127.0.0.1:${appPort}/you`);

  await expect(page.getByText(`API version ${version}`)).toBeVisible();
});
