# apps/mobile

The Issho iOS/Android app: **Expo SDK 58**, React Native 0.88, Expo Router, TanStack Query and Supabase Auth.

> Expo changes quickly between SDKs. Before touching Expo or React Native APIs, read [AGENTS.md](AGENTS.md) and the versioned docs at https://docs.expo.dev/versions/v58.0.0/.

## Run

First start the backend: steps 3–4 of [../../docs/onboarding.md](../../docs/onboarding.md) (`supabase start`, migrations, seed, API). Then:

```sh
cd apps/mobile
cp .env.example .env     # then fill in EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY from `supabase status`
npx expo start           # i = iOS Simulator (needs Xcode), a = Android emulator, QR code = phone with Expo Go
```

| Variable | Local value | Notes |
|---|---|---|
| `EXPO_PUBLIC_API_URL` | `http://localhost:8080` | Go API |
| `EXPO_PUBLIC_SUPABASE_URL` | `http://127.0.0.1:54321` | `API_URL` from `supabase status` |
| `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | `sb_publishable_…` | `PUBLISHABLE_KEY` from `supabase status`; public by design |

**EAS builds** don't see `.env` (it's gitignored): the `preview` and `production` environments on expo.dev hold `EXPO_PUBLIC_API_URL` (`https://api.joinissho.com`), `EXPO_PUBLIC_SUPABASE_URL` and `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY` (issho-dev's, for now) as plain-text variables.

**Every `EXPO_PUBLIC_*` value is bundled into the app and is public.** Never put secrets here (see [../../docs/secrets.md](../../docs/secrets.md)).

**On a physical phone**, `localhost` and `127.0.0.1` refer to the phone itself. Replace both hosts with your Mac's LAN IP (e.g. `http://192.168.1.20:8080` and `http://192.168.1.20:54321`). The phone and the Mac must be on the same network.

### Running on a device

The **App Store / Play Store builds of Expo Go can lag behind** a new SDK. On 2026-10-20 they still only ran SDK 57. The Expo CLI can install the matching Expo Go itself:

| Device | How | Needs |
|---|---|---|
| **Android phone (USB)** *(tested on a Pixel 8 Pro, Android 17)* | Turn on Developer options and **USB debugging**, connect with a data cable and accept the prompt. Then `npx expo start --android --lan`. The CLI installs the right Expo Go and opens the app. | `adb`: `brew install --cask android-platform-tools` (no Android Studio needed) |
| Android emulator | `npx expo start --android` | Android Studio with an emulator |
| iOS Simulator | `npx expo start --ios` | **Xcode 26.4+**, which needs **macOS 26.2+** |
| iPhone | `eas go` (your own Expo Go build on TestFlight), or a development build | Apple Developer Program membership |

Notes:
- Expo Go loads over the LAN, so set `EXPO_PUBLIC_API_URL` to the Mac's LAN IP (`ipconfig getifaddr en0`) and keep the phone on the same Wi-Fi.
- "Failed to resolve the Android SDK path" warnings are harmless when only `adb` is installed.
- If Expo Go opens to a **blank screen right after it was installed**, the CLI sent the project URL before the install finished. Reopen it: `adb shell am start -a android.intent.action.VIEW -d "exp://<mac-ip>:8081" host.exp.exponent`.
- Screenshots for bug reports: `adb exec-out screencap -p > screen.png`.

### Signing in locally

1. Enter any email address and tap **Send code**.
2. Open **Mailpit** at http://127.0.0.1:54324. Local Supabase never sends real email.
3. Type the 6-digit code. It submits automatically once all six digits are in.

A first sign-in creates the account. New users go through **onboarding** (languages, about you, area, availability, activities, kind of outing, code of conduct), saved in one `SaveProfile` at the end; the **Profile** tab edits each section afterwards and has **Sign out** and **Delete account**. Account deletion needs `SUPABASE_SECRET_KEY` set for the API (see [../../backend/README.md](../../backend/README.md)).

