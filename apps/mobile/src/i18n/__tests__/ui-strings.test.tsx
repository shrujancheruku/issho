// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen } from '@testing-library/react-native';
import { AppState, Text } from 'react-native';

import { api } from '@/lib/api';

import { I18nProvider, useI18n } from '../i18n-provider';
import { catalogFingerprint, english } from '../strings';
import { readCached, writeCached, type CachedStrings } from '../cache';

// useUiStrings is the provider's private hook, so these tests go through
// I18nProvider with the edges faked: the account, the device cache and bundle,
// the API and the app-state events. The real hook, query and clock run.
jest.mock('expo-localization', () => ({ useLocales: () => [{ languageCode: 'es', languageTag: 'es-ES' }] }));
jest.mock('@/features/auth/auth-provider', () => ({ useAuth: () => ({ session: null, languages: null }) }));
jest.mock('../direction', () => ({ applyDirection: jest.fn() }));
jest.mock('../cache', () => ({ readCached: jest.fn(), writeCached: jest.fn() }));
jest.mock('../bundle', () => ({ fromBundle: jest.fn(() => undefined) }));
jest.mock('@/lib/api', () => ({ api: { ui: { getStrings: jest.fn() } } }));

const getStrings = (api as unknown as { ui: { getStrings: jest.Mock } }).ui.getStrings;
const mockRead = readCached as jest.Mock;

const HOUR = 60 * 60 * 1000;
const KEY = 'signIn.sendCode';

function Probe() {
  const { t } = useI18n();
  return <Text>{t(KEY)}</Text>;
}

/** The app's QueryClient defaults (src/app/_layout.tsx): fresh for 30 s, the default 3 retries. */
async function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { staleTime: 30_000 } } });
  await render(
    <QueryClientProvider client={client}>
      <I18nProvider>
        <Probe />
      </I18nProvider>
    </QueryClientProvider>,
  );
}

const advance = (ms: number) => act(async () => { await jest.advanceTimersByTimeAsync(ms); });

const reply = (over: Record<string, unknown> = {}) => ({
  strings: { [KEY]: 'Enviar código' },
  complete: true,
  catalogVersion: 'v1',
  unchanged: false,
  ...over,
});

/** A device copy fetched for this build, `age` ms ago. */
const cachedCopy = (age: number, over: Partial<CachedStrings> = {}): CachedStrings => ({
  strings: { [KEY]: 'Copia' },
  complete: true,
  catalogVersion: 'v1',
  appCatalog: catalogFingerprint,
  checkedAt: Date.now() - age,
  ...over,
});

let appState: (s: string) => void;
let removeListener: jest.Mock;

beforeEach(() => {
  jest.useFakeTimers();
  jest.setSystemTime(new Date('2026-11-04T12:00:00Z'));
  getStrings.mockReset();
  mockRead.mockReset();
  (writeCached as jest.Mock).mockReset();
  removeListener = jest.fn();
  jest.spyOn(AppState, 'addEventListener').mockImplementation(((_: string, cb: (s: string) => void) => {
    appState = cb;
    return { remove: removeListener };
  }) as never);
});

// Fake timers must not leak into the next test or file.
afterEach(() => {
  jest.useRealTimers();
  jest.restoreAllMocks();
});

describe('a language with nothing on the device', () => {
  it('fetches it, shows English until it arrives, then the translation, and saves it', async () => {
    getStrings.mockResolvedValue(reply());
    await mount();
    expect(screen.getByText(english[KEY])).toBeTruthy();
    await advance(0);

    expect(getStrings).toHaveBeenCalledWith({ language: 'es', knownVersion: '' });
    expect(screen.getByText('Enviar código')).toBeTruthy();
    expect(writeCached).toHaveBeenCalledWith('es', expect.objectContaining({ complete: true, catalogVersion: 'v1', appCatalog: catalogFingerprint }));
  });

  it('asks again while the server is still translating, backing off from 5 s to a minute and staying there', async () => {
    getStrings.mockResolvedValue(reply({ complete: false }));
    await mount();
    await advance(0);
    expect(getStrings).toHaveBeenCalledTimes(1);

    // Gaps after each reply: 5, 10, 20, 40, then 60 s (80 s capped), and 60 s again.
    let calls = 1;
    for (const gap of [5_000, 10_000, 20_000, 40_000, 60_000, 60_000]) {
      await advance(gap - 1);
      expect(getStrings).toHaveBeenCalledTimes(calls);
      await advance(1);
      calls += 1;
      expect(getStrings).toHaveBeenCalledTimes(calls);
    }
  });

  it('stops asking once the copy is complete, and checks hourly from then on', async () => {
    getStrings.mockResolvedValueOnce(reply({ complete: false })).mockResolvedValue(reply({ catalogVersion: 'v2' }));
    await mount();
    await advance(0);
    await advance(5_000);
    expect(getStrings).toHaveBeenCalledTimes(2);

    await advance(HOUR - 1);
    expect(getStrings).toHaveBeenCalledTimes(2);
    await advance(1);
    expect(getStrings).toHaveBeenCalledTimes(3);
    // The hourly check sends the version it holds.
    expect(getStrings).toHaveBeenLastCalledWith({ language: 'es', knownVersion: 'v2' });
  });

  it('retries a failing fetch a few times with growing gaps, then gives up for good and shows English', async () => {
    getStrings.mockRejectedValue(new Error('offline'));
    await mount();
    await advance(0);
    expect(getStrings).toHaveBeenCalledTimes(1);

    // React Query's default: 1 s, 2 s, 4 s between the three retries.
    await advance(999);
    expect(getStrings).toHaveBeenCalledTimes(1);
    await advance(1);
    expect(getStrings).toHaveBeenCalledTimes(2);
    await advance(1_999);
    expect(getStrings).toHaveBeenCalledTimes(2);
    await advance(1);
    expect(getStrings).toHaveBeenCalledTimes(3);
    await advance(3_999);
    expect(getStrings).toHaveBeenCalledTimes(3);
    await advance(1);
    expect(getStrings).toHaveBeenCalledTimes(4);

    // Four tries in all, and no polling after that: with no copy there is no schedule.
    await advance(2 * HOUR);
    expect(getStrings).toHaveBeenCalledTimes(4);
    expect(screen.getByText(english[KEY])).toBeTruthy();
  });
});

