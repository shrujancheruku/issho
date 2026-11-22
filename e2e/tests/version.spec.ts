import { expect, test } from '@playwright/test';

// The first journey is the smallest one that crosses every seam: a browser
// loads the web, the web calls the API through the generated client, and the
// page shows what the API answered. The API is built with a version of its own
// (scripts/e2e.sh), so a page that printed a fixed string would fail.
test('the web page loads and shows the API\'s version', async ({ page }) => {
  const version = process.env.E2E_API_VERSION;
  if (!version) throw new Error('E2E_API_VERSION is not set; run this through `make e2e`');

  await page.goto('/');

  await expect(page.getByText(`API version ${version}`)).toBeVisible();
});
