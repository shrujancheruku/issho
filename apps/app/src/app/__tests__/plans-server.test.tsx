import { screen, waitFor } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';

import { fakeApi } from '@/test-support/fake-api';
import { aPlan } from '@/test-support/plan-fixtures';
import { TestProviders } from '@/test-support/providers';

import PlansScreen, { loader as plansLoader } from '../(tabs)/plans';
import PlanPage, { generateMetadata as planMetadataFor, loader as planLoader } from '../plans/[id]';

jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({ __esModule: true, default: jest.fn(() => 'light') }));
jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context'),
  useSafeAreaInsets: jest.fn(() => ({ top: 0, bottom: 0, left: 0, right: 0 })),
}));

const mockServerData = jest.fn();
jest.mock('@/plans/server-data', () => ({ useServerData: () => mockServerData() }));

// expo-server queues each `setResponseHeaders` call and applies the queue to the
// finished response in push order, a record by `headers.set` and a function on
// the headers (expo-server/build/cjs/runtime/index.js). The mock keeps the same
// queue, and `cacheControl()` applies it the same way, so a test sees the header
// the page would be sent with.
const mockQueuedHeaders: unknown[] = [];
const mockSetResponseHeaders = jest.fn((update: unknown) => {
  mockQueuedHeaders.push(update);
});
jest.mock('expo-server', () => ({ setResponseHeaders: (h: unknown) => mockSetResponseHeaders(h) }));

function cacheControl(): string | null {
  const headers = new Headers();
  for (const update of mockQueuedHeaders) {
    if (typeof update === 'function') (update as (h: Headers) => unknown)(headers);
    else for (const [name, value] of Object.entries(update as Record<string, string>)) headers.set(name, value);
  }
  return headers.get('cache-control');
}

async function render(lang: 'en' | 'vi', api: ReturnType<typeof fakeApi>, route: Record<string, unknown>, initialUrl: string) {
  await renderRouter(route as never, {
    initialUrl,
    wrapper: ({ children }: { children: ReactNode }) => (
      <TestProviders lang={lang} api={api}>
        {children}
      </TestProviders>
    ),
  } as never);
}

afterEach(() => {
  mockServerData.mockReset();
  mockSetResponseHeaders.mockClear();
  mockQueuedHeaders.length = 0;
  delete process.env.API_ORIGIN;
});

describe('a page the Expo server already filled in', () => {
  it("shows the server's plans without asking the API again, when they are in the reader's language", async () => {
    mockServerData.mockReturnValue({ lang: 'en', plans: [aPlan({ title: 'From the server' })] });
    const api = fakeApi();
    await render('en', api, { '(tabs)/plans': PlansScreen }, '/plans');
    expect(await screen.findByText('From the server')).toBeTruthy();
    expect(api.plans.listPlans).not.toHaveBeenCalled();
  });

  it("fetches again when the reader's language is not the one the server asked in", async () => {
    mockServerData.mockReturnValue({ lang: 'en', plans: [aPlan({ title: 'From the server' })] });
    const api = fakeApi();
    api.plans.listPlans.mockResolvedValue({ plans: [aPlan({ language: 'vi', title: 'Từ API' })] });
    await render('vi', api, { '(tabs)/plans': PlansScreen }, '/plans');
    expect(await screen.findByText('Từ API')).toBeTruthy();
    expect(api.plans.listPlans).toHaveBeenCalledWith({ city: 'da-nang-hoi-an', displayLanguage: 'vi' });
  });

  it('fetches for itself when the server could not', async () => {
    mockServerData.mockReturnValue(null);
    const api = fakeApi();
    api.plans.listPlans.mockResolvedValue({ plans: [aPlan({ title: 'From the client' })] });
    await render('en', api, { '(tabs)/plans': PlansScreen }, '/plans');
    expect(await screen.findByText('From the client')).toBeTruthy();
  });

  it("shows the server's plan page, and its not-found, without asking the API", async () => {
    mockServerData.mockReturnValue({ lang: 'en', plan: aPlan({ title: 'Served plan' }) });
    const api = fakeApi();
    await render('en', api, { 'plans/[id]': PlanPage }, '/plans/p1');
    expect(await screen.findByText('Served plan')).toBeTruthy();
    expect(api.plans.getPlan).not.toHaveBeenCalled();
  });

  it('shows not found from the server for a plan that is not there', async () => {
    mockServerData.mockReturnValue({ lang: 'en', plan: null });
    const api = fakeApi();
    await render('en', api, { 'plans/[id]': PlanPage }, '/plans/nope');
    await waitFor(() => expect(screen.getByText('We could not find that plan.')).toBeTruthy());
    expect(api.plans.getPlan).not.toHaveBeenCalled();
  });
});

