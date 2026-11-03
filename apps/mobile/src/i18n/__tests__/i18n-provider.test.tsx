// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { render } from '@testing-library/react-native';

import { I18nProvider, useI18n } from '../i18n-provider';

// Only the provider's own memoising is under test: its inputs are faked.
jest.mock('@tanstack/react-query', () => ({
  useQuery: () => ({ data: undefined, refetch: jest.fn(), dataUpdatedAt: 0 }),
  useQueryClient: () => ({ getQueryData: jest.fn() }),
}));
jest.mock('expo-localization', () => ({ useLocales: () => mockLocales }));
jest.mock('@/features/auth/auth-provider', () => ({ useAuth: () => ({ session: null, languages: null }) }));
jest.mock('@/lib/api', () => ({ api: {} }));
jest.mock('../direction', () => ({ applyDirection: () => undefined }));
jest.mock('../cache', () => ({ readCached: () => undefined, writeCached: () => undefined }));

const mockLocales = [{ languageTag: 'en-US', languageCode: 'en' }];

describe('I18nProvider', () => {
  it('keeps its context value when it re-renders with the same inputs', async () => {
    const seen: unknown[] = [];
    function Probe() {
      seen.push(useI18n());
      return null;
    }
    // A fresh element each time, so the provider really re-renders.
    const tree = () => (
      <I18nProvider>
        <Probe />
      </I18nProvider>
    );
    const view = await render(tree());
    await view.rerender(tree());
    expect(seen.length).toBeGreaterThan(1);
    // Each render rebuilds `languages`; the value must not follow it.
    expect(new Set(seen).size).toBe(1);
  });
});
