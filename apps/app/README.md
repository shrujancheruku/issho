# apps/app

The member app: one Expo project for iOS, Android and the web ([Architecture and stack](../../docs/spec/architecture.md#what-v2-is-built-from)). The web is the same project, server-rendered (`web.output: "server"` in `app.json`). Right now it is the shell (three tabs, the strings and the theme), the Plans tab and a plan's page (below), sign-in by email code, and on the You tab the person's email, their profile (with a way to change it), a sign-out button and the version the API reports (also on the sign-in screen, where a signed-out visit to `/you` lands) (`src/components/api-version.tsx`), which is what the dev deploy checks. Members never see the version: it shows only when the build's `EXPO_PUBLIC_API_URL` is an API that holds no real people (`isDevApi` in `src/api/api-url.ts`: a local address, or a name whose first label is `dev` or ends in `-dev`, as `dev.joinissho.com` and `api-dev.joinissho.com` do). Any other address, production's included, shows nothing and makes no call. Arriving by a join link and a member's own join links are in their own section below, and so is signing up to a plan with times, which Together lists. The rest of the screens come in later tickets.

## Setup

From the repo root, once: `make setup` (installs the workspace with pnpm and enables the git hooks). Nothing here needs a secret.

`EXPO_PUBLIC_API_URL` is the Go API's address (default `http://localhost:8080`). Like every `EXPO_PUBLIC_*` value it is bundled into the app, so it is public.

Sign-in also needs the Supabase project's address and publishable key: `EXPO_PUBLIC_SUPABASE_URL` and `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY`. Both are public by design ([docs/secrets.md](../../docs/secrets.md)). A build without them still runs; asking for a code then says it could not be sent. The web image and the dev deploy do not pass them yet.

## Running

```sh
pnpm --filter @issho/app web      # the web, server-rendered, at http://localhost:8081
pnpm --filter @issho/app start    # Expo's dev server for a phone or simulator
```

Pass `--port <n>` after `expo start` to use another port. To see the server render, fetch a page without running any JavaScript: `curl localhost:8081/plans/<id>` holds the plan's text, its link-card tags (`<title>`, `description`, `og:*`, `twitter:*`) and a `Cache-Control` header, and `curl localhost:8081/plans` holds the list. A visitor who opens `/` is sent to `/plans?open=1` by the server (a 302 from `src/app/+middleware.ts`; the `redirects` setting in `app.json` crashes the native app, which then finds `/` twice), and `src/app/index.tsx` does the same on a phone (see "Signing up with times" for what `open=1` is).

## The Plans tab and the plan page

`/plans` (the Plans tab, `src/app/(tabs)/plans.tsx`) lists the city's plans and `/plans/<id>` (`src/app/plans/[id].tsx`) is the one screen a plan is drawn on: title, description, price, when, the stops with the leg to the next, where to meet, what to bring. Both are public: a visitor sees them, the same as a member. Both call `api.plans` (`@issho/api-client`), whose RPCs need no token. The server decides what is shown (a closed city's plans, and a plan before its opening day, are NotFound): the app never filters.

- **On the web they are server-rendered.** Each route exports a `loader` (Expo Router's data loader) that runs on the Expo server, asks the API in English with no token, and returns the plans; the data is in the HTML and seeds the screen on hydration. A route's `generateMetadata` writes the plan's title and description into `<head>` for the browser tab and for the link card a messenger draws. The server sets `Cache-Control: public, max-age=60, s-maxage=300, stale-while-revalidate=600` (`src/plans/server-loaders.ts`) and holds no token or cookie, so nothing per-visitor is in a cached page. A shared cache never keeps a page the API failed to fill, whichever part failed. Expo Router runs a plan page's `generateMetadata` and then its `loader`, and expo-server applies their header updates in the order they were queued, so a plan page makes one call to the API and both read it (kept a few seconds, failure included: a plan holds nothing per visitor); a failure answers `Cache-Control: no-store`, and a success sets the public header only where `no-store` isn't already set, so no order or timing can end a failed page public. It calls the API at `API_ORIGIN` when set (the same address `apps/web` forwards the browser's calls to), else at `EXPO_PUBLIC_API_URL`.
- **A reader in another language** gets the server's English HTML first and the text in their language right after hydration (`src/plans/use-loaded.ts`): the server doesn't know the visitor, so a page that varied by language couldn't be cached.
- **On a phone** there is no server: the screens call the API themselves (`src/plans/server-data.ts` has no data, `server-data.web.ts` reads the loader's). A server that could not reach the API hands over nothing, and the screen fetches for itself.
- **A plan that doesn't exist** shows "We could not find that plan." with a link to the list, and the page is `noindex`. The status is still 200: Expo's loaders cannot set a status without replacing the page, so a real 404 (and the 410 for a retired plan, [Architecture and stack](../../docs/spec/architecture.md#the-web-at-one-address)) waits for plan states.
- The city is the Pilot's one, `DEFAULT_CITY` (`src/plans/city.ts`), until the You tab's settings give a member their own.

`ScreenFrame` (`src/components/screen-frame.tsx`) puts a screen's content below the top inset, as sign-in does; Android draws edge to edge and the title was under the status bar.

Screenshots of the tabs (Plans as the list, from a stand-in API that returns two plans; `plan-<language>-<theme>.png` is a plan's page and `plan-not-found-<language>-<theme>.png` a plan that is missing) and `plans-empty-…`, `plans-failed-…` and `plan-failed-…` the list with no plans, the list the API failed to fill and a plan page it failed to fill (web only: Android has one shot per screen, the empty and failed states on the emulator are the founder's call), at phone width, light and dark, English and Vietnamese, are in `docs/screenshots/` (`<tab>-<language>-<theme>.png`). The You tab's are of a dev build, so they show the API's version: the web image (`apps/web/Dockerfile`) built with `EXPO_PUBLIC_API_URL` set to its own origin and run with `API_ORIGIN` pointing at a local API, as on dev, and captured with Playwright's Chromium at 390×780, device scale 2, with the language and colour scheme set. A production build shows the tab without that line.

The sign-in screens (`sign-in-email`, `sign-in-code`, `sign-in-wrong-code`) and the signed-in You tab (`you-signed-in`) are in `docs/screenshots/` as `<state>-en-<theme>.png` from the web at 390 wide, and `<state>-android.png` from the emulator. They were taken against a local stand-in for Supabase Auth and the API (a throwaway script, not in the repo: code `123456` right, any other wrong), with `EXPO_PUBLIC_SUPABASE_URL` and `EXPO_PUBLIC_API_URL` pointed at it. The iOS simulator's are not attached: `make native-ios` still fails in `expo-modules-jsi` ([#631](https://github.com/shrujancheruku/issho/issues/631)). The same states in Vietnamese are `<state>-vi-<theme>.png`. `together-<language>-<theme>.png` are of a signed-in member (a session in the browser's storage, against the same stand-in), since Together is behind sign-in.

## Join links

Sign-in is a 6-digit email code and a join link is the only way in ([People](../../docs/spec/people.md#join-links)). A person without one can read plans and nothing else.

- **Arriving.** A link opens `/join/<code>` (`src/app/join/[code].tsx`), which moves the code into memory (`src/join/pending-join.ts`) and replaces the address with `/join` (`src/app/join/index.tsx`), so the code is not left in the URL bar or history. `/join` signs a signed-out person in first (`/sign-in?next=/join`: the code is not in the return path) and then makes their account (redeeming needs it) and redeems the link. A used, expired, cancelled or made-up code reads as one plain message (the API answers alike for all of them), and someone already in is told the link was not used. The code is forgotten as soon as the API has answered either way, and kept only when no answer came, so "Try again" works. It is never logged. The code lives in memory only: if the app is closed or the page reloaded during sign-in, `/join` with no code says "We no longer have your join link. Open your join link again." with a way on to the plans.
- **The deep link.** On the development build a link is `issho://join/<code>` (the app's `scheme` in `app.json`; `src/join/link-url.ts`). The universal link and app link for the real domain, which open the app from a message without a scheme, need the domain's association files and are the release milestone's; the web address `/join/<code>` works on the web build now.
- **The You tab** shows the member's links (`src/components/join-links.tsx`): the 3 in all, which are unused, used or expired, and how many can still be made. A member can make one, share it (the phone's share sheet; on the web the browser's, else the link is copied) and cancel one that is unused. A used link does not return to the 3; an expired or cancelled one does ([#609](https://github.com/shrujancheruku/issho/issues/609)).
- **Not admitted.** `useSignUp()` and `<SignUpRefusal>` (`src/join/sign-up.tsx`) are what a signup does with the API's `SignupProblem` `NOT_ADMITTED`: the person is told they need a join link. The signup screen itself is [#616](https://github.com/shrujancheruku/issho/issues/616); it calls `useSignUp().signUp` and shows `<SignUpRefusal refusal={refusal} />`.
- Screenshots: `join-admitted`, `join-unusable`, `join-already-in`, `join-gone`, `join-failed`, `you-join-links`, `you-join-links-failed` and `you-join-links-none-left` as `<state>-<language>-<theme>.png` from the web at 390 wide, and `join-admitted-android.png`, `join-unusable-android.png` and `you-join-links-android.png` from the emulator, opened by `adb shell am start -a android.intent.action.VIEW -d issho://join/<code>`. They were taken against a throwaway local stand-in for Supabase Auth and the API (not in the repo: code `123456` right). The iOS simulator's are not attached ([#631](https://github.com/shrujancheruku/issho/issues/631)). The not-admitted refusal has no screen yet (the signup screen is #616); a screen test proves it.
- No design for these screens exists on [#607](https://github.com/shrujancheruku/issho/issues/607): they use the existing components and tokens.

## Signing up with times

A plan's page has the sign-up action for a signed-in person ([#616](https://github.com/shrujancheruku/issho/issues/616); the sheet is `src/components/signup-panel.tsx`, drawn inside the page rather than over it).

- **The order.** Signed in (a visitor gets "Sign in to sign up", which returns to the plan), then the profile (`useProfileGate`: a person with none is sent to the questions and returns to this plan to press sign up again), then the times, then `useSignUp().signUp`, whose `<SignUpRefusal>` tells an account that is not admitted that it needs a join link. The API checks admission before the profile and the ticks, but only the signup call itself says so: there is no cheaper check, and a second entry point is not built.
- **The times.** The dates the server accepts, from 3 days ahead on the plan's city clock to 7 days after the round's last day (`signupDates`, `src/signup/times.ts`), each with only the parts of the day the plan allows (`Plan.parts_of_day`). The round's last day is `Plan.round_ends`, the same for every visitor, so it is in the cached page; the backend test `TestEveryDateTheAppOffersIsAcceptedAndTheDayAfterTheLastIsNot` (`backend/internal/plans/signups_test.go`) proves every offered date is accepted and the day after the last is refused. A cached page whose `round_ends` has passed offers today + 3 to today + 7, which is always accepted. The server doesn't prefill, so the app does: the ticks start from the person's last signup, the newest by `Signup.signed_at` (when it was made or last changed; `lastSignup`, which is not the last in `ListMySignups`'s earliest-hear-by order), where they fall in the new window on an allowed part; changing a signup starts from its own ticks. When the signups can't be loaded the action shows the page's own load-failed text with "Try again".
- **After signing up** the page says "You signed up. You will hear by ⟨date⟩.", how many are interested (the page's count is from the cached copy, so it is asked for again after a change), and offers "Change times" and "Withdraw".
- **Nothing per visitor is in the cached page.** The panel draws nothing until the stored session is read, and calls `ListMySignups` only for a signed-in person, in the browser (`src/signup/use-my-signups.ts`).
- **Together** lists each signup on the day it will hear (the round's last day) and each outing on its date, in time order (`src/together/items.ts`; `TogetherItem` is a union keyed by `kind`), each opening its plan. The invitations to answer are above them, in "Needs you" (below).
- **What opens first.** "With something in play, Together; with nothing, Plans" (`useOpenOnTogether`, `src/signup/use-open-on-together.ts`; in play is a signup, or an outing the person is still in). On a phone `src/app/index.tsx` applies it. On the web the server's 302 of `/` stays for everyone, because the server holds no token and cannot know who is looking; it adds `?open=1`, and the Plans screen, once the browser knows the person, moves one with something in play on to Together (`replace`, so Back does not return to the redirect) and drops the marker otherwise. A person who goes to `/plans` themselves is left there.
- **Dates** are the city's calendar dates ("YYYY-MM-DD"), written with `formatCalendarDate` (`src/lib/format.ts`), which goes through the one date formatter.
- No design for these screens exists on [#607](https://github.com/shrujancheruku/issho/issues/607): they use the existing components and tokens.
- Screenshots: `signup-signed-out`, `signup-action`, `signup-times`, `signup-times-none`, `signup-times-ticked`, `signup-signed-up`, `signup-change`, `signup-not-admitted` and `together-signup` as `<state>-<language>-<theme>.png` from the web at 390 wide, and `signup-*-android.png` from the emulator. They were taken against a throwaway local stand-in for Supabase Auth and the API (not in the repo). The iOS simulator's are not attached ([#631](https://github.com/shrujancheruku/issho/issues/631)).

## The outing

Together's "Needs you" block and the outing's page ([#619](https://github.com/shrujancheruku/issho/issues/619); [The app's shape](../../docs/spec/app-shape.md), [Outings](../../docs/spec/outings.md)). The API is `OutingService` (`ListMyOutings`, `AnswerInvitation`, `CancelSeat`); `CheckIn` and the feedback screens are [#621](https://github.com/shrujancheruku/issho/issues/621).

- **"Needs you"** sits above the list: each invitation still open (seat invited, outing offered or set, the 48 hours not over) is a card with the plan, the date and time, how many have said yes and the languages the group shares, answered Yes or No in place (`src/components/together-rows.tsx`). An invited person sees a count, never who. The Together tab carries the count as a badge.
- **The outing's page** is the plan's page with a private strip on top (`src/components/outing-strip.tsx`): the date and time, the languages, the first names of the others going (only to someone going), the stage and "Cancel my place", which asks first. The stage is the outing's state in the spec's words: proposed, offered, set, over, not set, cancelled. A seat that said no inside its 48 hours can say yes. The strip has a `children` slot at its end for check-in and feedback (#621).
- **Muted.** An outing that is over, not set or cancelled, or one the person is out of (declined, unanswered, full, cancelled, removed), is drawn at 60% opacity with its status written out, in Together's list and in the strip, with nothing to answer.
- **Time.** The date and time are the city's clock, with "Your time: …" beside them when the device reads differently (`formatOutingWhen`, `src/lib/format.ts`; the city's zone is the plan's `time_zone`).
- **Refusals.** The server answers a refused yes or cancel with FAILED_PRECONDITION and a message, with no structured reason, so the app reloads the outing and reads the cause from it (`refusalOf`, `src/outings/model.ts`): a full seat is "This group is full.", an invitation whose 48 hours ended, an outing not set or a seat unanswered is "This invitation has run out.", and a yes on a seat still open is "You are already going to another outing at that time…". A cancel after check-in says so. The refusal stays on the outing's card, row or strip until the next try.
- **Nothing per person is in the cached page.** The strip and the badge read `ListMyOutings` in the browser only for a signed-in person, as the signup panel does.
- No design for these screens exists on [#607](https://github.com/shrujancheruku/issho/issues/607): they use the existing components and tokens.
- Screenshots (`docs/screenshots/`): `outing-<state>-<language>-<theme>.png` from the web at 390 wide, `outing-together-needs-you-android.png` and `outing-invited-android.png` from the emulator, one per screen, from a throwaway local stand-in for Supabase Auth and the API (not in the repo). The iOS simulator's are not attached ([#631](https://github.com/shrujancheruku/issho/issues/631)).

## Testing

`make lint test` at the repo root runs everything below; `pnpm --filter @issho/app test` runs the tests alone.

| What | Where |
|---|---|
| The three tabs, Plans first, English and Vietnamese, light and dark, with a fake of the generated API client | `src/app/__tests__/tabs.test.tsx` |
| The sign-in screen shows the dev API's version to a visitor sent there from `/you` | `src/app/__tests__/sign-in.test.tsx` |
| The You tab shows the API's version, or that it could not be reached, in both languages, in dev's build and not in production's | `src/app/__tests__/api-version.test.tsx` |
| Which API addresses count as dev (`isDevApi`) | `src/api/__tests__/api-url.test.ts` |
| Sign-in: a build missing `EXPO_PUBLIC_SUPABASE_URL` or `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY` says at start which one (`SessionClient.missingConfig`); the title clears the status bar (top inset); email, then the 6-digit code; a wrong code says so and can be retried; Auth's rate limit (429) says to wait a minute; a new code can be asked for only after a minute; a return to the screen the person came from; Vietnamese | `src/app/__tests__/sign-in.test.tsx` |
| A signed-out visitor who opens You or Together is sent to sign-in and returned; Plans stays open; nothing is decided until the stored session is read | `src/app/__tests__/sign-in-guard.test.tsx` |
| The profile's questions in the spec's order (name, date of birth, gender with its reason, spoken languages, lives in Da Nang–Hoi An, code of conduct), the under-18 stop (the app's check and the API's `UNDER_AGE`), women-only offered only to woman and non-binary, a signup with no profile opened to them and returned to on finishing, the You tab's profile and change (no date of birth, and no code-of-conduct acceptance: it is shown, not sent), searching any language by name, labels and 44-point targets on every kind of control, the top inset, Vietnamese | `src/app/__tests__/profile.test.tsx` |
| The profile's rules: 18 on the day (29 February), a real date of birth, a name of 1 to 40 characters; the language search (CLDR names in the display language, accents and case ignored) | `src/profile/__tests__/answers.test.ts` |
| Where to go after signing in: in-app paths only, no whitespace or control character before the second slash | `src/auth/__tests__/return-path.test.ts` |
| The You tab shows the account's email and signs out | `src/app/__tests__/you-account.test.tsx` |
| Arriving by a join link: admits a signed-in person (account made first), signs a signed-out person in first and admits them after, keeps the code out of the address, the sign-in return path and the logs, says a used or expired link cannot be used, tells someone already in, retries when the check failed, says the link is gone when the code was lost, a member's own link reads as already in, Vietnamese | `src/app/__tests__/join.test.tsx` |
| The You tab's join links: listed by state with how many are left, make, share (share sheet, web copy), cancel, none left, not admitted, load failure, Vietnamese | `src/app/__tests__/you-join-links.test.tsx` |
| A signup refused as not admitted tells the person they need a join link (the entry point the signup screen uses) | `src/app/__tests__/sign-up-refusal.test.tsx` |
| The session is encrypted at rest on a phone, and unreadable if the key is gone | `src/auth/__tests__/encrypted-storage.test.ts` |
| A 401 from the API sends the person to sign-in, from any screen | `src/app/__tests__/you-account.test.tsx` |
| The Supabase adapter (code sent, code checked, session kept across a restart, sign-out) against a fake Auth server | `src/auth/__tests__/supabase-session-client.test.ts` |
| The Plans tab lists the city's plans (in the display language, with an empty and a failed state) and a plan opens its page with its stops and legs, price, meeting point and what to bring; a plan that doesn't exist says so; both start below the top inset | `src/app/__tests__/plans.test.tsx` |
| A page the Expo server already filled in is not fetched again, but is when the reader's language differs; the server's loaders ask the API in English with no token at `API_ORIGIN`, set `Cache-Control` (`no-store` when the API failed, for the list, the plan's head or its loader, in Expo's order; a plan page makes one call to the API), and answer a missing plan as not found | `src/app/__tests__/plans-server.test.tsx` |
| A plan page's `<head>`: title, description and link-card tags; `noindex` for a missing plan | `src/plans/__tests__/metadata.test.ts` |
| Signing up with times: the dates from 3 days ahead and only the plan's parts of the day, the profile first and a return to the plan, the ticks prefilled from the last signup, the times saved, "you signed up, hear by" and how many are interested, change and withdraw, not admitted, a visitor sent to sign in, nothing per visitor before the person is known; Together's signups in time order; what opens first (Together or Plans, and the server's `/plans?open=1`); Vietnamese | `src/app/__tests__/signup-times.test.tsx` |
| The signup window and the prefill | `src/signup/__tests__/times.test.ts` |
| The date and time formatter, price and time range | `src/lib/__tests__/format.test.ts` |
| Strings: language choice, `{placeholders}`, the device language | `src/i18n/__tests__/` |
| The strings check (`make lint`) | `scripts/__tests__/check-strings.test.js` |
| Lint fails on a string literal in a screen file | `src/__tests__/screen-lint.test.ts` |
| The native bundle fails on a web-only import (`make native-bundle-test`, a shell test, not Jest) | `scripts/native-bundle.test.sh` at the repo root (not `apps/app/scripts/`) |

Tests use Jest with `jest-expo` and React Native Testing Library. A screen test renders inside `TestProviders` (`src/test-support/providers.tsx`) with `fakeApi()` (`src/test-support/fake-api.ts`), a stand-in for `@issho/api-client` whose RPCs are `jest.fn`s, and `fakeSessionClient()` (`src/test-support/fake-session-client.ts`), a stand-in for Supabase Auth that accepts the code `123456`: no test reaches a network. `TestProviders` gives a test a signed-in person unless it passes a signed-out session client. Add a service's methods to the fake as screens start calling them.

## Layout

| Path | Holds |
|---|---|
| `src/app/` | The routes (expo-router). `index.tsx` redirects to `/plans`; `sign-in.tsx` is the email-and-code screen; `(tabs)/` is Together, Plans and You (Together and You wrap themselves in `RequireSignIn`; Plans is public) |
| `src/components/` | Shared pieces. Screens and components are "screen files" for the string lint |
| `src/i18n/` | `en.json`, `vi.json`, `strings.ts` (`translate`, `pickLanguage`), the provider and `useDeviceLanguage` |
| `src/lib/format.ts` | The one date, time and price formatter |
| `src/plans/` | What the Plans screens read: `load.ts` (the API calls, shared by the server and the client), `use-loaded.ts`, `server-data*.ts`, `server-loaders.ts`, `server-api.ts`, `metadata.ts`, `city.ts` |
| `src/constants/theme.ts`, `src/hooks/use-theme.ts` | Colour tokens and the theme in use |
| `src/hooks/use-color-scheme.ts` (and `.web.ts`) | The device's light or dark setting; the web version settles after hydration so the server render matches |
| `src/test-support/` | `providers.tsx` (`TestProviders`) and `fake-api.ts` (`fakeApi()`), shared by the screen tests |
| `src/types/` | Ambient declarations (`css.d.ts`, so the `.css` import type-checks) |
| `src/global.css` | Web-only global styles (`color-scheme`, so browser controls follow the theme) |
| `src/__tests__/` | Tests that read the source tree rather than render a screen (the string-literal lint) |
| `src/auth/` | Sign-in. `SessionClient` (`session-client.ts`) is the app's one view of Supabase Auth: `supabase-session-client.ts` implements it, `app-session-client.ts` builds the build's own from the `EXPO_PUBLIC_SUPABASE_*` values, and `session-storage.ts` keeps the session on a phone, encrypted in AsyncStorage with the key in `expo-secure-store` (`encrypted-storage.ts`; SecureStore alone holds about 2 KB, and `android.allowBackup` is `false` so a backup never carries the session; `.web.ts`: the browser's own storage). `src/api/app-api.ts` builds the API client: a 401 from the API ends the session, and the signed-in screens' guard sends the person to sign-in. `AuthProvider` and `useAuth()` hold the person's state: `loading` until the stored session is read, then `signedIn` or `signedOut`. `return-path.ts` accepts only an in-app `?next=` |
| `src/outings/` | The person's outings: `use-my-outings.tsx` (`MyOutingsProvider`, in the root layout, and `useMyOutings()`: `ListMyOutings` for a signed-in person on the client, one copy shared by the tab badge, Together and the plan's strip; `answer`, `cancel` and the refusals), `model.ts` (what needs an answer, what is muted, which outing a plan's page shows, and why a refusal was a refusal) and `text.ts` (the words for a state and a status). The pieces are `src/components/outing-parts.tsx`, `outing-strip.tsx` and `together-rows.tsx` |
| `src/signup/` | Signing up: `times.ts` (the offered dates on the city clock, the prefill), `use-my-signups.ts` (`ListMySignups` for a signed-in person, on the client), `use-open-on-together.ts`. The panel is `src/components/signup-panel.tsx`; `src/together/items.ts` is Together's rows |
| `src/join/` | Join links: `pending-join.ts` (the code in memory), `problems.ts` (reading the API's refusals), `link-url.ts`, `share.ts`, `sign-up.tsx` (`useSignUp`, `SignUpRefusal`). The routes are `src/app/join/` |
| `src/profile/` | The profile: `answers.ts` (the answers' shape, the genders and the short list of likely languages, 18-on-the-day), `languages.ts` and `language-names.ts` (every CLDR language, named in English and Vietnamese from a bundled table, since Hermes has no `Intl.DisplayNames`, for the search that lets any language be ticked), and `use-profile-gate.ts`, the entry point a signup calls: `const gate = useProfileGate(); if (!(await gate('/the-signup'))) return;` resolves true when the person has a profile; with none it opens `/profile?next=/the-signup` (`src/app/profile.tsx`, `src/components/profile-form.tsx`) and resolves false, and finishing returns the person to that path (`return-path.ts`). `src/components/profile-summary.tsx` is the You tab's profile |
| `src/api/` | `ApiProvider` and `useApi()`: the one generated client, given to screens; `api-url.ts`, the API's address and whether it is dev's |
| `assets/images/` | The icon, splash, favicon, logo and tab icons, ported from v1 (`apps/mobile` at `v1-final`) |
| `scripts/` | The strings check, the helper the lint test uses and `fixtures/native-bundle/` for the native bundle test (the native build scripts are in the repo's [`scripts/`](../../scripts/README.md)) |
| `docs/screenshots/` | Screenshots of the tabs (`<tab>-<language>-<theme>`) and of the native builds (`<name>-<platform>`) |

## The app's words

- Every string is a key in `src/i18n/en.json` and `src/i18n/vi.json` and reaches a screen through `useI18n().t('key')` ([Languages](../../docs/spec/languages.md)). The owner writes the English; Vietnamese is drafted and read before the Pilot.
- `node scripts/check-strings.js` (run by `make lint`) fails when a language lacks a key the other has, when a key appears nowhere in `src/`, and when a `t('…')` names a key that doesn't exist. A key passed as a prop (`titleKey="tabs.plans"`) counts as used.
- `expo lint` fails on a string literal in `src/app/` and `src/components/`: JSX text, a string in braces, and `title`, `label`, `placeholder`, `alt` and accessibility attributes. Pass a key through a prop named `…Key`.
- A language is added by its JSON file, an entry in `languages` (`src/i18n/strings.ts`) and one in `OTHER_LANGUAGES` (`scripts/check-strings.js`).
- On the web the server renders English and the browser's language follows right after hydration, so pages stay cacheable and hydration matches (`src/i18n/use-device-language.ts`).

## The web in production

`apps/web` serves the web build in a container, and CI deploys it to dev ([docs/deploy.md](../../docs/deploy.md)). Its image is built from this project with `EXPO_PUBLIC_API_URL` set to the address the page's own origin answers the API on. `make image` checks that the page is server-rendered: its words are in the HTML a client with no JavaScript gets.

## Native builds

`make native-bundle` (from the repo root; also run by `make lint`) exports the app for iOS and Android with `expo export` and fails on an import that resolves only on the web. `make native-bundle-test` (also run by `make test`) proves that failure with a fixture in `scripts/fixtures/native-bundle/`.

Local development builds, on the iOS simulator and the Android emulator, are for checking a screen on a phone before a ticket is done ([Native along the way](../../docs/spec/quality.md#native-along-the-way)). Nothing goes to EAS or a store. `ios/` and `android/` are generated and gitignored; nothing in them is committed.

| Command | What it does | Returns? |
|---|---|---|
| `make native-metro` | Starts Metro (Expo's dev server, port 8081) in the background, logging to `.expo/native-metro/metro.log`, and returns once it answers | yes |
| `make native-android` | `android emulator start $(NATIVE_AVD)` (default `issho`), then `expo run:android --no-bundler` builds, installs and opens the app | yes |
| `make native-ios` | `expo run:ios --no-bundler` generates `ios/`, runs `pod install`, builds and opens the app on the `NATIVE_IOS_DEVICE` simulator (default `iPhone 17`) | yes |
| `make native-wait` | Returns once a device has loaded the app's bundle; fails after `NATIVE_WAIT` seconds (default 180) with the log's last lines | yes |
| `make native-screenshot-android NAME=plans` | Captures the running emulator through the `android` CLI to `docs/screenshots/plans-android.png` | yes |
| `make native-screenshot-ios NAME=plans` | Captures the booted simulator to `docs/screenshots/plans-ios.png` | yes |
| `make native-stop` | Stops Metro, the `NATIVE_AVD` emulator and the `NATIVE_IOS_DEVICE` simulator, and leaves any other device running | yes |
| `make native-screenshot-test` | Checks that the screenshot and Metro scripts refuse bad arguments and that `native-wait` is bounded (also run by `make test`) | yes |

**A worker's sequence** (from the repo root; `make native-metro` first, so the app finds Metro when it opens):

```sh
make native-metro                      # Metro in the background; returns
make native-android                    # then, for iOS: make native-ios
make native-wait                       # the app has loaded its bundle
make native-screenshot-android NAME=plans
make native-stop                       # always, even when a step failed
```

Every command returns. Metro is the one long-running process, and `make native-metro` already starts it in the background: never run `expo start` or `expo run:*` without `--no-bundler` in the foreground and wait for it, because Metro keeps them alive for as long as it runs and the wait never ends. If a step fails, read `.expo/native-metro/metro.log`, run `make native-stop` and report.

Machine needs: full Xcode (`xcode-select -p` prints a path inside `Xcode.app`) with an iOS simulator runtime; the Android SDK (`ANDROID_HOME`) with platform `android-36`, build-tools `36.0.0`, the emulator package and the system image `system-images;android-36;google_apis;arm64-v8a` (`x86_64` on an Intel machine), JDK 17, and an AVD named `issho` made from that image (`android emulator list`). Gradle installs the platform Expo's template asks for on first build. CocoaPods (`pod` on the `PATH`), which Expo's iOS build runs. Expo's Swift Package Manager route (`experiments.swiftPackageManager`) is off: it can't build `expo-router` with `react-native-screens` yet ([#630](https://github.com/shrujancheruku/issho/issues/630)).

**iOS status: `make native-ios` does not build yet on Xcode 26.3 (Swift 6.2); [#631](https://github.com/shrujancheruku/issho/issues/631) owns it.** It generates `ios/`, runs `pod install` and compiles most pods, then fails in `expo-modules-jsi` 58.0.23, which Expo SDK 58 ships at its latest version:

```
expo-modules-jsi/apple/Sources/ExpoModulesJSI/Runtime/Values/JavaScriptObject.swift:946:36: error: sending 'slot' risks causing data races
xcodebuild exited with error code 65
```

The pod is built by a nested SwiftPM `xcodebuild` in Swift 6 language mode, so a Podfile or config-plugin setting can't change it from this repo. So `docs/screenshots/plans-ios.png` doesn't exist yet and `make native-screenshot-ios` is untested.

Metro's port, the simulator and the emulator are shared: two worktrees cannot run native builds at once.

## Dates and times

Use `formatDate`, `formatTime` and `formatDateTime` from `src/lib/format.ts`. They take the city's IANA time zone and the display language and never read the device's zone. Don't call `toLocaleString` or `Intl.DateTimeFormat` in a screen.

## Theme

Colours come from `useTheme()` (`src/constants/theme.ts`, light and dark) and follow the device setting. The person's own light/dark choice, fonts and the rest of v1's theme arrive with the screens that use them.

## Not here yet

The real screens, Making, push, the Sentry setup and EAS configuration.

