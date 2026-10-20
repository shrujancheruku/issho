# apps/mobile

The Issho iOS/Android app: **Expo SDK 58**, React Native 0.88, Expo Router, TanStack Query.

> Expo changes quickly between SDKs. Before touching Expo or React Native APIs, read [AGENTS.md](AGENTS.md) and the versioned docs at https://docs.expo.dev/versions/v58.0.0/.

## Run

From the repo root, first start the backend ([../../docs/onboarding.md](../../docs/onboarding.md) steps 3–4). Then:

```sh
cd apps/mobile
cp .env.example .env     # set EXPO_PUBLIC_API_URL
npx expo start           # i = iOS Simulator, a = Android emulator, scan QR = device (Expo Go / dev build)
```

| Variable | Example | Notes |
|---|---|---|
| `EXPO_PUBLIC_API_URL` | `http://localhost:8080` | Go API base URL. On a physical device, use your Mac's LAN IP. |

**Every `EXPO_PUBLIC_*` value is bundled into the app and is public.** Never put secrets here (see [../../docs/secrets.md](../../docs/secrets.md)).

Expo Go only includes bundled native modules. Once a library with native code is added (e.g. Apple sign-in in M2), use a development build: `npx expo run:ios`, or `npx eas-cli@latest build --profile development`.

## Structure

| Path | Contents |
|---|---|
| `src/app/` | **Routes only** (Expo Router file-based routing). `_layout.tsx` sets up providers (React Query, theme) and tabs. |
| `src/lib/api.ts` | The single API client instance (`createApi` from `@issho/api-client`). Auth token wiring arrives in M2. |
| `src/components/`, `src/hooks/`, `src/constants/` | UI building blocks, theme (from the Expo template) |
| `assets/` | Icons and splash images |
| `app.json` | App config: name, bundle IDs `com.joinissho.app`, scheme `issho`. `expo.version` is managed by release-please. |

## Calling the API

```tsx
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';

const activities = useQuery({
  queryKey: ['activities', 'tokyo', 'ja'],
  queryFn: () => api.catalog.listActivities({ cityId: 'tokyo', locale: 'ja' }),
});
```

Types come from the protos (`@issho/api-client`). After a proto change, run `make gen` at the root.

## Checks

```sh
npx tsc --noEmit      # typecheck (also runs in CI via `pnpm typecheck`)
npx expo-doctor       # dependency / config diagnostics
npx expo install <pkg>   # always use this to add packages (picks SDK-compatible versions)
```

## Builds & releases

M9 is planned to use EAS Build/Submit (TestFlight, Play internal testing). `ios/` and `android/` are generated (Continuous Native Generation) and gitignored. Never edit them by hand; configure native behaviour in `app.json` and config plugins.
