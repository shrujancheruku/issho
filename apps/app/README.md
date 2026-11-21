# apps/app

The member app: one Expo project for iOS, Android and the web ([Architecture and stack](../../docs/spec/architecture.md#what-v2-is-built-from)). The web is the same project, server-rendered (`web.output: "server"` in `app.json`). Right now it is the shell: three empty tabs, the strings and the theme. Sign-in and the real screens come in later tickets.

## Setup

From the repo root, once: `make setup` (installs the workspace with pnpm and enables the git hooks). Nothing here needs a secret.

`EXPO_PUBLIC_API_URL` is the Go API's address (default `http://localhost:8080`). Like every `EXPO_PUBLIC_*` value it is bundled into the app, so it is public.

## Running

```sh
pnpm --filter @issho/app web      # the web, server-rendered, at http://localhost:8081
pnpm --filter @issho/app start    # Expo's dev server for a phone or simulator
```

Pass `--port <n>` after `expo start` to use another port. To see the server render, fetch a page without running any JavaScript: `curl localhost:8081/plans` holds the tab bar and "No plans yet." A visitor who opens `/` is sent to `/plans` by the browser (`src/app/index.tsx`).

Screenshots of every empty tab at phone width, light and dark, English and Vietnamese, are in `docs/screenshots/` (`<tab>-<language>-<theme>.png`).

## Testing

`make lint test` at the repo root runs everything below; `pnpm --filter @issho/app test` runs the tests alone.

| What | Where |
|---|---|
| The three tabs, Plans first, English and Vietnamese, light and dark, with a fake of the generated API client | `src/app/__tests__/tabs.test.tsx` |
| The date and time formatter | `src/lib/__tests__/format.test.ts` |
| Strings: language choice, `{placeholders}`, the device language | `src/i18n/__tests__/` |
| The strings check (`make lint`) | `scripts/__tests__/check-strings.test.js` |
| Lint fails on a string literal in a screen file | `src/__tests__/screen-lint.test.ts` |
| The native bundle fails on a web-only import (`make native-bundle-test`, a shell test, not Jest) | `scripts/native-bundle.test.sh` |

Tests use Jest with `jest-expo` and React Native Testing Library. A screen test renders inside `TestProviders` (`src/test-support/providers.tsx`) with `fakeApi()` (`src/test-support/fake-api.ts`), a stand-in for `@issho/api-client` whose RPCs are `jest.fn`s: no test reaches a network. Add a service's methods to the fake as screens start calling them.

## Layout

| Path | Holds |
|---|---|
| `src/app/` | The routes (expo-router). `index.tsx` redirects to `/plans`; `(tabs)/` is Together, Plans and You |
| `src/components/` | Shared pieces. Screens and components are "screen files" for the string lint |
| `src/i18n/` | `en.json`, `vi.json`, `strings.ts` (`translate`, `pickLanguage`), the provider and `useDeviceLanguage` |
| `src/lib/format.ts` | The one date and time formatter |
| `src/constants/theme.ts`, `src/hooks/use-theme.ts` | Colour tokens and the theme in use |
| `src/api/` | `ApiProvider` and `useApi()`: the one generated client, given to screens |
| `assets/images/` | The icon, splash, favicon, logo and tab icons, ported from v1 (`apps/mobile` at `v1-final`) |
| `scripts/` | The strings check, the helper the lint test uses, `fixtures/native-bundle/` for the native bundle test, and the native build scripts (`native-prepare-ios.sh`, `native-screenshot.sh`) |
| `docs/screenshots/` | Screenshots of the tabs (`<tab>-<language>-<theme>`) and of the native builds (`<name>-<platform>`) |

## The app's words

- Every string is a key in `src/i18n/en.json` and `src/i18n/vi.json` and reaches a screen through `useI18n().t('key')` ([Languages](../../docs/spec/languages.md)). The owner writes the English; Vietnamese is drafted and read before the Pilot.
- `node scripts/check-strings.js` (run by `make lint`) fails when a language lacks a key the other has, when a key appears nowhere in `src/`, and when a `t('…')` names a key that doesn't exist. A key passed as a prop (`titleKey="tabs.plans"`) counts as used.
- `expo lint` fails on a string literal in `src/app/` and `src/components/`: JSX text, a string in braces, and `title`, `label`, `placeholder`, `alt` and accessibility attributes. Pass a key through a prop named `…Key`.
- A language is added by its JSON file, an entry in `languages` (`src/i18n/strings.ts`) and one in `OTHER_LANGUAGES` (`scripts/check-strings.js`).
- On the web the server renders English and the browser's language follows right after hydration, so pages stay cacheable and hydration matches (`src/i18n/use-device-language.ts`).

## Native builds

`make native-bundle` (from the repo root; also run by `make lint`) exports the app for iOS and Android with `expo export` and fails on an import that resolves only on the web. `make native-bundle-test` (also run by `make test`) proves that failure with a fixture in `scripts/fixtures/native-bundle/`.

Local development builds, on the iOS simulator and the Android emulator, are for checking a screen on a phone before a ticket is done ([Native along the way](../../docs/spec/quality.md#native-along-the-way)). Nothing goes to EAS or a store. `ios/` and `android/` are generated and gitignored; nothing in them is committed.

| Command | What it does |
|---|---|
| `make native-ios` | Prepares `ios/` for Swift Package Manager (`scripts/native-prepare-ios.sh`), then `expo run:ios` builds and opens the app on the `NATIVE_IOS_DEVICE` simulator (default `iPhone 17`) |
| `make native-android` | `android emulator start $(NATIVE_AVD)` (default `issho`), then `expo run:android` builds and installs the app on it |
| `make native-screenshot-ios NAME=plans` | Captures the booted simulator to `docs/screenshots/plans-ios.png` |
| `make native-screenshot-android NAME=plans` | Captures the running emulator through the `android` CLI to `docs/screenshots/plans-android.png` |
| `make native-screenshot-test` | Checks that the screenshot script refuses bad arguments (also run by `make test`) |

Machine needs: full Xcode (`xcode-select -p` prints a path inside `Xcode.app`) with an iOS simulator runtime; the Android SDK (`ANDROID_HOME`) with platform `android-36`, build-tools `36.0.0` and the emulator package, JDK 17, and an AVD named `issho` (`android emulator list`). Gradle installs the platform Expo's template asks for on first build. No CocoaPods is installed or used: the iOS build uses Swift Package Manager, which `experiments.swiftPackageManager` in `app.json` turns on.

Metro (Expo's dev server) listens on port 8081 during a native run. Two worktrees cannot run one at once, since they would share the simulator, the emulator and that port. Stop Metro (Ctrl-C) when a run is done.

Status: the Android build and screenshot work. The iOS build is blocked: `react-native-screens` and `@react-native-masked-view/masked-view` ship no `Package.swift`, scaffolding one for `@react-native-masked-view/masked-view` produces a target whose umbrella header layout SPM rejects (its `RNCMaskedView.xcodeproj` sits beside the public headers), and no committed fix exists yet.

## Dates and times

Use `formatDate`, `formatTime` and `formatDateTime` from `src/lib/format.ts`. They take the city's IANA time zone and the display language and never read the device's zone. Don't call `toLocaleString` or `Intl.DateTimeFormat` in a screen.

## Theme

Colours come from `useTheme()` (`src/constants/theme.ts`, light and dark) and follow the device setting. The person's own light/dark choice, fonts and the rest of v1's theme arrive with the screens that use them.

## Not here yet

Sign-in, the real screens, Making, push, the Sentry setup and EAS configuration.
