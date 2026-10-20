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

### Signing in locally

1. Enter any email address and tap **Send code**.
2. Open **Mailpit** at http://127.0.0.1:54324. Local Supabase never sends real email.
3. Type the 6-digit code. It submits automatically once all six digits are in.

A first sign-in creates the account. New users land on the **onboarding** placeholder, which has **Sign out** and **Delete account**. Account deletion needs `SUPABASE_SECRET_KEY` set for the API (see [../../backend/README.md](../../backend/README.md)).

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
| `src/app/` | **Routes only** (Expo Router). `_layout.tsx` sets up the providers and auth guards; `sign-in.tsx`, `onboarding.tsx`, and `(tabs)/` for the main app. |
| `src/features/auth/` | `AuthProvider` / `useAuth()`: session state, onboarding status, sign-out |
| `src/features/account/` | Account actions (sign out, delete account) |
| `src/lib/` | `supabase.ts` (client), `secure-storage.ts` (session storage), `api.ts` (Go API client) |
| `src/components/`, `src/hooks/`, `src/constants/` | UI building blocks and theme. `Brand` colours are in `constants/theme.ts`. |
| `src/types/expo.d.ts` | Expo's ambient types, committed so fresh clones and CI can typecheck |
| `assets/images/logo.png` | The logo, rendered from `assets/brand/logo.svg` (`rsvg-convert -w 900 ../../assets/brand/logo.svg -o assets/images/logo.png`) |
| `app.json` | App config: name, bundle IDs `com.joinissho.app`, scheme `issho`. `expo.version` is managed by release-please. |

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

## Builds & releases

M9 is planned to use EAS Build/Submit (TestFlight and Play internal testing). `ios/` and `android/` are generated (Continuous Native Generation) and gitignored. Never edit them by hand; configure native behaviour in `app.json` and config plugins (e.g. `expo-secure-store` is registered there).