## How auth works

```
sign-in.tsx ── supabase.auth.signInWithOtp / verifyOtp ──▶ Supabase Auth
     │                                                         │ session (access + refresh token)
     ▼                                                         ▼
AuthProvider (features/auth) ◀── onAuthStateChange ── supabase client (lib/supabase.ts)
     │  session, onboarded (from IdentityService.GetMe)        │ storage: lib/secure-storage.ts (Keychain/Keystore)
     ▼                                                         │
app/_layout.tsx  Stack.Protected guards:                       │
  no session → sign-in │ not onboarded → onboarding │ else → (tabs)
                                                               │
lib/api.ts ── Authorization: Bearer <access token> ────────────┘──▶ Go API
```

- **Session storage** (`src/lib/secure-storage.ts`): the session lives in the OS secure store, never in plain storage. It's split into chunks because SecureStore values must stay under ~2 KB and a session is ~2.2 KB. Every write carries a generation ID, so a write interrupted halfway is detected and discarded; the user just signs in again.
- **Token refresh:** supabase-js refreshes while the app is in the foreground (`AppState` listener in `lib/supabase.ts`). `getSession()` refreshes an expired access token before each API call.
- **Route guards** are only for the user experience. The API checks the token on every request.
- **The cache is cleared** whenever the signed-in user changes, so one user never sees another's data.
- If the API rejects the session (`unauthenticated`, e.g. the account was deleted on another device), the app signs out automatically.
- **Suspended accounts:** `GetMe` says `suspended`, and the root layout shows only "Your account is suspended" with the account controls (`AccountActions`: sign out, delete account), since the API still allows deleting. If someone is suspended mid-session, their next call fails with `permission_denied` / `account suspended`; the query client's error handler (`app/_layout.tsx`) then reloads `GetMe`, which brings up the same screen.

## Structure

| Path | Contents |
|---|---|
| `src/app/` | **Routes only** (Expo Router). `_layout.tsx` sets up the providers and auth guards; `sign-in.tsx`, `onboarding.tsx`, `(tabs)/` for the main app (the plan feed, profile), `plan/[id].tsx` for one plan and signing up, and `profile-edit/[step].tsx` for editing one profile section. |
| `src/features/auth/` | `AuthProvider` / `useAuth()`: session state, onboarding status, display languages, sign-out |
| `src/features/profile/` | Onboarding and settings: `draft.ts` (the profile being edited, step rules; unit-tested), `steps.tsx` (one component per step), `language-picker.tsx`, `data.ts` (cities, activities, languages) |
| `src/features/plans/` | The feed and signups (`PlanService`): `data.ts` (feed, plan, sign up and withdraw, refreshing both), `names.ts` (plans carry ids; names come from the catalogue the app already loads), `plan-card.tsx`, `level.ts` (level labels and colours, matching the interest tiles), `signup.ts` (the times being edited ↔ the API's `Signup`; unit-tested), `format.ts` (dates) |
| `src/i18n/` | The app's UI text: `en.json` (the plain source), `human/` (people's versions of single strings), `i18n-provider.tsx` (`useI18n()`: `t`, `pair`, the language switch), `cache.ts` (translations kept on the device) |
| `src/features/account/` | Account actions (sign out, delete account) |
| `src/lib/` | `supabase.ts` (client), `secure-storage.ts` (session storage), `api.ts` (Go API client), `slots.ts` (weekly slots as strings) |
| `src/components/`, `src/hooks/`, `src/constants/` | UI building blocks and theme. Colours (`Colors` per theme, `Brand` fixed) and fonts are in `constants/theme.ts`, following [docs/design-system.md](../../docs/design-system.md): red is the logo's only, gold is the main button and the interest levels, the accent is selection. `availability-grid.tsx` is the week grid used by the profile and by signups. |
| `src/types/expo.d.ts` | Expo's ambient types, committed so fresh clones and CI can typecheck |
| `assets/images/logo.png` | The logo, rendered from `assets/brand/logo.svg` (`rsvg-convert -w 900 ../../assets/brand/logo.svg -o assets/images/logo.png`) |
| `app.json` | App config: name, bundle IDs `com.joinissho.app`, scheme `issho`. `expo.version` is managed by release-please. |

