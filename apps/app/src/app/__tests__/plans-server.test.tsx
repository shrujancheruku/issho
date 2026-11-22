import { screen, waitFor } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';

import { fakeApi } from '@/test-support/fake-api';
import { aPlan } from '@/test-support/plan-fixtures';
import { TestProviders } from '@/test-support/providers';

import PlansScreen, { loader as plansLoader } from '../(tabs)/plans';
import PlanPage, { loader as planLoader } from '../plans/[id]';

jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({ __esModule: true, default: jest.fn(() => 'light') }));
jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context'),
  useSafeAreaInsets: jest.fn(() => ({ top: 0, bottom: 0, left: 0, right: 0 })),
}));

const mockServerData = jest.fn();
jest.mock('@/plans/server-data', () => ({ useServerData: () => mockServerData() }));

const mockSetResponseHeaders = jest.fn();
jest.mock('expo-server', () => ({ setResponseHeaders: (h: unknown) => mockSetResponseHeaders(h) }));

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
  mockSetResponseHeaders.mockReset();
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
  const answer = (body: unknown, status = 200) => jest.fn().mockResolvedValue(new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } }));
  let fetchSpy: jest.SpyInstance;
  beforeEach(() => {
    process.env.API_ORIGIN = 'http://api.internal:8080';
  });
  afterEach(() => fetchSpy?.mockRestore());

  const sent = () => {
    const [url, init] = fetchSpy.mock.calls[0] as [string, RequestInit];
    return { url, headers: new Headers(init.headers), body: JSON.parse(new TextDecoder().decode(init.body as Uint8Array)) };
  };

  it('asks the API at its own address, in English, with no token, and marks the page cacheable', async () => {
    fetchSpy = jest.spyOn(globalThis, 'fetch').mockImplementation(answer({ plans: [] }));
    const data = await plansLoader();
    expect(data).toEqual({ lang: 'en', plans: [] });
    const request = sent();
    expect(request.url).toBe('http://api.internal:8080/issho.v1.PlanService/ListPlans');
    expect(request.headers.get('authorization')).toBeNull();
    expect(request.body).toEqual({ city: 'da-nang-hoi-an', displayLanguage: 'en' });
    expect(mockSetResponseHeaders).toHaveBeenCalledWith({ 'Cache-Control': expect.stringMatching(/^public, max-age=\d+, s-maxage=\d+/) });
  });

  it("loads a plan by the route's id, and a missing one as null, still cacheable", async () => {
    fetchSpy = jest.spyOn(globalThis, 'fetch').mockImplementation(answer({ code: 'not_found', message: 'no' }, 404));
    expect(await planLoader(undefined, { id: 'nope' })).toEqual({ lang: 'en', plan: null });
    expect(sent().body).toEqual({ id: 'nope', displayLanguage: 'en' });
    expect(mockSetResponseHeaders).toHaveBeenCalledWith({ 'Cache-Control': expect.stringContaining('public') });
  });

  it('hands the screen nothing when the API cannot be reached, so it fetches for itself', async () => {
    fetchSpy = jest.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('down'));
    expect(await plansLoader()).toBeNull();
  });
});
