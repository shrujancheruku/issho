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
| `scripts/` | The strings check, the helper the lint test uses, and `fixtures/native-bundle/` for the native bundle test |
| `docs/screenshots/` | Screenshots of the tabs and of the native builds, named `<tab>-<language>-<theme>` or `<name>-<platform>` |

## The app's words

- Every string is a key in `src/i18n/en.json` and `src/i18n/vi.json` and reaches a screen through `useI18n().t('key')` ([Languages](../../docs/spec/languages.md)). The owner writes the English; Vietnamese is drafted and read before the Pilot.
- `node scripts/check-strings.js` (run by `make lint`) fails when a language lacks a key the other has, when a key appears nowhere in `src/`, and when a `t('…')` names a key that doesn't exist. A key passed as a prop (`titleKey="tabs.plans"`) counts as used.
- `expo lint` fails on a string literal in `src/app/` and `src/components/`: JSX text, a string in braces, and `title`, `label`, `placeholder`, `alt` and accessibility attributes. Pass a key through a prop named `…Key`.
- A language is added by its JSON file, an entry in `languages` (`src/i18n/strings.ts`) and one in `OTHER_LANGUAGES` (`scripts/check-strings.js`).
- On the web the server renders English and the browser's language follows right after hydration, so pages stay cacheable and hydration matches (`src/i18n/use-device-language.ts`).

## Native builds

Local development builds only: nothing goes to EAS or a store before the release milestone ([Native along the way](../../docs/spec/quality.md#native-along-the-way)). From the repo root:

| Command | What it does |
|---|---|
| `make native-bundle` | `expo export` for iOS and Android. Fails on an import that resolves only on the web. Also runs in `make lint`. |
| `make native-bundle-test` | Proves that failure with a fixture (`apps/app/scripts/fixtures/native-bundle/`). Also runs in `make test`. |
| `make native-ios` | Builds a development build (`expo run:ios`) and opens it on the booted iOS simulator |
| `make native-android` | Builds a development build (`expo run:android`) and opens it on the running Android emulator |
| `make native-screenshot-ios NAME=plans` | Captures the booted simulator to `docs/screenshots/plans-ios.png` |
| `make native-screenshot-android NAME=plans` | Captures the running emulator to `docs/screenshots/plans-android.png` |

`make native-ios` and `make native-android` generate `ios/` and `android/` and build the native projects; both folders, and what they hold (Pods, `.gradle`), are gitignored and not committed.

What a machine needs:

- **iOS:** full Xcode, not only the Command Line Tools (`xcode-select -p` must print a path inside `Xcode.app`), with an iOS simulator runtime. CocoaPods, which `expo run:ios` uses to install the pods. Open a simulator with `open -a Simulator` before the screenshot.
- **Android:** the Android SDK (`ANDROID_HOME` set) with platform-tools (`adb`), build-tools and the emulator package, a JDK, and an emulator image (AVD). Create one in Android Studio's Device Manager, then start it with `emulator -avd <name>` (`emulator -list-avds` lists them).

Expo's dev server (Metro) listens on port 8081 during a native run. Two worktrees cannot both run a native build at once, because they would share the simulator, the emulator and that port. Stop Metro (Ctrl-C in the terminal running it) when the run is done.

The failure of a missing tool is explicit: `make native-ios` stops with the Xcode error, and the screenshot script names what to install.

## Dates and times

Use `formatDate`, `formatTime` and `formatDateTime` from `src/lib/format.ts`. They take the city's IANA time zone and the display language and never read the device's zone. Don't call `toLocaleString` or `Intl.DateTimeFormat` in a screen.

## Theme

Colours come from `useTheme()` (`src/constants/theme.ts`, light and dark) and follow the device setting. The person's own light/dark choice, fonts and the rest of v1's theme arrive with the screens that use them.

## Not here yet

Sign-in, the real screens, Making, push, the Sentry setup and EAS configuration. Nothing here is sent to EAS or a store.
