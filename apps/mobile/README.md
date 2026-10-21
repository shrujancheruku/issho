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

## Structure

| Path | Contents |
|---|---|
| `src/app/` | **Routes only** (Expo Router). `_layout.tsx` sets up the providers and auth guards; `sign-in.tsx`, `onboarding.tsx`, `(tabs)/` for the main app (home, profile) and `profile-edit/[step].tsx` for editing one profile section. |
| `src/features/auth/` | `AuthProvider` / `useAuth()`: session state, onboarding status, display languages, sign-out |
| `src/features/profile/` | Onboarding and settings: `draft.ts` (the profile being edited, step rules; unit-tested), `steps.tsx` (one component per step), `language-picker.tsx`, `data.ts` (cities, activities, languages) |
| `src/i18n/` | The app's UI text: `en.json` (the source), `i18n-provider.tsx` (`useI18n()`: `t`, `pair`, the language switch), `cache.ts` (translations kept on the device) |
| `src/features/account/` | Account actions (sign out, delete account) |
| `src/lib/` | `supabase.ts` (client), `secure-storage.ts` (session storage), `api.ts` (Go API client) |
| `src/components/`, `src/hooks/`, `src/constants/` | UI building blocks and theme. `Brand` colours are in `constants/theme.ts`. |
| `src/types/expo.d.ts` | Expo's ambient types, committed so fresh clones and CI can typecheck |
| `assets/images/logo.png` | The logo, rendered from `assets/brand/logo.svg` (`rsvg-convert -w 900 ../../assets/brand/logo.svg -o assets/images/logo.png`) |
| `app.json` | App config: name, bundle IDs `com.joinissho.app`, scheme `issho`. `expo.version` is managed by release-please. |

## Languages (i18n)

The app is written in English; every other language comes from the server (`UIService.GetStrings`), machine-translated once and cached for everyone, then kept on the device in `ui-strings-<lang>.json` (documents directory), so the next launch shows the right language immediately, even offline.

- **Which languages:** before sign-in, the device's most preferred language that the server supports (`expo-localization`, re-read when it changes; `i18n/device.ts` walks the whole preferred list, maps codes such as Filipino `fil` → `tl`, and skips ones with no ISO 639-1 code such as Hawaiian). Onboarding starts with that as the main language and the device's next supported language, if any, as the second. Signed in, the profile's primary + optional secondary language, which `GetMe` returns. During onboarding, and while editing languages in settings, the screens preview the languages being chosen.
- **Writing UI text:** add the English string to `src/i18n/en.json` and use `const { t } = useI18n(); t('key', { name })`. Never hard-code UI text in components. Placeholders are `{camelCase}` only. Run `make gen` afterwards: the API embeds a copy and only translates strings it knows (CI fails if the copy is stale). Write plain, unambiguous English: machine translation has no context ("Pick at least 3" became "tap at least 3 times" in Japanese until it said "Choose at least 3 activities").
- **Two languages:** `t()` returns the language currently shown; `LanguageSwitch` toggles it for longer text. Short labels show both side by side: `pair()` for UI text, or `name` + `secondaryName` from the API (e.g. activity tiles via `Chip secondary`).
- **Fallbacks:** a missing translation shows the other display language, then English (`lookup` in `strings.ts`). Incomplete catalogues are refetched every 30 seconds while the app is open.
- **Weekday names** come from `Intl.DateTimeFormat`, not the catalogue.

## Typography

- **Standout and hero text** (`ThemedText` types `title` and `subtitle`) uses **Dela Gothic One**, the face of the "ISSHO" wordmark (see `assets/brand`). It covers Latin and Japanese.
- Everything else (body text, inputs, buttons) uses the system font, for readability.
- The font comes from `@expo-google-fonts/dela-gothic-one` (OFL) and is loaded at startup with `useFonts` in `app/_layout.tsx`, which also works in Expo Go. The splash screen stays up until it's loaded. If loading fails, the app falls back to the system font.
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
- Web-only code lives in `*.web.ts(x)` files, which Metro picks over the native ones: `lib/secure-storage.web.ts` (session in `localStorage`; read its comment before any public release), `i18n/cache.web.ts`, `components/app-tabs.web.tsx`, `hooks/use-color-scheme.web.ts`.
- `app.json` uses `"web": { "output": "single" }`: one `index.html` for every route, with no server rendering. Server rendering would run app code in Node at build time, where the secure store doesn't exist.
- `Alert.alert` does nothing on web, so confirmation dialogs (account deletion) need a web version before release.

## Builds & releases

M9 is planned to use EAS Build/Submit (TestFlight and Play internal testing). `ios/` and `android/` are generated (Continuous Native Generation) and gitignored. Never edit them by hand; configure native behaviour in `app.json` and config plugins (e.g. `expo-secure-store` is registered there).
