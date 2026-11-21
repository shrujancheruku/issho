import { render, screen } from '@testing-library/react-native';

import { fakeApi } from '@/test-support/fake-api';
import { TestProviders } from '@/test-support/providers';

import YouScreen from '../(tabs)/you';

// The dev deploy's walk (docs/deploy.md): the web page shows the version the API
// reports, so a deploy that left an old API running shows on the page.
async function renderYou(lang: 'en' | 'vi', api = fakeApi()) {
  await render(
    <TestProviders lang={lang} api={api}>
      <YouScreen />
    </TestProviders>,
  );
  return api;
}

describe('the You tab', () => {
  it('shows the version the API reports', async () => {
    const api = fakeApi();
    api.health.check.mockResolvedValue({ version: 'a1b2c3d' });
    await renderYou('en', api);
    expect(await screen.findByText('API version a1b2c3d')).toBeTruthy();
  });

  it('says so in Vietnamese', async () => {
    const api = fakeApi();
    api.health.check.mockResolvedValue({ version: 'a1b2c3d' });
    await renderYou('vi', api);
    expect(await screen.findByText('Phiên bản API a1b2c3d')).toBeTruthy();
  });

  it('says the API could not be reached when the health check fails', async () => {
    const api = fakeApi();
    api.health.check.mockRejectedValue(new Error('connection refused'));
    await renderYou('en', api);
    expect(await screen.findByText('The API could not be reached.')).toBeTruthy();
  });
});
