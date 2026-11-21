# On-device translation for the chat translate toggle

Research for #572 (Architecture and stack). Checked 2026-11-21. Nothing here is a decision.

## The question

The member app is one Expo project (SDK 58, expo-router) on iOS, Android and the web (server-rendered React Native Web). The chat's translate toggle must translate on the device. No chat text may go to a server for translation, and the toggle is hidden wherever the device can't translate ([chat spec](../spec/chat.md#translation)). The first pair is Vietnamese <-> English. Japanese is likely next.

The current candidate is `@bsky.app/expo-translate-text`. The worry is that it hasn't been published for months.

Claims marked **(unverified)** could not be confirmed from a primary source.

## Short answer

- Every realistic native option comes down to two engines: **Apple's Translation framework** on iOS and **Google ML Kit Translation** on Android. Optionally ML Kit runs on iOS too. The npm packages differ only in how well they wrap them.
- `@bsky.app/expo-translate-text` is stale on npm (last publish 2026-03-06), but Bluesky's own app still ships it on Expo SDK 58. It lacks the per-language-pair availability check that the spec's "hide the toggle" rule needs. It also lacks the iOS 26 no-view session.
- The newest wrapper, `@hzblj/expo-translation`, covers the spec almost exactly, but it is four weeks old and has one maintainer.
- On the web, only desktop Chrome (138+) and Edge (148+) can translate on the device through a web API. Mobile browsers, Safari and Firefox have no such API. Shipping our own model in the browser is possible (Mozilla's Bergamot models are 17–32 MB per direction for vi/en) but costly.
- Running our own model inside the native app (Opus-MT, NLLB, Gemma) is possible but large: about 100 MB to 2.5 GB per direction. NLLB's licence forbids commercial use.

## 1. `@bsky.app/expo-translate-text` and its upstream

### Health

| | `@bsky.app/expo-translate-text` | `expo-translate-text` (upstream) |
| --- | --- | --- |
| Repo | [bluesky-social/expo-translate-text](https://github.com/bluesky-social/expo-translate-text), a fork of the upstream | [TomAtterton/expo-translate-text](https://github.com/TomAtterton/expo-translate-text) |
| First publish | 2026-02-24 | 2025-03-05 |
| Last publish | **0.2.9, 2026-03-06** | **0.3.0, 2026-07-02** (0.2.1 on 2026-05-09) |
| Last code commit | 2026-03-06 ("Ensure iOS returns sourceLanguage") | 2026-07-02 ("Expand native translation support and bump to 0.3.0") |
| Activity since | Only Dependabot PRs (#13–#22, 2026-05 to 2026-08), none merged | none |
| Issues | Disabled on the repo | 1 ever (#3, 16 KB pages, closed 2026-04-13) |
| Stars / weekly downloads | 3 / 8,911 | 41 / 3,116 |
| Licence | MIT | MIT |
| Peer deps | `expo`, `react`, `react-native`: `*` | `expo >=51`, `react-native >=0.74` |
| Built against | Expo 54 (devDependency) | Expo 57 (devDependency) |

Sources: `npm view <pkg> time` (2026-11-21); `gh api repos/<repo>/commits`; [npm downloads API](https://api.npmjs.org/downloads/point/last-week/@bsky.app/expo-translate-text).

### Does Bluesky still use it?

Yes. On 2026-11-21, [social-app `package.json`](https://github.com/bluesky-social/social-app/blob/main/package.json) depends on `"@bsky.app/expo-translate-text": "^0.2.9"` alongside `"expo": "58.0.23"` and `"react-native": "0.88.1"`. Its wrapper, [`src/lib/translation/index.tsx`](https://github.com/bluesky-social/social-app/blob/main/src/lib/translation/index.tsx), was last changed 2026-10-14. So the package builds and runs on Expo SDK 58 in a large production app. Bluesky's own notes in that file:

- Android only takes two-letter codes, so it strips regions (`en-US` -> `en`).
- On iOS it picks the regional English variant from the device locale, because missing language packs must be downloaded.
- When on-device translation fails, it falls back to opening Google Translate. That would send text off the device, which our spec forbids.
- Its gate is `IS_TRANSLATION_SUPPORTED = (iOS >= 18) || (Android API > 22)` ([`src/env/index.ts`](https://github.com/bluesky-social/social-app/blob/main/src/env/index.ts)). This is an OS-version check, not a per-pair check.

### New architecture

Expo SDK 55 dropped the legacy architecture. From SDK 55 the new architecture is always on ([Expo SDK 55 changelog](https://expo.dev/changelog/sdk-55), [Expo new architecture guide](https://docs.expo.dev/guides/new-architecture/)). Both packages are Expo Modules API modules, which run on the new architecture. Bluesky running it on SDK 58 is the practical proof.

### What it wraps (read from the source, 2026-11-21)

| | iOS | Android | Web |
| --- | --- | --- | --- |
| Engine | Apple Translation: `TranslationSession` from a hidden 1×1 SwiftUI view with `.translationTask` (iOS 18+). `translateSheet` uses `.translationPresentation` (iOS 17.4+). | ML Kit `com.google.mlkit:translate:17.0.3` plus `language-id:17.0.6` (both 16 KB page aligned) | none |
| Availability | `isTranslationSupported()` returns true on iOS 18+ | true if Google Play services is available | n/a |
| Podspec / min | iOS 15.1 deployment, features gated at runtime | minSdk 21 default | |

Gaps that matter for our spec:

- **No per-pair check.** Neither package calls Apple's `LanguageAvailability` or ML Kit's model listing, so the app can't hide the toggle for an unsupported pair before trying it.
- **No iOS 26 path.** Neither uses `TranslationSession(installedSource:target:)`, so every iOS translation mounts a hidden SwiftUI view.
- **One shared hosting controller.** `presentTranslationView` keeps a single `hostingController`. Two overlapping `translateTask` calls replace each other's view. Apple says a session used after its view disappears is a `fatalError` ([translationTask docs](https://developer.apple.com/documentation/swiftui/view/translationtask(_:action:))). Our toggle translates many messages at once, so we would have to send them as one batch call. (This reading of the code is ours, not a reported bug.)
- **No translator reuse on Android.** The fork creates and closes the ML Kit translators on every call.
- The upstream's 0.3.0 adds `prepareTranslation` (ask for the download ahead) and `preferredStrategy` (iOS 26.4 `highFidelity`). It drops the fork's `isTranslationSupported`. The two have diverged, and the fork has not taken 0.3.0.

## 2. Other wrappers

### Apple Translation and ML Kit together

| Package | Engines | Last release | Health | New arch / SDK 58 | Notes |
| --- | --- | --- | --- | --- | --- |
| [`@hzblj/expo-translation`](https://github.com/hzblj/expo-translation) | iOS: Apple Translation, plus ML Kit for missing languages when the config plugin turns it on. Android: ML Kit. | 0.1.4, 2026-10-27 (first 2026-10-24) | 1 maintainer, 0 stars, 164 downloads/week; Swift and Kotlin tests in repo | Expo Modules API; peer `expo >=57`; built on 57. An outside PR, [#5 "Use the public Expo SDK 58 shared-object hook"](https://github.com/hzblj/expo-translation/pull/5), is open since 2026-11-20 | Per-pair `getStatusAsync` (installed / downloadable / unsupported), `getAvailabilityAsync`, iOS 26 `installedSource` session with no view, 26.4 `highFidelity`, batching, a cache, hooks, language identification, Apple's sheet, ML Kit model download/delete. Config plugin only for ML Kit on iOS. Dev build needed. MIT. |

Source: its [README](https://github.com/hzblj/expo-translation/blob/main/packages/expo-translation/README.md) and source (`ios/Core/AppleEngine.swift` uses `TranslationSession(installedSource:target:)` under `@available(iOS 26.0, *)`), checked 2026-11-21.

### Apple Translation only

No other React Native or Expo wrapper of Apple's Translation framework turned up. We searched npm for "translation framework", "TranslationSession", "apple translation react-native" and "expo translation" on 2026-11-21. `@react-native-ai/apple` wraps Apple's Foundation Models (an LLM), not the Translation framework.

### ML Kit only (both platforms)

| Package | Last release | Health | New arch / Expo | Notes |
| --- | --- | --- | --- | --- |
| [`@react-native-ml-kit/translate-text`](https://github.com/a7med-mahmoud/react-native-ml-kit) | 0.5.0, 2025-09-01 | Monorepo, 588 stars, 25 open issues/PRs. [#85 "Untested on New Architecture"](https://github.com/a7med-mahmoud/react-native-ml-kit/issues/85) open; a new-arch PR (#86) open since 2026-04 | Old-style bridge module. Runs through the interop layer **(unverified)**. No config plugin; needs a dev build. | iOS 15.5, `GoogleMLKit/Translate 8.0.0`; Android `translate:17.0.3`. 1,309 downloads/week. MIT. |
| [`fast-mlkit-translate-text`](https://github.com/yaaliuzhipeng/fast-mlkit-translate-text) | 1.0.3, 2025-11-30 | 1 maintainer, 40 stars, 13,651 downloads/week. "Expo Integration" issue #2 open since 2024-02 | README claims new-arch support | 16 KB fix in 1.0.3. MIT. |
| [`react-native-mlkit-translation`](https://github.com/luongminh2410dev/react-native-mlkit-translation) | 1.0.0, 2026-11-03 (first 2026-11-02) | 1 maintainer, 0 stars, 33 downloads/week | TurboModule, RN >= 0.76, new arch only; dev build | iOS 15.5, Android API 24. MIT. |
| [`@nitro-mlkit/translation`](https://github.com/pologonzalo/react-native-nitro-mlkit) | 0.1.0-beta.0, 2026-07-18 | 1 maintainer, beta, 15 downloads/week | Nitro modules | MIT. |
| `expo-mlkit-translation` | 1.0.2, 2024-10-03 | Abandoned | | |
| `react-native-mlkit-translate-text` | 1.0.3, 2023-08-10 | Abandoned | | |
| Infinite Red [`react-native-mlkit`](https://github.com/infinitered/react-native-mlkit) | n/a | Active (SDK 58 upgrade PR #270 open) | | **Has no translation module.** Its modules are core, document scanner, face detection, image labeling, object detection and text recognition. |

Sources: `npm view`, `gh api repos/...`, npm downloads API, all 2026-11-21.

None of the ML Kit-only wrappers uses Apple's engine on iOS. On iOS they ship Google's models inside our app, and the ML Kit attribution rules apply there too (see §3).

## 3. The two engines

### Apple Translation framework

From Apple's docs (fetched as JSON from developer.apple.com, 2026-11-21):

| API | Since | What it does |
| --- | --- | --- |
| [`translationPresentation(isPresented:text:…)`](https://developer.apple.com/documentation/swiftui/view/translationpresentation(ispresented:text:attachmentanchor:arrowedge:replacementaction:)) | iOS 17.4 | The system translation popover, with optional "Replace with Translation" |
| [`TranslationSession`](https://developer.apple.com/documentation/translation/translationsession) via [`.translationTask`](https://developer.apple.com/documentation/swiftui/view/translationtask(_:action:)) | iOS 18.0 | A session handed to a SwiftUI view. `translate(_:)`, `translate(batch:)`, `translations(from:)`. Can ask the user to download languages. |
| [`LanguageAvailability`](https://developer.apple.com/documentation/translation/languageavailability) | iOS 18.0 | `status(from:to:)` gives `installed`, `supported` (downloadable) or `unsupported`. `supportedLanguages` lists them. |
| [`prepareTranslation()`](https://developer.apple.com/documentation/translation/translationsession/preparetranslation()) | iOS 18.0 | Asks permission to download a pair without translating |
| [`init(installedSource:target:)`](https://developer.apple.com/documentation/translation/translationsession/init(installedsource:target:)) | iOS 26.0 | A session with no view, for languages already installed. Throws if they aren't. Downloads still need a `.translationTask` session. |
| [`TranslationSession.Strategy`](https://developer.apple.com/documentation/translation/translationsession/strategy) `.highFidelity` / `.lowLatency` | iOS 26.4 | `highFidelity` uses Apple Intelligence's model, needs no download and supports more languages. It falls back to `lowLatency` on devices without Apple Intelligence. |

Pitfalls:

- **Before iOS 26 a session needs a SwiftUI view in a window.** From React Native that means a hidden hosting view, which every wrapper does. Apple: using a session after its view disappears, or after the configuration changes, is a `fatalError`.
- **Downloads are the user's choice.** The framework shows Apple's own download sheet. Language packs are system-wide, listed in Settings, and an app can't delete them ([`lowLatency` docs](https://developer.apple.com/documentation/translation/translationsession/strategy/lowlatency)). A first translation may therefore pop a system sheet. `prepareTranslation()` lets us ask at a moment we choose, such as when the toggle is turned on.
- **The simulator doesn't translate.** Apple's sample "runs only in macOS and on physical iOS devices" ([Translating text within your app](https://developer.apple.com/documentation/translation/translating-text-within-your-app)).
- **Privacy:** "All translations using the `TranslationSession` class are processed on the user's device. Apple may collect API usage and performance metrics including the app bundle ID and the original and translated language, but this data does not include the original or translated content" ([TranslationSession](https://developer.apple.com/documentation/translation/translationsession)). The privacy policy may want to say this.
- **Languages:** Apple's docs give no fixed list; `supportedLanguages` is read at run time. The Translate app's [App Store listing](https://apps.apple.com/us/app/translate/id1514844618) names 21 languages, including Vietnamese, Japanese, English (US) and English (UK), "supported in the most recent operating system". Hindi is new in iOS 27. That the framework offers the same set as the app is **(unverified)**.
- Apple's framework has no Czech, Slovak, Hungarian, Romanian or Greek ([hzblj README](https://github.com/hzblj/expo-translation/blob/main/packages/expo-translation/README.md); **(unverified)** against Apple). This is irrelevant for vi/en/ja but could matter for later cities.

### Google ML Kit Translation

From Google's docs (2026-11-21):

- 59 languages, including `vi`, `en` and `ja` ([language support](https://developers.google.com/ml-kit/language/translation/translation-language-support)).
- Runs on the device. It is "intended for casual and simple translations", and non-English pairs pivot through English, which "can affect quality" ([overview](https://developers.google.com/ml-kit/language/translation)). vi <-> en is direct; vi <-> ja pivots.
- Models are about 30 MB per language. Google recommends downloading on Wi-Fi. The API has `downloadModelIfNeeded`, list and delete calls ([iOS guide](https://developers.google.com/ml-kit/language/translation/ios)).
- Android: all ML Kit APIs need API 23+ ([guides](https://developers.google.com/ml-kit/guides)). The latest artifacts are `translate:17.0.3` and `language-id:17.0.6`, both from 2024-08-07 and 16 KB-aligned ([Google Maven metadata](https://dl.google.com/dl/android/maven2/com/google/mlkit/translate/maven-metadata.xml), [release notes](https://developers.google.com/ml-kit/release-notes)). No translation changes in 2025–2026.
- iOS: pod `GoogleMLKit/Translate`. The guide pins 8.0.0, and the 2025-06-27 release table lists GoogleMLKit 9.0.0 ([release notes](https://developers.google.com/ml-kit/release-notes)). The wrappers' iOS minimum is 15.5 **(unverified at Google)**. ML Kit frameworks have no arm64 simulator slice ([hzblj README](https://github.com/hzblj/expo-translation/blob/main/packages/expo-translation/README.md); **(unverified)**).
- **Attribution terms.** The [ML Kit translation guidelines](https://developers.google.com/ml-kit/language/translation/translation-terms) say apps "must comply with the Google Cloud Translation API attribution requirements". It may not be used on embedded devices (cars, TVs, speakers). The [Cloud Translation attribution page](https://docs.cloud.google.com/translate/attribution) requires:
  - Google attribution (the Google logo preferred, or a text badge) "adjacent any translation results"
  - a triggering button labeled "Translate with Google"
  - the app's description and help to say Google Translate powers the translation
  - machine-translated markup when indexable web pages show the output

  So every translated chat line, or the toggle, would carry Google branding wherever ML Kit is the engine. Whether Google enforces the Cloud rules on ML Kit in the same way is **(unverified)**.
- **Model downloads come from Google's servers.** The download reveals the language pair and the device's IP, but not the message text. That is still on-device translation under our spec.
- The spec says "using the system's own translation". On Android, ML Kit is a Google library inside our app with models it downloads. It is not an OS service. Whether that counts as "the system's own" is a wording question for the spec.

### ML Kit on iOS instead of Apple Translation

ML Kit's iOS SDK would give iPhones older than iOS 18 a translate toggle. The cost is a bigger app, the attribution rules, Google's models on iOS, and the arm64-simulator issue. `@hzblj/expo-translation` does this as an opt-in config plugin, so Apple's engine is still used first. How many target users are below iOS 18 is not known here.

## 4. Writing our own Expo module

Native surface needed:

- **iOS (Swift):**
  - `LanguageAvailability.status(from:to:)` to decide whether the toggle shows
  - `TranslationSession(installedSource:target:)` on iOS 26+ for installed pairs
  - a hidden `UIHostingController` with `.translationTask` for iOS 18–25 and for any download
  - `prepareTranslation()` when the toggle is turned on
  - `translate(batch:)` for a screen of messages
- **Android (Kotlin):**
  - ML Kit `Translator` kept open per pair
  - `RemoteModelManager` for listing, downloading and deleting models
  - `DownloadConditions` (Wi-Fi)
  - optionally `LanguageIdentification` when a message's language is unknown
- **Web:** a `.web.ts` file that uses `Translator` where it exists (see §6) and reports "unavailable" otherwise.

Rough size, judged from the existing code (2026-11-21):

- The Bluesky fork is about 390 lines of Swift and 380 of Kotlin, plus TypeScript.
- `@hzblj/expo-translation`, with both engines on iOS, caching and hooks, is about 1,600 lines of Swift and Kotlin plus 600 of TypeScript, without tests.

A module scoped to our needs (one pair at a time, batch, status, prepare, no ML Kit on iOS) would likely sit near the fork's size. Expect a few days of native work plus testing on real devices, since the iOS simulator can't translate **(estimate)**.

Pitfalls to design for:

- the `.translationTask` view lifetime and the `fatalError` rule before iOS 26
- concurrent calls, so queue them or use one session per pair
- the first-use download sheet on iOS and a 30 MB ML Kit download on Android
- region tags: Android wants `en`, iOS may need `en-US` / `en-GB`
- ML Kit drops line breaks, so translate line by line ([hzblj README](https://github.com/hzblj/expo-translation/blob/main/packages/expo-translation/README.md); **(unverified)**)
- the ML Kit attribution UI

An alternative to writing from scratch is to vendor (fork) one of the MIT packages into `apps/mobile/modules/`, the Expo local-module layout.

## 5. Running a translation model ourselves

| Model | Licence | Size per direction | vi / ja | Runtime from RN | Notes |
| --- | --- | --- | --- | --- | --- |
| Opus-MT / Marian ([`Helsinki-NLP/opus-mt-vi-en`](https://huggingface.co/Helsinki-NLP/opus-mt-vi-en), `opus-mt-en-vi`, `opus-mt-ja-en`, `opus-mt-en-jap`) | Apache-2.0 | ONNX int8 about 47 MB encoder + 54 MB decoder, so about 100 MB ([Xenova/opus-mt-vi-en files](https://huggingface.co/Xenova/opus-mt-vi-en/tree/main/onnx)) | vi yes, ja yes | `onnxruntime-react-native` 1.24.3 (2026-07-16) plus a SentencePiece tokenizer and beam search we write | Two models per pair (one each way). Quality for chat Vietnamese **(unverified)**. |
| Mozilla Bergamot (Firefox Translations) | Engine MPL-2.0; model licence **(unverified)** | en->vi 31.6 MB, vi->en 17.1 MB, en->ja 43.8 MB, ja->en 44.0 MB ([model registry JSON](https://storage.googleapis.com/moz-fx-translations-data--303e-prod-translations-data/db/models.json), generated 2026-11-21) | vi yes, ja yes, all marked "Release" | No maintained RN binding. The C++ engine is `bergamot-translator`; npm `@browsermt/bergamot-translator` was last published 2022 and there are forks. `@qvac/translation-nmtcpp` (Tether, Marian-based, Apache-2.0) runs on its own Bare runtime **(unverified for Expo)**. | Smallest models. CPU only, built for this job. Flores COMET22: en->vi 0.871, vi->en 0.849. |
| NLLB-200 distilled 600M | **CC-BY-NC-4.0 (no commercial use)** | about 900 MB int8 ([files](https://huggingface.co/Xenova/nllb-200-distilled-600M/tree/main/onnx)) | yes | ONNX | The licence rules it out. |
| TranslateGemma 4B (Google, January 2026) | Gemma terms | Q4_K_M GGUF 2.49 GB ([GGUF](https://huggingface.co/bullerwins/translategemma-4b-it-GGUF)); LiteRT web task 3.9 GB | 55 benchmarked languages; Vietnamese on the list **(unverified)** | `llama.rn` 0.13.0-rc (active), `react-native-executorch` 0.11.1 (needs new arch, SDK 55+) | Gigabytes per user, so RAM, battery and download are all heavy. |
| Gemini Nano (Android AICore / ML Kit GenAI) | Google terms | Built into the OS | **(unverified)** | ML Kit GenAI | [ML Kit GenAI](https://developers.google.com/ml-kit/genai) has no text-translation API. We would prompt the general model, and only on Gemini Nano devices. |
| Apple Foundation Models | Apple terms | Built into the OS | Vietnamese and Japanese are Apple Intelligence languages in iOS 27 ([Apple](https://support.apple.com/en-us/121115)) | Swift module, or `@react-native-ai/apple` | iOS 26+, Apple Intelligence devices only (iPhone 15 Pro and later). Guardrails may refuse chat text. Apple's own route to "Apple Intelligence translation" is the Translation framework's `highFidelity` strategy (iOS 26.4). |

Battery and speed figures for any of these on a phone are **(unverified)**. App-size cost: models would be downloaded after install, not bundled, so the store binary grows only by the runtime. The ONNX runtime adds about 10–20 MB **(unverified)**.

## 6. The web

| Browser | On-device API | Source |
| --- | --- | --- |
| Chrome desktop | `Translator` and `LanguageDetector` since **Chrome 138**. "Availability may be subject to geographical restrictions." | [MDN browser-compat-data `api/Translator.json`](https://github.com/mdn/browser-compat-data/blob/main/api/Translator.json) |
| Chrome Android, all iOS browsers | **No.** "These APIs don't work on mobile devices." | [Chrome Translator API](https://developer.chrome.com/docs/ai/translator-api) (updated 2025-05-20) |
| Edge desktop | `Translator` since **Edge 148**, with a local, task-specific model | BCD; [Edge docs](https://learn.microsoft.com/en-us/microsoft-edge/web-platform/translator-api) (updated 2026-11-18) |
| Firefox, Safari | No web API | BCD |

Details for Chrome:

- The language list includes `vi` and `ja`.
- Pairs are reported as `downloadable` until the site creates one, for privacy. So we can't learn ahead whether a pair is installed, only that it is possible.
- `create()` needs a user activation if a download is required. Our toggle tap is one.
- It works in top-level windows and same-origin iframes only, not in workers. It needs a secure context ([MDN](https://developer.mozilla.org/en-US/docs/Web/API/Translator)).
- Expert models, not Gemini Nano. The 22 GB storage rule applies only to the Gemini Nano APIs ([Chrome get-started](https://developer.chrome.com/docs/ai/get-started)).
- With server-rendered React Native Web, the check must run on the client after hydration (`'Translator' in self`). The toggle stays hidden in server HTML.

WASM in any browser:

- **Bergamot.** Firefox's engine compiled to WASM, with the models above (17–32 MB per vi/en direction). Firefox itself ships on-device page translation this way, including on Android (arm64, Firefox 130+) ([Mozilla support](https://support.mozilla.org/kb/android-translation)). Using it in our web app means hosting the models ourselves and maintaining a WASM build.
- **transformers.js** (`@huggingface/transformers` 4.3.2, Apache-2.0) runs the Opus-MT ONNX models (about 100 MB per direction) on WASM or WebGPU. Speed on a phone **(unverified)**.

## 7. The system's own translation UI as a no-code fallback

| Platform | What exists | What the app must do |
| --- | --- | --- |
| iOS | The system Translate item in the text edit menu, and the translation sheet | React Native's `<Text selectable>` on the new architecture only offers **Copy**. `RCTParagraphComponentView` answers `canPerformAction` with YES only for `copy:` ([source](https://github.com/facebook/react-native/blob/main/packages/react-native/React/Fabric/Mounting/ComponentViews/Text/RCTParagraphComponentView.mm)). So there is no Translate item. Options: a long-press action that calls the sheet through a module (`translationPresentation`, iOS 17.4+; the Bluesky and hzblj packages have it), or render the message in a read-only `TextInput` / `UITextView`, whose system menu includes Translate **(unverified)**. Whether the sheet translates on the device is **(unverified)**: Apple's on-device statement covers `TranslationSession` only. |
| Android | Selection toolbar items from `PROCESS_TEXT` apps such as Google Translate | `<Text selectable>` uses a selectable `TextView`, which shows these items **(unverified on RN 0.88)**. The translating app is the user's choice, and it may use the network **(unverified)**. |
| Web | The browser's page translation | Nothing needed. Chrome's page translation is widely understood to use Google's servers **(unverified; Google's admin page lists only URLs and timestamps collected)**. Firefox's runs on the device. `translate="no"` can opt elements out. |

None of these gives the spec's inline "translation under the original". They are a manual fallback, and some of them leave the device.

## 8. Language coverage and minimum OS

| Method | en | vi | ja | Minimum |
| --- | --- | --- | --- | --- |
| Apple Translation | yes | yes | yes | iOS 17.4 (sheet), iOS 18 (session), iOS 26 (no-view session), iOS 26.4 (`highFidelity`). Real device only. |
| ML Kit (Android) | yes | yes | yes (pivots through English for vi<->ja) | API 23 (ML Kit). The wrappers ask 21–24. |
| ML Kit (iOS) | yes | yes | yes | iOS 15.5 per wrappers **(unverified)** |
| Chrome / Edge `Translator` | yes | yes | yes | Chrome 138 / Edge 148, desktop only |
| Bergamot models | yes | yes (both directions) | yes (both directions) | WASM-capable browser |
| Opus-MT | yes | yes | yes | Any, with ONNX runtime |
| Apple Foundation Models | yes | yes (iOS 27 list) | yes | iOS 26, Apple Intelligence device |

## Comparison

| Method | iOS | Android | Web | vi/en/ja | Maintenance health (2026-11-21) | Work for us | Risks |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `@bsky.app/expo-translate-text` | Apple, iOS 18+ | ML Kit | no | yes | Last publish 2026-03-06; no commits since; issues off; used in Bluesky prod on SDK 58 | Small: call it, write the web side and a pair check ourselves | No per-pair availability; no iOS 26 path; overlapping calls; it may never be updated; ML Kit attribution |
| `expo-translate-text` (upstream) | Apple, iOS 18+ (26.4 strategy) | ML Kit | no | yes | Last publish 2026-07-02; 1 maintainer; quiet | Same as above, plus our own `isSupported` | Same; no per-pair check; not proven on SDK 58 in public |
| `@hzblj/expo-translation` | Apple iOS 18+, iOS 26 no-view; optional ML Kit for older/missing | ML Kit | no | yes | Four weeks old; 1 maintainer; 0 stars; SDK 58 PR open | Smallest: per-pair status, prepare, batch, cache exist; we add the web side | Bus factor 1; young; SDK 58 not released yet; attribution on ML Kit paths |
| ML Kit-only wrappers (`@react-native-ml-kit/translate-text`, `fast-mlkit-translate-text`, `react-native-mlkit-translation`) | ML Kit on iOS (Google models in our app) | ML Kit | no | yes | Mixed: the big monorepo has new-arch untested; the others are single-maintainer | Small to medium | Google branding on iOS too; bigger iOS app; no arm64 simulator **(unverified)**; skips Apple's engine |
| Our own Expo module (Apple + ML Kit) | Apple, all paths | ML Kit | via `.web.ts` | yes | Ours | Medium: about 800 native lines, device testing | We own native code across iOS releases; Apple's view-lifetime rules |
| Fork or vendor a wrapper into `modules/` | as the source | as the source | via `.web.ts` | yes | Ours from day one | Small to medium | Same as own module, with a head start |
| Own model in the app (Opus-MT / Bergamot / Gemma) | yes | yes | yes (WASM) | yes | Runtimes active; Bergamot RN binding missing | Large: runtime, tokenizer, model hosting, downloads | 17 MB–2.5 GB per direction; battery and speed unknown; quality unknown |
| Gemini Nano / Apple Foundation Models | iOS 26, AI devices | Gemini Nano devices | no | partly unverified | OS vendors | Medium | Few devices; LLM guardrails and refusals; not a translation API |
| Chrome/Edge `Translator` | n/a | n/a | desktop Chrome 138+, Edge 148+ | yes | Shipped, still "experimental" on MDN | Small | Desktop only; no mobile web; "installed" can't be known ahead |
| System UI only | sheet via module, or a UITextView menu | selection toolbar | browser page translate | depends | n/a | Small | Not inline; some paths leave the device; RN's iOS `Text` has no Translate item |

## What the trade-offs come down to

1. **The engines are settled; the wrapper isn't.** On native, every serious option uses Apple Translation on iOS and ML Kit on Android. The choice is who owns the thin layer over them:
   - an abandoned-looking fork that a big app still ships (`@bsky.app`)
   - a complete but brand-new single-maintainer package (`@hzblj`)
   - our own or vendored module
2. **The "hide the toggle" rule needs a per-pair check.** Only `@hzblj/expo-translation`, or our own module, has one today. With either Bluesky package we would add `LanguageAvailability` and the ML Kit model checks ourselves, which is most of a module anyway.
3. **iOS 26 removes the hidden-view hack for installed pairs.** A wrapper that uses `init(installedSource:target:)` is simpler and safer on current iPhones. iOS 18–25 still needs the view, and so does any download.
4. **ML Kit brings Google's attribution rules.** Using it on Android (and on iOS if chosen) means Google branding next to translations and in the store text. Apple's engine has no such rule. Staying on Apple's engine on iOS keeps iOS free of it, at the cost of no toggle before iOS 18.
5. **The web is mostly "toggle hidden".** Only desktop Chrome and Edge can do it through a web API. Mobile web, Safari and Firefox would need us to ship a WASM engine and host models, about 50 MB for both vi/en directions with Bergamot. That is a real project, not a library call.
6. **Our own model is possible but heavy.** Bergamot's 17–32 MB models are the only small ones. They have no maintained React Native binding. Everything else is 100 MB to gigabytes per direction, or has a licence problem (NLLB).

## Unverified points

- `@react-native-ml-kit/translate-text` works through the new-architecture interop layer.
- The Apple Translation framework offers the same languages as the Translate app's 21. The languages Apple lacks (Czech, Slovak, Hungarian, Romanian, Greek) come from a third-party README.
- ML Kit iOS minimum is iOS 15.5; ML Kit iOS frameworks lack an arm64 simulator slice.
- ML Kit loses line breaks.
- Google applies the Cloud Translation attribution rules to on-device ML Kit in practice, as the terms say.
- TranslateGemma covers Vietnamese.
- Gemini Nano's language coverage for vi/ja.
- Model quality for chat-style Vietnamese (Opus-MT, Bergamot beyond Flores scores, ML Kit, Apple).
- Speed, battery and runtime size of ONNX Runtime, llama.rn or ExecuTorch on phones, and of transformers.js in a mobile browser.
- The licence of Mozilla's Bergamot models; `@qvac/translation-nmtcpp` working in an Expo app.
- Apple's translation sheet (`translationPresentation`) translating on the device.
- A read-only `TextInput` / `UITextView` on iOS showing the system Translate menu item; Android `<Text selectable>` showing `PROCESS_TEXT` translate items on RN 0.88; whether those apps translate on the device.
- Chrome's page translation sending text to Google's servers.
- The size estimate for our own module (about 800 native lines, a few days).