## Languages (i18n)

The app is written in English; every other language comes from the server (`UIService.GetStrings`), machine-translated once and cached for everyone, then kept on the device in `ui-strings-<lang>.json` (documents directory), so the next launch shows the right language immediately, even offline.

- **Which languages:** before sign-in, the device's most preferred language that the server supports (`expo-localization`, re-read when it changes; `i18n/device.ts` walks the whole preferred list, maps codes such as Filipino `fil` → `tl`, maps Taiwan, Hong Kong and Macau Chinese and Cantonese to `zh-Hant` (Traditional; `zh` is Simplified), and skips ones with no ISO 639-1 code such as Hawaiian). Onboarding starts with that as the main language and the device's next supported language, if any, as the second. Signed in, the profile's primary + optional secondary language, which `GetMe` returns. During onboarding, and while editing languages in settings, the screens preview the languages being chosen.
- **Writing UI text:** add the plain English string to `src/i18n/en.json` and use `const { t } = useI18n(); t('key', { name })`. Never hard-code UI text in components. Placeholders are `{camelCase}` only; for a count in a sentence, use a plural block, `{count, plural, one {Someone is in} other {# people are in}}` (translations bring their language's own forms), or a label ("Interested: {count}") that needs none. Run `make gen` afterwards: the API embeds a copy and only translates strings it knows (CI fails if the copy is stale). Write plain, unambiguous English: machine translation has no context ("Pick at least 3" became "tap at least 3 times" in Japanese until it said "Choose at least 3 activities").
- **Notes for translators:** when a string could be read two ways, or doesn't say what it is (a button, a tab, a status), add a note in `src/i18n/notes.json` ("Button: store the changes made on this screen (not saving money)"). Machines get it as context; `make gen` checks every note has a string.
- **Livelier English:** `src/i18n/human/en.json` replaces single strings for people using the app in English, while machines keep translating the plain text. Add `"key": {"text": "…"}` and run `make gen`, which checks it (the key exists, the placeholders match) and adds `"source"`, a hash of the plain English it was written for. Rewording a string in `en.json` makes its entry stale: tests fail until you check the livelier text, delete its `"source"` and run `make gen` again. Keep buttons, errors and safety or account text plain. Other languages' human versions (`human/<lang>.json`) come merged from the server; see `backend/internal/uistrings`.
- **Two languages:** `t()` returns the language currently shown; `LanguageSwitch` toggles it for longer text. Short labels show both side by side: `pair()` for UI text, or `name` + `secondaryName` from the API (e.g. activity tiles via `Chip secondary`).
- **Right to left:** the layout follows the primary language's direction (`i18n/direction.ts`): at once on the web; phones reload (expo-updates in release builds). **Layout in other languages:** use `start`/`end` styles, never left/right (lint enforces it), so right-to-left languages mirror. To see how longer text fits, start a dev build with `EXPO_PUBLIC_PSEUDO=1`: English becomes accented, ~40% longer and bracketed; text that isn't bracketed skipped `t()`.
- **Bundled languages:** the city packs' languages (now Japanese and Spanish) are bundled in `src/i18n/bundled/` (generated by `make ui-bundle API_URL=<api>`), so a fresh install shows them before its first fetch, even offline; the fetch then replaces them. Regenerate against the production API before each build.
- **Fallbacks:** a missing translation shows the other display language, then the plain English (`lookup` in `strings.ts`), never the livelier one: whoever gets the fallback may not read English well. Incomplete catalogues are refetched every 30 seconds while the app is open; complete ones are checked hourly and on returning to the foreground, sending their version so an unchanged catalogue costs no strings. That's how admin corrections and new human versions reach phones without a release.
- **Weekday names** come from `Intl.DateTimeFormat`, not the catalogue.
- **Sign-in emails** follow the same language: sign-up stores the screen's language as `user_metadata.lang`, and `AuthProvider` keeps it equal to the profile's main language (`supabase.auth.updateUser`). See `supabase/README.md`.

## Typography

- **Standout and hero text** (`ThemedText` types `title` and `subtitle`) uses **Dela Gothic One**, the face of the "ISSHO" wordmark (see `assets/brand`). It covers Latin and Japanese.
- Everything else (body text, buttons) uses **Zen Kaku Gothic New** in 400, 500 and 700. Each weight is its own family (`BodyFont`), because custom fonts ignore `fontWeight` on Android: `ThemedText` picks the family from the style's `fontWeight` (600 and up is bold) and drops the weight. Text outside `ThemedText` sets the family itself; every `TextInput` uses `BodyFont.regular`.
- The fonts come from `@expo-google-fonts/dela-gothic-one` and `@expo-google-fonts/zen-kaku-gothic-new` (OFL) and are loaded at startup with `useFonts` in `app/_layout.tsx`, which also works in Expo Go. The splash screen stays up until they're loaded. If loading fails, the app falls back to the system font. The three body weights add about 7 MB to the app.
- Use the `DisplayFont` constant (`constants/theme.ts`) rather than the family name. The face has a single weight, so **never set `fontWeight`** on display text: Android would fall back to the system font.

## Calling the API

```tsx
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';

const activities = useQuery({
  queryKey: ['activities', 'tokyo', 'ja'],
  queryFn: () => api.catalog.listActivities({ cityId: 'tokyo', locale: 'ja' }),
});
```

The access token is attached automatically. For error handling, `Code` and `ConnectError` are exported from `@issho/api-client`.

## Checks

```sh
npx tsc --noEmit          # typecheck   (CI: pnpm typecheck)
npx expo lint             # ESLint      (CI: pnpm lint)
npx jest                  # unit tests  (CI: pnpm test), e.g. src/lib/__tests__/secure-storage.test.ts
npx expo-doctor           # dependency / config diagnostics
npx expo install <pkg>    # always use this to add packages (picks SDK-compatible versions)
```

Tests use the `jest-expo` preset. TypeScript 6 no longer loads `@types` packages automatically, so test files start with `/// <reference types="jest" />`.

If installing a package fails with `ERR_PNPM_IGNORED_BUILDS`, approve or deny its install script in `pnpm-workspace.yaml` (see [onboarding troubleshooting](../../docs/onboarding.md#troubleshooting)).

## Web

The app also builds for the web (Expo Router + react-native-web) as a static single-page app. It isn't released yet; CI exports it on every PR so it keeps building. See the roadmap's "Web release" section for what's left before a launch.

```sh
npx expo start --web                 # dev server at http://localhost:8081
npx expo export -p web               # static build in dist/ (pnpm export:web)
```

- The API only accepts browser origins in its allowlist: start it with `ALLOWED_ORIGINS=http://localhost:8081` (see the backend README).
- Web-only code lives in `*.web.ts(x)` files, which Metro picks over the native ones: `lib/secure-storage.web.ts` (session in `localStorage`; read its comment before any public release), `i18n/cache.web.ts`, `components/app-tabs.web.tsx`, `hooks/use-color-scheme.web.ts`, `lib/push.web.ts` (no push in browsers; `expo-notifications` throws there), `features/plans/plans-map.web.tsx` (Google's web map instead of `react-native-maps`).
- `app.json` uses `"web": { "output": "single" }`: one `index.html` for every route, with no server rendering. Server rendering would run app code in Node at build time, where the secure store doesn't exist.
- **Wide screens:** form-like screens (sign-in, onboarding, profile editing, the language picker) keep their header, content and footer in one centred column at most `FormWidth` (560 px, `constants/theme.ts`) wide; the logo is capped too. Phones are narrower, so this only changes tablets and the web. `global.css` sets `color-scheme` so browser scrollbars follow the theme.
- **The floating tab bar:** on the web the tab bar sits over the top of the screen, so tab screens add `TopTabInset` (`constants/theme.ts`, 0 on phones) to their top padding. The feed uses `MaxContentWidth` (800 px) rather than `FormWidth`.
- `Alert.alert` does nothing on web: use `lib/dialogs.ts` (`confirm`, `notify`), which uses the browser's dialogs there.
- **Test deploy:** `scripts/deploy-web.sh <api-url> web` builds it and deploys it as a static-assets-only Cloudflare Worker, `issho-web` (`wrangler.jsonc`), at https://issho-web.shrujancheruku.workers.dev, against `issho-dev`. Unknown paths serve `index.html`. `public/_headers` sets basic security headers only: a full Content-Security-Policy is part of the session-security decision before release. Not linked anywhere and not a release; see [apps/admin/README.md](../admin/README.md#deploy) for running the API behind a tunnel.

## Push notifications

The server sends notifications through Expo's push service when a plan someone signed up for gets a date, and when their group goes ahead or is cancelled (backend: `app.notifications`, sent by the worker every minute). `lib/push.ts`:

- registers the device's Expo push token with `PushService` when the main app opens, **only if permission was already given**; it asks for permission right after the person signs up for a plan, when notifications are worth having
- unregisters it on sign-out, before the session ends
- opens the event when a notification is tapped (`useNotificationTaps`, also for the one that launched the app)
- does nothing in Expo Go (no remote push on Android since SDK 53), simulators, the web (`lib/push.web.ts`) or without an EAS project id, so the app works the same without push

**One-time setup for Android** (owner; uploads credentials to Expo, so it's done by hand). Already in the repo: the EAS project (`owner` and `extra.eas.projectId` in `app.json`), `eas.json` (a `development` profile that builds an installable APK with `expo-dev-client`) and `app.config.js`, which points the Android build at `google-services.json` without committing it.

1. **Register the app with Firebase.** In the [Firebase console](https://console.firebase.google.com), open the project → *Add app* → Android. Package name `com.joinissho.app`; the nickname is up to you; skip the SHA-1 (FCM doesn't need it). Download `google-services.json` into `apps/mobile/` (it's gitignored: the repo is public). Skip the "Add Firebase SDK" steps; `expo-notifications` does that.
2. **Give EAS Build the same file**, as a file environment variable (`app.config.js` reads it):
   ```sh
   cd apps/mobile
   npx eas-cli@latest env:set --name GOOGLE_SERVICES_JSON --type file --value ./google-services.json \
     --visibility sensitive --environment development --environment preview --environment production
   ```
3. **Let Expo send through FCM.** Firebase console → *Project settings* → *Service accounts* → *Generate new private key*. That JSON is a **secret**: keep it out of the repo and delete it once uploaded. Upload it:
   ```sh
   npx eas-cli@latest credentials -p android
   ```
   Choose the `development` profile → *Google Service Account* → *Manage your Google Service Account Key for Push Notifications (FCM V1)* → *Set up a Google Service Account Key for Push Notifications (FCM V1)* → *Upload a new service account key* → pick the file. (If Firebase says the Cloud Messaging API is disabled, enable *Firebase Cloud Messaging API (V1)* under *Project settings* → *Cloud Messaging*.)
4. **Build and install a development build** (push doesn't work in Expo Go):
   ```sh
   npx eas-cli@latest build --profile development --platform android
   ```
   EAS prints a link and QR code to install the APK on the phone. The `development` profile builds only `arm64-v8a` (`ORG_GRADLE_PROJECT_reactNativeArchitectures` in `eas.json`), the CPU type of real modern phones: a much smaller APK and a faster compile, but it won't run on an Android emulator on an Intel Mac or an x86 emulator image; remove the variable for that. Expect about 20 minutes on the free plan (queue plus compile); only native changes need a new build. It contains the native code only: the JavaScript still comes from your machine, so start Metro with `pnpm start` (or `npx expo start`) and open the project from the development build instead of Expo Go. Rebuild only when native code changes (a new native package or a config plugin change).
5. **iOS:** deferred to M9 (needs the paid Apple Developer Program).
6. **Worker:** `EXPO_ACCESS_TOKEN` only if you turn on *Enhanced security for push notifications* in the Expo project settings.

Test it by signing up for a plan in the development build (allow notifications; the token appears in `app.push_tokens`), then scheduling a round for it: the worker sends the invitation within a minute. To check delivery alone, paste the token into Expo's [push notifications tool](https://expo.dev/notifications).

## Maps

The feed has a **List | Map** toggle. The map shows one pin per area with open plans, coloured by the best plan there (as the feed's labels) and showing how many; tapping a pin lists that area's plans below the map. It's Google Maps everywhere (Google's Places policy: venue data on a map must be on a Google map):

- **Phones:** `react-native-maps` with `PROVIDER_GOOGLE` (`features/plans/plans-map.tsx`). It's native code: a development build made before it was added doesn't have it, so the module is loaded defensively and the map says "not available" instead of crashing the feed. Expo Go includes it.
- **Web:** Google's `@vis.gl/react-google-maps` (`plans-map.web.tsx`).

**Keys (owner, one-time):** in the Google Cloud project, enable the **Maps SDK for Android** and the **Maps JavaScript API**, then create two keys:

1. **Android key**, restricted to *Android apps*: package `com.joinissho.app` and the SHA-1 of the signing certificate (`npx eas-cli@latest credentials -p android` shows it, per build profile), API restriction *Maps SDK for Android*. Store it as the `GOOGLE_MAPS_ANDROID_API_KEY` EAS variable (`env:set … --visibility plaintext`; it's in the app anyway, the restriction protects it); `app.config.js` passes it to the `react-native-maps` plugin. Then **build a new development build**, since the plugin and native module changed.
2. **Browser key**, restricted to *websites* (`http://localhost:8081/*` for development, plus `https://issho-web.shrujancheruku.workers.dev/*` for the test deploy), API restriction *Maps JavaScript API*. Put it in `apps/mobile/.env` as `EXPO_PUBLIC_GOOGLE_MAPS_WEB_API_KEY` (public by design). Optionally create a Map ID (Map management) for `EXPO_PUBLIC_GOOGLE_MAPS_WEB_MAP_ID`; without one the web map uses Google's `DEMO_MAP_ID`, fine for development.

iOS (Maps SDK for iOS key) waits for M9 with the rest of iOS. Mobile map loads are free; web map loads are billed after a monthly free allowance.

## Builds & releases

M9 is planned to use EAS Build/Submit (TestFlight and Play internal testing). `ios/` and `android/` are generated (Continuous Native Generation) and gitignored. Never edit them by hand; configure native behaviour in `app.json` and config plugins (e.g. `expo-secure-store` is registered there).

## Over-the-air updates

`expo-updates` is configured for EAS Update: `runtimeVersion` uses the fingerprint policy (an update only reaches builds with the same native code) and each build profile in `eas.json` has its own channel (`development`, `preview`, `production`). Nothing is published automatically. To ship JavaScript-only changes to a channel: `eas update --channel preview --message "…"` (and run `make ui-bundle` first if the UI text changed). The app also uses it to reload after a right-to-left switch.

## Error reporting

Sentry (`@sentry/react-native`, `src/lib/monitoring.ts`) is on only when `EXPO_PUBLIC_SENTRY_DSN` is set at build time (a DSN is public: it can only send reports); `EXPO_PUBLIC_SENTRY_ENVIRONMENT` names the build. It reports errors and render crashes, with no personal data and no performance tracing. Without the variable (development, tests, the web build today) nothing is sent. Source maps aren't uploaded yet: that needs the Sentry project and an auth token for EAS builds.
