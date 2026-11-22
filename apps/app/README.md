# apps/app

The member app: one Expo project for iOS, Android and the web ([Architecture and stack](../../docs/spec/architecture.md#what-v2-is-built-from)). The web is the same project, server-rendered (`web.output: "server"` in `app.json`). Right now it is the shell (three tabs, the strings and the theme), sign-in by email code, and on the You tab the person's email, a sign-out button and the version the API reports (`src/components/api-version.tsx`), which is what the dev deploy checks. Members never see the version: it shows only when the build's `EXPO_PUBLIC_API_URL` is an API that holds no real people (`isDevApi` in `src/api/api-url.ts`: a local address, or a name whose first label is `dev` or ends in `-dev`, as `dev.joinissho.com` and `api-dev.joinissho.com` do). Any other address, production's included, shows nothing and makes no call. The real screens come in later tickets.

## Setup

From the repo root, once: `make setup` (installs the workspace with pnpm and enables the git hooks). Nothing here needs a secret.

`EXPO_PUBLIC_API_URL` is the Go API's address (default `http://localhost:8080`). Like every `EXPO_PUBLIC_*` value it is bundled into the app, so it is public.

Sign-in also needs the Supabase project's address and publishable key: `EXPO_PUBLIC_SUPABASE_URL` and `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY`. Both are public by design ([docs/secrets.md](../../docs/secrets.md)). A build without them still runs; asking for a code then says it could not be sent. The web image and the dev deploy do not pass them yet.

## Running

```sh
pnpm --filter @issho/app web      # the web, server-rendered, at http://localhost:8081
pnpm --filter @issho/app start    # Expo's dev server for a phone or simulator
```

Pass `--port <n>` after `expo start` to use another port. To see the server render, fetch a page without running any JavaScript: `curl localhost:8081/plans` holds the tab bar and "No plans yet." A visitor who opens `/` is sent to `/plans` by the browser (`src/app/index.tsx`).

Screenshots of every empty tab at phone width, light and dark, English and Vietnamese, are in `docs/screenshots/` (`<tab>-<language>-<theme>.png`). The You tab's are of a dev build, so they show the API's version: the web image (`apps/web/Dockerfile`) built with `EXPO_PUBLIC_API_URL` set to its own origin and run with `API_ORIGIN` pointing at a local API, as on dev, and captured with Playwright's Chromium at 390×780, device scale 2, with the language and colour scheme set. A production build shows the tab without that line.

The sign-in screens (`sign-in-email`, `sign-in-code`, `sign-in-wrong-code`) and the signed-in You tab (`you-signed-in`) are in `docs/screenshots/` as `<state>-en-<theme>.png` from the web at 390 wide, and `<state>-android.png` from the emulator. They were taken against a local stand-in for Supabase Auth and the API (a throwaway script, not in the repo: code `123456` right, any other wrong), with `EXPO_PUBLIC_SUPABASE_URL` and `EXPO_PUBLIC_API_URL` pointed at it. The iOS simulator's are not attached: `make native-ios` still fails in `expo-modules-jsi` ([#631](https://github.com/shrujancheruku/issho/issues/631)). The same states in Vietnamese are `<state>-vi-<theme>.png`.

## Testing

`make lint test` at the repo root runs everything below; `pnpm --filter @issho/app test` runs the tests alone.

| What | Where |
|---|---|
| The three tabs, Plans first, English and Vietnamese, light and dark, with a fake of the generated API client | `src/app/__tests__/tabs.test.tsx` |
| The You tab shows the API's version, or that it could not be reached, in both languages, in dev's build and not in production's | `src/app/__tests__/api-version.test.tsx` |
| Which API addresses count as dev (`isDevApi`) | `src/api/__tests__/api-url.test.ts` |
| Sign-in: email, then the 6-digit code; a wrong code says so and can be retried; Auth's rate limit (429) says to wait a minute; a new code can be asked for only after a minute; a return to the screen the person came from; Vietnamese | `src/app/__tests__/sign-in.test.tsx` |
| A signed-out visitor who opens You or Together is sent to sign-in and returned; Plans stays open; nothing is decided until the stored session is read | `src/app/__tests__/sign-in-guard.test.tsx` |
| The You tab shows the account's email and signs out | `src/app/__tests__/you-account.test.tsx` |
| The session is encrypted at rest on a phone, and unreadable if the key is gone | `src/auth/__tests__/encrypted-storage.test.ts` |
| A 401 from the API sends the person to sign-in, from any screen | `src/app/__tests__/you-account.test.tsx` |
| The Supabase adapter (code sent, code checked, session kept across a restart, sign-out) against a fake Auth server | `src/auth/__tests__/supabase-session-client.test.ts` |
| The date and time formatter | `src/lib/__tests__/format.test.ts` |
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
| `src/lib/format.ts` | The one date and time formatter |
| `src/constants/theme.ts`, `src/hooks/use-theme.ts` | Colour tokens and the theme in use |
| `src/hooks/use-color-scheme.ts` (and `.web.ts`) | The device's light or dark setting; the web version settles after hydration so the server render matches |
| `src/test-support/` | `providers.tsx` (`TestProviders`) and `fake-api.ts` (`fakeApi()`), shared by the screen tests |
| `src/types/` | Ambient declarations (`css.d.ts`, so the `.css` import type-checks) |
| `src/global.css` | Web-only global styles (`color-scheme`, so browser controls follow the theme) |
| `src/__tests__/` | Tests that read the source tree rather than render a screen (the string-literal lint) |
| `src/auth/` | Sign-in. `SessionClient` (`session-client.ts`) is the app's one view of Supabase Auth: `supabase-session-client.ts` implements it, `app-session-client.ts` builds the build's own from the `EXPO_PUBLIC_SUPABASE_*` values, and `session-storage.ts` keeps the session on a phone, encrypted in AsyncStorage with the key in `expo-secure-store` (`encrypted-storage.ts`; SecureStore alone holds about 2 KB, and `android.allowBackup` is `false` so a backup never carries the session; `.web.ts`: the browser's own storage). `src/api/app-api.ts` builds the API client: a 401 from the API ends the session, and the signed-in screens' guard sends the person to sign-in. `AuthProvider` and `useAuth()` hold the person's state: `loading` until the stored session is read, then `signedIn` or `signedOut`. `return-path.ts` accepts only an in-app `?next=` |
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
