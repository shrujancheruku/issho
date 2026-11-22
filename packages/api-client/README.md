# @issho/api-client

The generated TypeScript client for the Issho API, used by the app, the Expo server and the admin.

- `src/gen/` is **generated** from `proto/` by `protoc-gen-es` (`make gen` at the root, after `make setup`). Don't edit it.
- `src/index.ts` exports `createApi()` and re-exports the generated message types.

## Usage

```ts
import { createApi } from '@issho/api-client';

const api = createApi({
  baseUrl: 'http://localhost:8080',
  getAccessToken: async () => session?.access_token ?? null, // the Supabase session
});

const { version } = await api.health.check({});

// The signed-in person's own account; the first call makes it (members only).
const { account } = await api.account.getAccount({ displayLanguage: 'vi' });

// The profile asked at the first signup; unset until it is saved. A refusal carries a
// ProfileProblem detail: err.findDetails(ProfileProblemSchema)[0]?.reason is
// PROFILE_REQUIRED (open the questions) or UNDER_AGE (stop with a plain message).
const { profile } = await api.profile.getProfile({});

// A signup commits nobody: parts of the day on real dates, from 3 days ahead. Calling it
// again changes the times. A tick the plan refuses carries a SignupProblem detail
// (err.findDetails(SignupProblemSchema)[0]?.reason), and a caller with no profile gets
// ProfileProblem PROFILE_REQUIRED, as above.
const { signup } = await api.signup.signUp({ planId, ticks: [{ date: '2026-01-08', part: 'evening' }] });
// signup.hearBy is the day they will hear by; api.signup.listMySignups({}) and
// api.signup.withdrawSignup({ planId }) are the rest.

// The caller's own outings, earliest date first: the date, the time (startsAt), the languages the
// group shares and the state of their seat. Never another person's name or id.
const { outings } = await api.outing.listMyOutings({});
// An invited person answers for 48 hours (a no can become a yes); the third yes sets the outing.
// Each outing carries yesCount (a count, never who) and, once the person is going, othersGoing
// (the first names of the others going).
await api.outing.answerInvitation({ outingId: outings[0].id, answer: InvitationAnswer.YES });
await api.outing.cancelSeat({ outingId: outings[0].id }); // only a person going can cancel
```

- `getAccessToken` is called before every request. A token goes out as `Authorization: Bearer <token>`; with none, no header is sent, which only public RPCs accept.
- `onUnauthenticated` (optional) is called when the API answers that the caller is not signed in (the Connect code `unauthenticated`, HTTP 401); the error is still thrown. The app ends the session there, once, so no screen handles it.
- The transport is Connect over `fetch` with JSON encoding, which works with React Native's `fetch` and reads in a network inspector.
- Errors are thrown as `ConnectError`; branch on its `.code` (`Code.Unauthenticated`, `Code.PermissionDenied`).

## Adding a service

Add the `.proto` and run `make gen` (see `proto/README.md`), then import the generated service in `src/index.ts`, re-export its types and add it to the object `createApi` returns.

## Checks

`make lint` type-checks it (`pnpm --filter @issho/api-client typecheck`) and `make test` runs its tests (`src/index.test.ts`, against a fake `fetch`).
