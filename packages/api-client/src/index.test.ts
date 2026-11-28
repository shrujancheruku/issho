import { describe, expect, it } from 'vitest';

import { Code, ConnectError } from '@connectrpc/connect';

import { create } from '@bufbuild/protobuf';

import { createApi, InvitationAnswer, JoinLinkProblemReason, JoinLinkProblemSchema, OutingState, ProfileProblemReason, ProfileProblemSchema, profileProblem, SeatState } from './index';

// A fake fetch that answers the health check and keeps the request it saw.
function fakeFetch() {
  const seen: Request[] = [];
  const fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    seen.push(new Request(input, init));
    return new Response(JSON.stringify({ version: '1.2.3' }), {
      headers: { 'content-type': 'application/json' },
    });
  };
  return { fetch, seen };
}

describe('createApi', () => {
  it('calls the health check and reads the version', async () => {
    const { fetch, seen } = fakeFetch();
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => null, fetch });

    const res = await api.health.check({});

    expect(res.version).toBe('1.2.3');
    expect(seen[0].url).toBe('https://api.example.test/issho.v1.HealthService/Check');
  });

  it('sends the session token as a bearer token when there is one', async () => {
    const { fetch, seen } = fakeFetch();
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'tok-1', fetch });

    await api.health.check({});

    expect(seen[0].headers.get('authorization')).toBe('Bearer tok-1');
  });

  it('sends no authorization header when signed out', async () => {
    const { fetch, seen } = fakeFetch();
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => null, fetch });

    await api.health.check({});

    expect(seen[0].headers.has('authorization')).toBe(false);
  });

  it("reads the caller's own account from the account service", async () => {
    const seen: Request[] = [];
    const fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      seen.push(new Request(input, init));
      return new Response(
        JSON.stringify({ account: { id: 'a-1', email: 'ada@example.com', city: 'da-nang-hoi-an', displayLanguage: 'vi' } }),
        { headers: { 'content-type': 'application/json' } },
      );
    };
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'tok-1', fetch });

    const res = await api.account.getAccount({ displayLanguage: 'vi' });

    expect(res.account?.email).toBe('ada@example.com');
    expect(seen[0].url).toBe('https://api.example.test/issho.v1.AccountService/GetAccount');
    expect(seen[0].headers.get('authorization')).toBe('Bearer tok-1');
  });

  it('tells the caller when the API answers 401, and still throws the error', async () => {
    const fetch = async () =>
      new Response(JSON.stringify({ code: 'unauthenticated', message: 'bad token' }), {
        status: 401,
        headers: { 'content-type': 'application/json' },
      });
    let told = 0;
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'stale', fetch, onUnauthenticated: () => void told++ });

    await expect(api.account.getAccount({})).rejects.toMatchObject({ code: Code.Unauthenticated });
    expect(told).toBe(1);
  });

  it('does not call onUnauthenticated for other failures', async () => {
    const fetch = async () =>
      new Response(JSON.stringify({ code: 'internal', message: 'boom' }), {
        status: 500,
        headers: { 'content-type': 'application/json' },
      });
    let told = 0;
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'tok', fetch, onUnauthenticated: () => void told++ });

    await expect(api.account.getAccount({})).rejects.toBeDefined();
    expect(told).toBe(0);
  });

  it("saves the caller's own profile through the profile service", async () => {
    const seen: Request[] = [];
    const fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      seen.push(new Request(input, init));
      return new Response(JSON.stringify({ profile: { name: 'Ada', gender: 'woman', languages: ['en'], livesInCity: true } }), {
        headers: { 'content-type': 'application/json' },
      });
    };
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'tok-1', fetch });

    const res = await api.profile.saveProfile({
      name: 'Ada',
      dateOfBirth: '1990-06-15',
      gender: 'woman',
      languages: ['en'],
      livesInCity: true,
      acceptedCodeOfConduct: true,
    });

    expect(res.profile?.name).toBe('Ada');
    expect(seen[0].url).toBe('https://api.example.test/issho.v1.ProfileService/SaveProfile');
    expect(seen[0].headers.get('authorization')).toBe('Bearer tok-1');
  });

  it('signs the caller up to a plan with times through the signup service', async () => {
    const seen: Request[] = [];
    const fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      seen.push(new Request(input, init));
      return new Response(
        JSON.stringify({ signup: { planId: 'p1', ticks: [{ date: '2026-01-08', part: 'evening' }], hearBy: '2026-01-11' } }),
        { headers: { 'content-type': 'application/json' } },
      );
    };
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'tok-1', fetch });

    const res = await api.signup.signUp({ planId: 'p1', ticks: [{ date: '2026-01-08', part: 'evening' }] });

    expect(res.signup?.hearBy).toBe('2026-01-11');
    expect(seen[0].url).toBe('https://api.example.test/issho.v1.SignupService/SignUp');
    expect(seen[0].headers.get('authorization')).toBe('Bearer tok-1');
  });

  it("lists the caller's own outings through the outing service, with no one else in them", async () => {
    const seen: Request[] = [];
    const fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      seen.push(new Request(input, init));
      return new Response(
        JSON.stringify({
          outings: [
            {
              id: 'o1',
              planId: 'p1',
              date: '2026-01-09',
              part: 'evening',
              startsAt: '17:00',
              languages: ['en', 'vi'],
              seatState: 'SEAT_STATE_INVITED',
              invitedAt: '2026-01-05T09:00:00Z',
              invitationEndsAt: '2026-01-07T09:00:00Z',
            },
          ],
        }),
        { headers: { 'content-type': 'application/json' } },
      );
    };
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'tok-1', fetch });

    const res = await api.outing.listMyOutings({});

    expect(res.outings[0]).toMatchObject({ date: '2026-01-09', startsAt: '17:00', languages: ['en', 'vi'] });
    expect(res.outings[0].seatState).toBe(SeatState.INVITED);
    expect(seen[0].url).toBe('https://api.example.test/issho.v1.OutingService/ListMyOutings');
    expect(seen[0].headers.get('authorization')).toBe('Bearer tok-1');
  });

  it('answers an invitation and reads the count of yeses and the first names of those going', async () => {
    const seen: Request[] = [];
    const fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      seen.push(new Request(input, init));
      return new Response(
        JSON.stringify({
          outing: {
            id: 'o1',
            seatState: 'SEAT_STATE_GOING',
            outingState: 'OUTING_STATE_SET',
            yesCount: 3,
            othersGoing: ['Grace', 'Katherine'],
          },
        }),
        { headers: { 'content-type': 'application/json' } },
      );
    };
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'tok-1', fetch });

    const res = await api.outing.answerInvitation({ outingId: 'o1', answer: InvitationAnswer.YES });

    expect(res.outing).toMatchObject({ yesCount: 3, othersGoing: ['Grace', 'Katherine'] });
    expect(res.outing?.seatState).toBe(SeatState.GOING);
    expect(res.outing?.outingState).toBe(OutingState.SET);
    expect(seen[0].url).toBe('https://api.example.test/issho.v1.OutingService/AnswerInvitation');
    expect(await seen[0].json()).toEqual({ outingId: 'o1', answer: 'INVITATION_ANSWER_YES' });
  });

  it('cancels a seat', async () => {
    const seen: Request[] = [];
    const fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      seen.push(new Request(input, init));
      return new Response(JSON.stringify({ outing: { id: 'o1', seatState: 'SEAT_STATE_CANCELLED' } }), {
        headers: { 'content-type': 'application/json' },
      });
    };
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'tok-1', fetch });

    const res = await api.outing.cancelSeat({ outingId: 'o1' });

    expect(res.outing?.seatState).toBe(SeatState.CANCELLED);
    expect(seen[0].url).toBe('https://api.example.test/issho.v1.OutingService/CancelSeat');
  });

  it('redeems a join link through the join link service, with the code in the body and a problem the app can read', async () => {
    const seen: Request[] = [];
    const fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      seen.push(new Request(input, init));
      return new Response(
        JSON.stringify({
          code: 'failed_precondition',
          message: 'this join link cannot be used',
          details: [{ type: 'issho.v1.JoinLinkProblem', value: 'CAM=' }],
        }),
        { status: 400, headers: { 'content-type': 'application/json' } },
      );
    };
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'tok-1', fetch });

    const err = await api.joinLink.redeemJoinLink({ code: 'abc' }).catch((e) => e);

    expect(seen[0].url).toBe('https://api.example.test/issho.v1.JoinLinkService/RedeemJoinLink');
    expect(await seen[0].json()).toEqual({ code: 'abc' });
    expect(ConnectError.from(err).code).toBe(Code.FailedPrecondition);
    expect(ConnectError.from(err).findDetails(JoinLinkProblemSchema)[0]?.reason).toBe(JoinLinkProblemReason.LINK_UNUSABLE);
  });
});

// The profile screens' block (#614).
describe('profileProblem', () => {
  it('reads the ProfileProblem reason an RPC refused with, and nothing from any other error', async () => {
    const { ConnectError } = await import('@connectrpc/connect');
    const refused = new ConnectError('under 18', Code.InvalidArgument, undefined, [
      { desc: ProfileProblemSchema, value: create(ProfileProblemSchema, { reason: ProfileProblemReason.UNDER_AGE }) },
    ]);
    expect(profileProblem(refused)).toBe(ProfileProblemReason.UNDER_AGE);
    expect(profileProblem(new ConnectError('nope', Code.InvalidArgument))).toBeUndefined();
    expect(profileProblem(new Error('network'))).toBeUndefined();
  });
});
