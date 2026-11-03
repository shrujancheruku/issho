import { create } from '@bufbuild/protobuf';
import { Code, ConnectError, GetTranslatorResponseSchema } from '@issho/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen } from '@testing-library/react';
import { act } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { App } from './App';
import { fake } from './test/fakeApi';

const auth = vi.hoisted(() => ({
  session: null as unknown,
  listener: null as null | ((event: string, s: unknown) => void),
}));

vi.mock('./lib', async () => ({
  api: (await import('./test/fakeApi')).fake.api,
  supabase: {
    auth: {
      getSession: async () => ({ data: { session: auth.session } }),
      onAuthStateChange: (cb: (event: string, s: unknown) => void) => {
        auth.listener = cb;
        return { data: { subscription: { unsubscribe: () => { auth.listener = null; } } } };
      },
      signOut: async () => {},
      refreshSession: async () => {},
    },
  },
}));
vi.mock('./SignIn', () => ({ SignIn: () => <p>sign-in screen</p> }));

// Each page is a stub that shows the props the router handed it.
vi.mock('./pages/Plans', () => ({ Plans: () => <p>page:plans</p> }));
vi.mock('./pages/Groups', () => ({ Groups: () => <p>page:groups</p> }));
vi.mock('./pages/Venues', () => ({ Venues: () => <p>page:venues</p> }));
vi.mock('./pages/Reports', () => ({ Reports: ({ about }: { about?: string }) => <p>page:reports about={about ?? 'none'}</p> }));
vi.mock('./pages/Flags', () => ({ Flags: () => <p>page:flags</p> }));
vi.mock('./pages/CrewLeaves', () => ({ CrewLeaves: () => <p>page:leaves</p> }));
vi.mock('./pages/Translations', () => ({ Translations: () => <p>page:translations</p> }));
vi.mock('./pages/People', () => ({ People: () => <p>page:people</p> }));
vi.mock('./pages/Metrics', () => ({ Metrics: () => <p>page:metrics</p> }));
vi.mock('./pages/Cities', () => ({ Cities: () => <p>page:cities</p> }));
vi.mock('./pages/Person', () => ({ Person: ({ userId }: { userId: string }) => <p>person:{userId}</p> }));
vi.mock('./pages/PlanDetail', () => ({ PlanDetail: ({ planId }: { planId: string }) => <p>plan:{planId}</p> }));
vi.mock('./pages/GroupReview', () => ({ GroupReview: ({ eventId }: { eventId: string }) => <p>event:{eventId}</p> }));
vi.mock('./pages/CityEditor', () => ({ CityEditor: ({ cityId }: { cityId: string }) => <p>city:{cityId}</p> }));

const id = '123e4567-e89b-12d3-a456-426614174000';

function open(hash: string, session: unknown = { user: { email: 'a@example.com' } }) {
  auth.session = session;
  window.location.hash = hash;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><App /></QueryClientProvider>);
}

beforeEach(() => {
  fake.on('translation.getTranslator', () => create(GetTranslatorResponseSchema, { admin: true }));
});
afterEach(() => {
  cleanup();
  window.location.hash = '';
});

describe('the hash router', () => {
  it.each([
    ['#/plans', 'page:plans'],
    ['#/groups', 'page:groups'],
    ['#/venues', 'page:venues'],
    ['#/reports', 'page:reports about=none'],
    ['#/flags', 'page:flags'],
    ['#/leaves', 'page:leaves'],
    ['#/translations', 'page:translations'],
    ['#/people', 'page:people'],
    ['#/metrics', 'page:metrics'],
    ['#/cities', 'page:cities'],
    [`#/reports?about=${id}`, `page:reports about=${id}`],
    [`#/person/${id}`, `person:${id}`],
    [`#/plan/${id}`, `plan:${id}`],
    [`#/event/${id}`, `event:${id}`],
    ['#/city/tokyo', 'city:tokyo'],
    ['#/city/new', 'city:new'],
  ])('%s shows %s', async (hash, text) => {
    open(hash);
    expect(await screen.findByText(text)).toBeTruthy();
  });

  it.each(['', '#/nonsense', '#/person/not-a-uuid', '#/reports?about=nope'])('an unknown hash (%j) falls back to Reports', async (hash) => {
    open(hash);
    expect(await screen.findByText('page:reports about=none')).toBeTruthy();
  });

  it('follows a hash change while open', async () => {
    open('#/plans');
    await screen.findByText('page:plans');
    await act(async () => {
      window.location.hash = '#/venues';
      window.dispatchEvent(new HashChangeEvent('hashchange'));
    });
    expect(await screen.findByText('page:venues')).toBeTruthy();
  });

  it('keeps a deep link through sign-in: signed out shows the sign-in screen and no RPC; signing in lands on the link', async () => {
    open(`#/plan/${id}`, null);
    expect(await screen.findByText('sign-in screen')).toBeTruthy();
    expect(fake.calls).toEqual([]);
    expect(window.location.hash).toBe(`#/plan/${id}`);

    await act(async () => { auth.listener?.('SIGNED_IN', { user: { email: 'a@example.com' } }); });
    expect(await screen.findByText(`plan:${id}`)).toBeTruthy();
  });

  it('shows a translator only the translation editor, whatever the hash', async () => {
    fake.on('translation.getTranslator', () => create(GetTranslatorResponseSchema, { admin: false, languages: ['ja'] }));
    open(`#/person/${id}`);
    expect(await screen.findByText('page:translations')).toBeTruthy();
    expect(screen.queryByText(`person:${id}`)).toBeNull();
    expect(screen.getByText(/translator \(ja\)/)).toBeTruthy();
  });

  it('says there is no access when the role is denied', async () => {
    fake.on('translation.getTranslator', () => { throw new ConnectError('no', Code.PermissionDenied); });
    open('#/plans');
    expect(await screen.findByText('No access')).toBeTruthy();
  });
});