describe('a copy on the device', () => {
  it('current and checked a minute ago: shown at once, nothing fetched until an hour after the app opened it', async () => {
    mockRead.mockReturnValue(cachedCopy(60_000));
    getStrings.mockResolvedValue(reply({ unchanged: true }));
    await mount();
    await advance(0);
    expect(screen.getByText('Copia')).toBeTruthy();
    expect(getStrings).not.toHaveBeenCalled();

    // The poll runs every hour from when the screen mounted (staleTime only
    // decides the check at mount); it sends the version the device holds.
    await advance(HOUR - 1);
    expect(getStrings).not.toHaveBeenCalled();
    await advance(1);
    expect(getStrings).toHaveBeenCalledTimes(1);
    expect(getStrings).toHaveBeenCalledWith({ language: 'es', knownVersion: 'v1' });
  });

  it('stale (checked over an hour ago): shown at once and checked with its version; "unchanged" keeps the text and renews the check', async () => {
    mockRead.mockReturnValue(cachedCopy(HOUR + 1));
    getStrings.mockResolvedValue(reply({ unchanged: true, strings: {} }));
    await mount();
    expect(screen.getByText('Copia')).toBeTruthy(); // before the server answers
    await advance(0);

    expect(getStrings).toHaveBeenCalledWith({ language: 'es', knownVersion: 'v1' });
    expect(screen.getByText('Copia')).toBeTruthy();
    expect(writeCached).toHaveBeenCalledWith('es', expect.objectContaining({ strings: { [KEY]: 'Copia' }, checkedAt: Date.now() }));
  });

  it('stale and the server has a newer version: the new text replaces it', async () => {
    mockRead.mockReturnValue(cachedCopy(HOUR + 1));
    getStrings.mockResolvedValue(reply({ catalogVersion: 'v2', strings: { [KEY]: 'Mandar código' } }));
    await mount();
    await advance(0);
    expect(screen.getByText('Mandar código')).toBeTruthy();
  });

  it('fetched for other English text (an older build): refetched in full, not by version', async () => {
    mockRead.mockReturnValue(cachedCopy(60_000, { appCatalog: 'an-older-build' }));
    getStrings.mockResolvedValue(reply());
    await mount();
    await advance(0);
    expect(getStrings).toHaveBeenCalledWith({ language: 'es', knownVersion: '' });
    expect(screen.getByText('Enviar código')).toBeTruthy();
  });

  it('incomplete: shown at once and asked for again, backing off', async () => {
    mockRead.mockReturnValue(cachedCopy(60_000, { complete: false }));
    getStrings.mockResolvedValue(reply({ complete: false }));
    await mount();
    await advance(0);
    expect(getStrings).toHaveBeenCalledTimes(1);
    await advance(5_000);
    expect(getStrings).toHaveBeenCalledTimes(2);
    await advance(10_000);
    expect(getStrings).toHaveBeenCalledTimes(3);
  });
});

describe('coming back to the foreground', () => {
  // Timers are paused in the background, so the hook checks on return, but
  // only once the copy is an hour old. The interval is held back here by
  // moving the clock without running timers.
  it('refetches when the copy is over an hour old, and not before', async () => {
    mockRead.mockReturnValue(cachedCopy(60_000));
    getStrings.mockResolvedValue(reply({ unchanged: true }));
    await mount();
    await advance(0);

    jest.setSystemTime(Date.now() + HOUR - 120_000);
    await act(async () => appState('active'));
    await advance(0);
    expect(getStrings).not.toHaveBeenCalled();

    await act(async () => appState('background'));
    jest.setSystemTime(Date.now() + 10 * 60_000);
    await act(async () => appState('background'));
    await advance(0);
    expect(getStrings).not.toHaveBeenCalled(); // going to the background never fetches

    await act(async () => appState('active'));
    await advance(0);
    expect(getStrings).toHaveBeenCalledTimes(1);
  });

  it('stops listening when the screen goes away', async () => {
    mockRead.mockReturnValue(cachedCopy(60_000));
    await mount();
    await screen.unmount();
    expect(removeListener).toHaveBeenCalled();
  });
});