describe("the server's loaders", () => {
  const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
  const answer = (body: unknown, status = 200) => jest.fn().mockImplementation(async () => json(body, status));
  const PUBLIC = /^public, max-age=\d+, s-maxage=\d+/;
  let fetchSpy: jest.SpyInstance;
  let now = 1_000_000;
  beforeEach(() => {
    process.env.API_ORIGIN = 'http://api.internal:8080';
    now = 1_000_000;
    jest.spyOn(Date, 'now').mockImplementation(() => now);
  });
  afterEach(() => {
    fetchSpy?.mockRestore();
    jest.restoreAllMocks();
  });

  const sent = () => {
    const [url, init] = fetchSpy.mock.calls[0] as [string, RequestInit];
    return { url, headers: new Headers(init.headers), body: JSON.parse(new TextDecoder().decode(init.body as Uint8Array)) };
  };

  /** One request for a plan page, in the order Expo Router runs it: the head (`resolveMetadata`), then the loader (`executeLoader`). */
  async function servePlanPage(id: string) {
    const head = await planMetadataFor(undefined, { id });
    const data = await planLoader(undefined, { id });
    return { head, data };
  }

  /** The API answers each call with the next of these, then repeats the last. */
  function apiAnswers(...outcomes: Array<'ok' | 'down'>) {
    let calls = 0;
    fetchSpy = jest.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      const outcome = outcomes[Math.min(calls++, outcomes.length - 1)];
      if (outcome === 'down') throw new Error('down');
      return json({ plan: aPlan({ title: 'Lantern making' }) });
    });
  }

  it('asks the API at its own address, in English, with no token, and marks the page cacheable', async () => {
    fetchSpy = jest.spyOn(globalThis, 'fetch').mockImplementation(answer({ plans: [] }));
    const data = await plansLoader();
    expect(data).toEqual({ lang: 'en', plans: [] });
    const request = sent();
    expect(request.url).toBe('http://api.internal:8080/issho.v1.PlanService/ListPlans');
    expect(request.headers.get('authorization')).toBeNull();
    expect(request.body).toEqual({ city: 'da-nang-hoi-an', displayLanguage: 'en' });
    expect(cacheControl()).toMatch(PUBLIC);
  });

  it("loads a plan by the route's id, and a missing one as null, still cacheable", async () => {
    fetchSpy = jest.spyOn(globalThis, 'fetch').mockImplementation(answer({ code: 'not_found', message: 'no' }, 404));
    expect(await planLoader(undefined, { id: 'nope' })).toEqual({ lang: 'en', plan: null });
    expect(sent().body).toEqual({ id: 'nope', displayLanguage: 'en' });
    expect(cacheControl()).toMatch(PUBLIC);
  });

  it('hands the screen nothing when the API cannot be reached, so it fetches for itself', async () => {
    fetchSpy = jest.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('down'));
    expect(await plansLoader()).toBeNull();
  });

  describe('the list page', () => {
    it('never lets a shared cache keep a list the API failed to fill', async () => {
      fetchSpy = jest.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('down'));
      expect(await plansLoader()).toBeNull();
      expect(cacheControl()).toBe('no-store');

      mockQueuedHeaders.length = 0;
      fetchSpy.mockImplementation(async () => json({ code: 'unavailable', message: 'x' }, 503));
      expect(await plansLoader()).toBeNull();
      expect(cacheControl()).toBe('no-store');
    });

    it('answers no-store when the server cannot even build its API client', async () => {
      fetchSpy = jest.spyOn(globalThis, 'fetch');
      delete process.env.API_ORIGIN;
      delete process.env.EXPO_PUBLIC_API_URL;
      expect(await plansLoader()).toBeNull();
      expect(cacheControl()).toBe('no-store');
    });
  });

  describe('a plan page, with the head and the loader run in Expo order and their header updates queued', () => {
    it('both succeed: public, from one call to the API', async () => {
      apiAnswers('ok');
      const { head, data } = await servePlanPage('both-ok');
      expect(head).toMatchObject({ title: 'Lantern making' });
      expect(data).toMatchObject({ plan: { title: 'Lantern making' } });
      expect(cacheControl()).toMatch(PUBLIC);
      expect(fetchSpy).toHaveBeenCalledTimes(1);
    });

    it('the head call fails and the loader would have succeeded: no-store, not the later public header', async () => {
      apiAnswers('down', 'ok');
      const { head, data } = await servePlanPage('head-fails');
      expect(head).toEqual({ robots: 'noindex' });
      expect(data).toBeNull();
      expect(cacheControl()).toBe('no-store');
      expect(fetchSpy).toHaveBeenCalledTimes(1);
    });

    it("the loader reads the head's answer rather than asking again: a second call that would fail is never made", async () => {
      apiAnswers('ok', 'down');
      const { head, data } = await servePlanPage('one-answer');
      // Both read the one answer, so a page never has a head the body does not match.
      expect(head).toMatchObject({ title: 'Lantern making' });
      expect(data).toMatchObject({ plan: { title: 'Lantern making' } });
      expect(cacheControl()).toMatch(PUBLIC);
      expect(fetchSpy).toHaveBeenCalledTimes(1);
    });

    it('the API fails for both: no-store', async () => {
      apiAnswers('down');
      const { head, data } = await servePlanPage('both-fail');
      expect(head).toEqual({ robots: 'noindex' });
      expect(data).toBeNull();
      expect(cacheControl()).toBe('no-store');
      expect(fetchSpy).toHaveBeenCalledTimes(1);
    });

    it('a 503 is a failure too', async () => {
      fetchSpy = jest.spyOn(globalThis, 'fetch').mockImplementation(async () => json({ code: 'unavailable', message: 'x' }, 503));
      await servePlanPage('five-oh-three');
      expect(cacheControl()).toBe('no-store');
    });

    // The memo is a few seconds long. If the two calls ever land further apart
    // than that, each makes its own call and they can disagree; no-store must win
    // whichever one fails, in whichever order the updates were queued.
    it('keeps no-store when the two calls disagree: the head fails, then the loader succeeds', async () => {
      apiAnswers('down', 'ok');
      await planMetadataFor(undefined, { id: 'late-loader' });
      now += 60_000;
      await planLoader(undefined, { id: 'late-loader' });
      expect(fetchSpy).toHaveBeenCalledTimes(2);
      expect(cacheControl()).toBe('no-store');
    });

    it('keeps no-store when the two calls disagree: the head succeeds, then the loader fails', async () => {
      apiAnswers('ok', 'down');
      await planMetadataFor(undefined, { id: 'late-loader-fails' });
      now += 60_000;
      await planLoader(undefined, { id: 'late-loader-fails' });
      expect(fetchSpy).toHaveBeenCalledTimes(2);
      expect(cacheControl()).toBe('no-store');
    });

    it('asks again for a later request: a failure is not remembered past the memo', async () => {
      apiAnswers('down', 'ok');
      await servePlanPage('later-request');
      expect(cacheControl()).toBe('no-store');
      mockQueuedHeaders.length = 0;
      now += 60_000;
      await servePlanPage('later-request');
      expect(cacheControl()).toMatch(PUBLIC);
      expect(fetchSpy).toHaveBeenCalledTimes(2);
    });
  });
});
