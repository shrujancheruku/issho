# What the v2 spec asks of the stack

Research for [#571](https://github.com/shrujancheruku/issho/issues/571). It feeds [#572](https://github.com/shrujancheruku/issho/issues/572) (Architecture and stack), which makes the decisions. This note decides nothing.

- **Sources checked:** 2026-11-20, unless a line says otherwise. Each claim links its primary source.
- **Unverified:** marked **(unverified)**, and gathered at the end.
- **Not legal advice:** the law and terms-of-service parts are research, not legal advice.
- **Earlier notes:** this one builds on four earlier research branches and summarises them rather than repeating them:
  - [scraping-terms-and-law](https://github.com/shrujancheruku/issho/blob/research/scraping-terms-and-law/docs/research/scraping-terms-and-law.md) (2026-11-12)
  - [vietnam-phone-verification](https://github.com/shrujancheruku/issho/blob/research/vietnam-phone-verification/docs/research/vietnam-phone-verification.md) (2026-11-07)
  - [vietnam-data-localization](https://github.com/shrujancheruku/issho/blob/research/vietnam-data-localization/docs/research/vietnam-data-localization.md) (2026-11-07)
  - [store-paths-vietnam](https://github.com/shrujancheruku/issho/blob/research/store-paths-vietnam/docs/research/store-paths-vietnam.md) (2026-11-04)

**v1's stack:**

| Part | What v1 uses |
|---|---|
| API | Go, ConnectRPC |
| Sign-in and database | Supabase Auth (email OTP) and Postgres, in `ap-southeast-1` Singapore ([docs/supabase-hosted.md](../supabase-hosted.md)) |
| Member app | Expo SDK 58 for iOS, Android and web (`apps/mobile/package.json`: expo ~58.0.19, expo-router ~58.0.26) |
| Admin | a separate SPA (`apps/admin`) |
| Website | a static marketing site |
| Email | Resend SMTP |
| Push | Expo push |
| Error tracking | Sentry, EU region |

## Summary

| # | Ask | v1 meets it? | One line |
|---|---|---|---|
| 1 | One address (#558) | **Partly** | Expo SDK 58 can render pages on a server, with stable support and per-page metadata. v1 is a client-only app on another subdomain. A server is needed because plans change after build time, and one Expo project can't mix static and server rendering. |
| 2a | Email code | **Met** | Already in use. |
| 2b | Apple and Google | **Met (config work)** | Supabase `signInWithIdToken` works with the native Apple and Google SDKs. Web and Android Apple sign-in uses OAuth, which needs a new secret every 6 months. |
| 2c | Verified +84 phone | **Partly** | Supabase can add and verify a phone (`phone_change`) through Twilio Verify or the Send SMS hook. Vietnam requires a registered sender ID, which needs a business registration certificate. Whether Twilio Verify removes that need for Vietnam is **unverified**. Zalo ZNS is cheaper but needs a verified Official Account. |
| 2d | Admin second step | **Met (config work)** | TOTP MFA is free and on by default; the Go API checks `aal2`. Phone MFA is a $75/month add-on. Passkeys are experimental and first-factor only. |
| 3 | On-device translation | **Partly** | No first-party Expo module. A community module wraps Apple Translation (iOS 18+) and ML Kit (Android, `vi`/`en`). On the web only desktop Chrome 138+ and Edge 148+ have a Translator API. Safari, Firefox and all mobile browsers have none, so the toggle is hidden there, which #551 allows. |
| 4 | Push and email | **Met** | Expo push is free and wraps FCM and APNs. iOS needs the Apple Developer Program. Resend stores data in the US. Deliverability to Vietnamese inboxes is **unverified**. |
| 5a | Web search we may keep notes from | **Met by Anthropic web search** | Citations are required when output is shown to users, and there is no storage limit; $10 per 1,000 searches. Gemini Search grounding forbids building a store and is ruled out (#568). |
| 5b | MT that forbids training | **Met by all three** | Anthropic: "may not train models on Customer Content". Google Cloud Translation: "does not use the content you send to train". DeepL Pro: content stored only as technically required, with a training-consent line on a marketing page. DeepL API Free may store content perpetually and forbids personal data. |
| 6 | Where data is stored | **Not met as-is** | Every personal-data service is outside Vietnam (Singapore, US, EU). That makes a PDPL cross-border transfer, so a transfer impact assessment (TIA) must be filed with A05 within 60 days of the first transfer. No law found requires storage in Vietnam. |

---

## 1. One address (#558)

What #558 asks:
- `joinissho.com` serves the front page, the plans list and plan pages as HTML that search engines and link-card crawlers can read without JavaScript.
- A pasted plan link shows a card with title, city and summary.
- Retired plans drop out of search engines.
- Sign-in and the whole signed-in product live at the same address.
- No trackers.

What v1 does:
- The site was a static page.
- The member web app was a client-only Expo export at `app.joinissho.com`.
- Web-only code lives in `*.web.ts(x)` files (`apps/mobile/README.md`).

### What Expo Router can do (SDK 58)

| Mode | What it gives | Limits |
|---|---|---|
| `web.output: "single"` (v1) | One client-rendered SPA | Crawlers see an empty shell. |
| `web.output: "static"` | HTML per route at build time ([static rendering](https://docs.expo.dev/router/web/static-rendering/), modified 2026-11-02) | Dynamic routes need `generateStaticParams` at build time; anything else needs a server. A new or retired plan means a rebuild. |
| `web.output: "server"` | HTML per request; `loader`s run per request; `generateMetadata` feeds `<head>` ([server rendering](https://docs.expo.dev/router/web/server-rendering/), modified 2026-11-02) | Needs a server host. Caching is yours to set up. |

Details from the static rendering page:
- On SDK 58+, `generateMetadata` writes meta tags (title, Open Graph) into the static HTML.
- Tags set with `expo-router/head` `<Head>` are not in the initial HTML on SDK 58. Link-card crawlers that don't run JavaScript would miss them.
- SDK 58 allows `apiRoutes: true` alongside static output.

Details from the server rendering page:
- "Server rendering is stable in SDK 58 and later". On SDK 55–57 it was behind `unstable_useServerRendering`.
- Hosts: EAS Hosting, or adapters for Express, Cloudflare Workers, Vercel, Netlify and Bun. Static file hosts can't run it.
- **One project can't mix server and static rendering.**

[API routes](https://docs.expo.dev/router/reference/api-routes/):
- Labelled a "beta release".
- Run on WinterCG: no dynamic imports or native binaries, and CommonJS only.
- Expo hasn't tested the Vercel and Netlify adapters.

### Hosting cost

[EAS Hosting](https://docs.expo.dev/eas/hosting/introduction/) (2026-11-02):
- Runs on Cloudflare Workers, with partial Node compatibility.
- Custom domains on paid plans only, one per project.

[Expo pricing](https://expo.dev/pricing):

| Plan | Price | Hosting terms |
|---|---|---|
| Free | $0 | 100k requests/month, no custom domain |
| Starter | $19/month + usage | 1 custom domain; $2 per 1M requests beyond 100k |
| Production | $199/month + usage | same hosting terms |

Self-hosting the server build on Cloudflare Workers through the adapter is another route. Its cost was not checked here.

### Options for #572

| Option | How it meets #558 | Cost to a solo developer |
|---|---|---|
| A. Expo `output: "server"` for the whole web app | One codebase and one address. Plan pages get `generateMetadata` and per-request HTML, so retired plans can return 404/410 at once. | One more runtime to run (EAS Hosting $19+/month, or Workers). Server rendering only became stable in SDK 58. React Native Web markup has to be SEO-checked. Caching is DIY. |
| B. Expo `output: "static"` plus a rebuild on plan changes | Real HTML with no server. | A CI deploy on every plan publish or retire, so crawlers lag until each rebuild. Plans never known at build time would need a client fallback. |
| C. A web framework (Astro, Next, etc.) for public pages, with the Expo web SPA under the same domain | Mature SEO and link cards. A Worker or proxy routes `/`, `/plans/*` to one and the signed-in paths to the other. | Two web codebases and two design implementations, plus path routing and shared sign-in cookies or tokens across both. |
| D. Drop Expo on the web: a separate web app for everything | Full control over the web. | The signed-in product is written twice (native and web), the largest cost. |

Not checked: how Google indexes React Native Web output, beyond Expo's own claims.

## 2. Sign-in (#552, #559, #557)

### 2a. Email code: met

- v1 uses Supabase email OTP sent through Resend SMTP (`supabase/config.toml`).
- Supabase plans ([pricing](https://supabase.com/pricing)):
  - Free: 50k MAU, paused after 1 week of inactivity.
  - Pro: $25/month, 100k MAU, then $0.00325 per MAU.

### 2b. Apple and Google: met, with config work

| Provider | Native (iOS/Android) | Web | Notes |
|---|---|---|---|
| Apple ([docs](https://supabase.com/docs/guides/auth/social-login/auth-apple)) | `expo-apple-authentication`, then `signInWithIdToken` on iOS | OAuth flow | The OAuth flow (web, Android) needs the client secret regenerated **every 6 months**; native iOS alone doesn't. |
| Google ([docs](https://supabase.com/docs/guides/auth/social-login/auth-google)) | `@react-native-google-signin/google-signin`, then `signInWithIdToken` | `signInWithOAuth` with PKCE | The nonce check is on by default, with an option to skip it. |

- The Go API's JWKS verification is the same whatever the provider.
- Account linking (one person using both email and Google) was not checked.

### 2c. A verified +84 phone: partly met

This was already researched in [vietnam-phone-verification](https://github.com/shrujancheruku/issho/blob/research/vietnam-phone-verification/docs/research/vietnam-phone-verification.md) (2026-11-07). Its findings:
- **The law:** Decree 333/2026 Art. 16.2(b) and (d) require verifying an account by Vietnamese mobile number before it posts or interacts.
- **The Supabase path:** keep email OTP as the first sign-in. Add the phone with `updateUser({phone})`, then `verifyOtp` with type `phone_change`. The Go API gates interactive features on a `+84` `phone` claim.
- **Foreign visitors:** no lawful fallback beyond a local SIM.

Rechecked 2026-11-20:

| Fact | Source |
|---|---|
| Supabase SMS providers: MessageBird, Twilio, Twilio Verify, Vonage, TextLocal. One OTP request per 60 s; codes expire after 1 h. | [Phone login](https://supabase.com/docs/guides/auth/phone-login) |
| The Send SMS hook "replaces Supabase's built-in SMS sending", so any sender can be used, including ZNS. | [Send SMS hook](https://supabase.com/docs/guides/auth/auth-hooks/send-sms-hook) |
| Twilio SMS to Vietnam: **$0.2852 per segment** ("current as of November 2026"), plus a $0.001 failed-message fee. | [Twilio VN pricing](https://www.twilio.com/en-us/sms/pricing/vn) |
| Twilio Verify: **$0.05 per successful verification** plus channel fees; SMS attempts "are always charged". | [Verify pricing](https://www.twilio.com/en-us/verify/pricing) |
| Vietnam: sender ID and templates must be pre-registered, with a business registration certificate, in about 5 weeks. Unregistered sender IDs fully blocked from 2025-08-25. Body must include the brand name, with no URLs or phone numbers, transactional only. | [Twilio VN guidelines](https://www.twilio.com/en-us/guidelines/vn/sms) |
| Twilio suggests "consider using Twilio Verify which will automatically take care of all applicable registrations". Whether that covers Vietnam is **unverified**. | [Error 30018](https://www.twilio.com/docs/api/errors/30018) |
| AWS End User Messaging for Vietnam: a BM04 letter on company letterhead with company seal, company registration documents and a tax ID. One-time passwords is a listed use case. | [AWS VN registration](https://docs.aws.amazon.com/sms-voice/latest/userguide/registrations-vietnam.html) |
| Vonage Vietnam rules: page returned 403, **unverified**. | — |
| Zalo ZNS: **300 VND per OTP template message**, VAT excluded, no date on the page. | [ZNS pricing](https://zalo.solutions/zns/pricing) |
| ZNS needs a verified Zalo Official Account, and verification seems to need a business registration. **Unverified**: secondary sources only. | — |

Can an individual register a sender?
- Decree 91/2020 lets individuals register brand names (per the earlier note).
- But Twilio and AWS both ask for company documents.
- So the only individual route seems to be Twilio Verify's managed sender, which is **unverified for Vietnam**.
- Cost per verification: about **US$0.34** (0.05 + 0.2852), or more with retries.

Options for #572:
1. Twilio Verify through Supabase's built-in provider. Least code; Vietnam coverage needs a live test.
2. The Send SMS hook plus a registered brand name through a Vietnamese aggregator. Needs a business entity.
3. The Send SMS hook plus ZNS. Cheapest per message, but needs a verified OA and the member needs Zalo.

### 2d. A second sign-in step for admin staff (#557): met, with config work

| Factor | Status | Cost | Source |
|---|---|---|---|
| TOTP | "free to use and is enabled on all Supabase projects by default" | $0 | [TOTP](https://supabase.com/docs/guides/auth/auth-mfa/totp) |
| Phone MFA | Advanced MFA add-on, Pro and Team only | $75/month for the first project, $10 for each more, plus SMS | [Advanced MFA Phone](https://supabase.com/docs/guides/platform/manage-your-usage/advanced-mfa-phone) |
| Passkeys | "Passkey support is experimental"; beta announced 2026-05-28. First factor only; web (supabase-js ≥2.105.0), Flutter and Swift. React Native not mentioned. | — | [Passkeys](https://supabase.com/docs/guides/auth/passkeys), [changelog](https://supabase.com/changelog/46458-passkeys-for-supabase-auth-beta) |

How enforcement works ([MFA](https://supabase.com/docs/guides/auth/auth-mfa)):
- The JWT carries `aal` (`aal1` or `aal2`); a JWT without one counts as `aal1`.
- For APIs outside the database, check the `aal` claim of a verified JWT.
- So the Go API can require `aal2` on admin RPCs, with no Supabase change beyond enrolling TOTP in the admin SPA.

Other options:
- Put the admin behind an identity-aware proxy such as Cloudflare Access. Not researched here.

## 3. On-device translation in chat (#551)

What #551 asks:
- A translate toggle in chat that runs on the device; the server translates nothing.
- The toggle is hidden where the system can't translate (a device, a language pair, or the web).

| Platform | API | `vi` and `en`? | From Expo | Source |
|---|---|---|---|---|
| iOS | Apple Translation framework, `TranslationSession` (iOS 18.0+). "All translations using the TranslationSession class are processed on the user's device". A session comes from SwiftUI `.translationTask()`, or from `init(installedSource:target:)` (iOS 26+), which throws if the languages aren't installed. `prepareTranslation()` asks to download them. iOS 26.4+ adds a `highFidelity` strategy ("using Apple Intelligence") and `lowLatency` ("using traditional models"). | Vietnamese is in the Translate app. The developer-page language list is **unverified**. | Native module needed | [TranslationSession](https://developer.apple.com/documentation/translation/translationsession), [init(installedSource:target:)](https://developer.apple.com/documentation/translation/translationsession/init(installedsource:target:)), [Strategy](https://developer.apple.com/documentation/translation/translationsession/strategy) |
| Android (and iOS) | Google ML Kit Translation. "intended for casual and simple translations". Pairs pivot through English. Models download per language. | Yes: 59 languages including `vi` and `en` | Native module needed | [ML Kit](https://developers.google.com/ml-kit/language/translation), [languages](https://developers.google.com/ml-kit/language/translation/translation-language-support) |
| Web | Chrome Translator API, Chrome 138+. "These APIs don't work on mobile devices". The model downloads on first use. | `vi` is listed | Plain JS in a `*.web.ts` file | [Chrome docs](https://developer.chrome.com/docs/ai/translator-api) (updated 2025-05-20) |

More on the web API:
- [MDN](https://developer.mozilla.org/en-US/docs/Web/API/Translator_and_Language_Detector_APIs) (2026-08-04): experimental, not Baseline, secure contexts only.
- [MDN compat data](https://github.com/mdn/browser-compat-data/blob/main/api/Translator.json):

  | Browser | Translator API |
  |---|---|
  | Chrome desktop | 138 |
  | Edge | 148 |
  | Chrome Android | no |
  | Firefox | no |
  | Safari and iOS Safari | no |

ML Kit terms ([translation terms](https://developers.google.com/ml-kit/language/translation/translation-terms)):
- The Cloud Translation attribution rules apply, so the UI must credit Google.
- ML Kit is limited to phones, tablets, laptops and desktops.

Expo modules:
- No first-party Expo module.
- `@bsky.app/expo-translate-text` 0.2.9 (2026-03-06, MIT, published by bluesky-social): Apple Translation on iOS (task API iOS 18+, sheet 17.4+) and ML Kit on Android.
- Its upstream `expo-translate-text` 0.3.0 (2026-07-02).
- Sources: the npm registry entries. Maintenance health beyond the publish dates was not checked.

Fallback when there's no API:
- Hide the toggle (#551 already says so).
- Otherwise, on the web, rely on the browser's own page translation (Chrome, Safari and Edge offer whole-page translate outside the app's control).
- Server translation is ruled out by #551.

Gaps:
- Older iPhones (below iOS 18).
- All mobile browsers.
- Safari and Firefox on desktop.
- How well Apple handles `vi`→`en` on device, which needs a device test.

## 4. Push and email (#554): met

Push (v1: Expo push, per `docs/decisions-2026.md`):

| | Expo push service | Direct FCM v1 and APNs |
|---|---|---|
| Cost | "There is no cost associated with sending notifications through Expo push notification service" ([FAQ](https://docs.expo.dev/push-notifications/faq/)) | Free from Google and Apple |
| Limits | 600 notifications/s per project; 100 per request; about 4 KiB payload; receipts cleared after 24 h ([sending](https://docs.expo.dev/push-notifications/sending-notifications/), 2026-11-16) | Per-provider |
| Work | One HTTP API and one token type; optional access token | `getDevicePushTokenAsync`. FCM v1 needs a service-account OAuth token; APNs needs an ES256 JWT over HTTP/2 ([custom sending](https://docs.expo.dev/push-notifications/sending-notifications-custom/), 2026-11-02) |
| Data | Expo is one more recipient (US). Contents are held "in memory and queues, not databases" ([FAQ](https://docs.expo.dev/push-notifications/faq/)) | One fewer recipient |

- iOS push needs the paid Apple Developer Program and an APNs key; v1 deferred iOS push.
- No browser push: `lib/push.web.ts` is already a no-op (`apps/mobile/README.md`).

Email (v1: Resend SMTP; `joinissho.com` verified):
- [Pricing](https://resend.com/pricing): Free plan 3,000/month, 100/day, 30-day retention; Pro $20/month for 50k.
- [Regions](https://resend.com/docs/dashboard/domains/regions): sending from us-east-1, eu-west-1, sa-east-1 or ap-northeast-1. The region "does not control where customer data is stored"; data and logs stay in the US.
- Deliverability to Vietnamese inboxes (Gmail is common, plus local providers): **unverified**. No primary source measures it. Standard SPF, DKIM and DMARC alignment applies.

## 5. Model tools and their terms

### 5a. Web search with notes and links kept (#568)

Earlier note: [scraping-terms-and-law](https://github.com/shrujancheruku/issho/blob/research/scraping-terms-and-law/docs/research/scraping-terms-and-law.md) (2026-11-12). Its findings:
- **Anthropic web search:** citations are required when output is shown to users; no storage limit.
- **Ownership:** the Commercial Terms say the customer owns its Outputs.
- **Web fetch** obeys robots.txt.
- **Gemini Grounding with Google Search:** results may be shown only to the end user who asked. It forbids caching, training, or building a database or index. Ruled out by #568's "store only our notes plus the links".

New since that note (checked 2026-11-20):

| Fact | Source |
|---|---|
| Web search tool versions: `web_search_20250305`, `web_search_20260209` (dynamic filtering), `web_search_20260318` (`response_inclusion`). | [Web search tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-search-tool) |
| Web search costs **$10 per 1,000 searches** plus tokens; failed searches aren't billed. | same |
| Citations are always on. Each carries `cited_text` of up to 150 characters, plus `url` and `title`. Results come back as `encrypted_content`. | same |
| The citation rule: "When displaying API outputs directly to end users, citations must be included to the original source. If you are making modifications to API outputs, including by reprocessing or combining them with your own material ... display citations as appropriate based on consultation with your legal team." | same |
| Web search is zero-data-retention eligible, except dynamic filtering. `user_location` and allowed/blocked domains are available. | same |
| Web fetch: no extra charge beyond tokens. Fetches only URLs already in the conversation. Obeys robots.txt and blocks private addresses. No JavaScript rendering. Results cached (`use_cache`). Citations optional, with the same display note. "Website publishers might retain request data". | [Web fetch tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-fetch-tool) |

So keeping our own notes plus the source links fits the Anthropic terms, as long as the links are shown where output reaches users. The rights in the pages themselves are covered in the earlier note.

### 5b. One machine-translation service that forbids training

Use: the moderator's view of reports and messages (personal data).

| Service | The clause | Retention | Region | Vietnamese |
|---|---|---|---|---|
| **Anthropic API** | "Anthropic may not train models on Customer Content from Services." ([Commercial Terms](https://www.anthropic.com/legal/commercial-terms), Section B, effective 2025-06-17). Also: "Retained data is never used for model training without your express permission." ([API and data retention](https://platform.claude.com/docs/en/manage-claude/api-and-data-retention)) | "we automatically delete inputs and outputs on our backend within 30 days" ([retention policy](https://privacy.claude.com/en/articles/7996866-how-long-do-you-store-my-organization-s-data)). Zero data retention by arrangement. | Workspace geo (storage at rest): only `"us"`. Inference geo: `"global"` (default) or `"us"` at 1.1x price ([data residency](https://platform.claude.com/docs/en/manage-claude/data-residency)). | General LLM; no language list. Quality for vi→en not measured here. |
| **Google Cloud Translation** | "Google does not use the content you send to train and improve our Google Translation features." and "Google does not use any of your content for any purpose except to provide you with the Cloud Translation API service." ([data usage](https://docs.cloud.google.com/translate/data-usage), no date on page) | Text "held briefly in-memory"; no period given. | Global, EU and US multi-region endpoints; no Asia endpoint ([endpoints](https://docs.cloud.google.com/translate/docs/advanced/endpoints)). | Yes |
| **DeepL API Pro** | "DeepL will only temporarily store Content or Processed Content to the extent technically required to provide its Services" (Section 3.1.2), except up to 72 h encrypted for debugging (3.1.4). DeepL's licence covers content "solely in order to provide DeepL's Services to Customer" (7.4) ([Pro terms](https://www.deepl.com/en/pro-license), no date on page). The security page says "Texts are never stored or used for model training without your consent" ([data security](https://www.deepl.com/en/pro-data-security)). | Temporary; 72 h maximum for debugging | "DeepL has the right to determine the location of data processing" (3.1.3). EU hosting is **unverified** (it would be in the DPA, not read). | `VI` listed in the API OpenAPI spec v3.17.0 ([spec](https://developers.deepl.com/api-reference/openapi.yaml)) |
| DeepL API Free | DeepL "reserves the right to perpetually store any Content or Processed Content" (3.3.2), and the customer "shall not process any personal data with the API" (8.3.11). | — | — | Not usable for this. |

- Only Anthropic and Google state no-training explicitly in contractual or product terms.
- DeepL's explicit no-training line is on a marketing page. Its terms imply it through storage and purpose limits.
- Pricing per character for Google and DeepL could not be read from their pages: **unverified**.
- Using Anthropic for translation adds no new recipient if the product already uses Claude. The README lists Claude in the stack.

## 6. Where data is stored

### Per service

| Service (personal data held) | Where it's stored | Region choosable? | Source |
|---|---|---|---|
| Supabase (database, auth, storage) | The project region. v1 is in `ap-southeast-1` Singapore, the nearest to Vietnam. Other APAC regions: Tokyo, Seoul, Mumbai, Sydney. "The region you choose also determines where your primary project data is stored." | Yes, at project creation. v1's doc says it can't change later; the regions page doesn't say. | [Regions](https://supabase.com/docs/guides/platform/regions), [docs/supabase-hosted.md](../supabase-hosted.md) |
| Go API host | v1: GCP `asia-southeast1` (Singapore). Per #567 the API server is torn down. | Yes | [docs/supabase-hosted.md](../supabase-hosted.md) |
| Resend (email) | US. The sending region doesn't change where data is stored. | Sending only | [Resend regions](https://resend.com/docs/dashboard/domains/regions) |
| Expo push | Transferred to the US. Push tokens collected; under the EU-US Data Privacy Framework and SCCs. | No | [Expo privacy](https://expo.dev/privacy) (updated 2026-11-10), [subprocessors](https://expo.dev/privacy/subprocessors) |
| FCM | "may process and store your data anywhere Google ... maintain facilities". Installation IDs kept until deleted. | No | [Firebase privacy](https://firebase.google.com/support/privacy) (2026-11-16) |
| APNs | **Unverified** (not checked) | — | — |
| Twilio (SMS/Verify) | US1 by default. Other regions exist, but not all products are in them, and it "doesn't guarantee that all data will remain". Verify's region support is **unverified**. | Partly | [Twilio regions](https://www.twilio.com/docs/global-infrastructure/understanding-twilio-regions) |
| Zalo ZNS | **Unverified** (presumably Vietnam) | — | — |
| Sentry | US (Iowa) or EU (Frankfurt), fixed once chosen. Events stay in region; accounts, settings, audit logs and cron check-ins are in the US. v1 uses EU. | Yes, at org creation | [Sentry data storage](https://docs.sentry.io/organization/data-storage-location/) |
| Anthropic API | At rest in the US (workspace geo `"us"` only); inference global or US | Inference only | [Data residency](https://platform.claude.com/docs/en/manage-claude/data-residency) |
| Google Cloud Translation | Global, EU or US endpoint | Yes (EU/US) | [Endpoints](https://docs.cloud.google.com/translate/docs/advanced/endpoints) |
| DeepL API Pro | DeepL chooses (3.1.3) | No, per the terms | [Pro terms](https://www.deepl.com/en/pro-license) |

No service in the list stores data in Vietnam, apart from possibly ZNS.

### Vietnam's PDPL 91/2025: cross-border transfer

Research, not legal advice. This summarises [vietnam-data-localization](https://github.com/shrujancheruku/issho/blob/research/vietnam-data-localization/docs/research/vietnam-data-localization.md) (2026-11-07), which cites the official gazette texts:
[PDPL 91/2025/QH15](https://congbaocdn.chinhphu.vn/CongBaoCP/VanBan/2025/6/45578/57730-1-2025971-97291-2025-qh15.pdf), [Decree 356/2025](https://congbaocdn.chinhphu.vn/180507251028987904/2026/1/17/356signed-1768638052103952849513.pdf), [Decree 333/2026](https://congbaocdn.chinhphu.vn/180507251028987904/2026/9/4/470335-1787890927_v1_1788486330_signed.pdf), [Decree 330/2026](https://congbaocdn.chinhphu.vn/180507251028987904/2026/9/4/470339-1787824085_v1_1788483945_signed.pdf).

**What counts as a transfer:**
- PDPL Art. 20.1(c): processing data collected in Vietnam on a platform abroad is a cross-border transfer.
- Decree 356 Art. 17.1 includes storage on a foreign cloud.
- So every row above counts.

**The filing (Decree 356 Art. 18, form Mẫu 09):**
- **What:** a transfer impact assessment (TIA) dossier: the report, a copy of the transfer contract, and the sender's policies.
- **When and where:** within 60 days of the first transfer, to the Ministry of Public Security's personal-data authority (A05), online, in person or by post.
- **Response:** A05 answers within 15 days; an incomplete dossier must be completed within 30 days.
- **Updates:** every 6 months when something changes, and within 10 days for certain events (Mẫu 03).
- **Individual filers:** forms 01b, 02b and 03b exist. No fee was found.

**The DPIA:**
- Art. 19, form Mẫu 10.
- The small-enterprise relief covers only the DPIA and the DPO, and not above 100,000 data subjects. The TIA exemptions don't cover Issho's case.

**Fines (Decree 330 Art. 56):**
- TIA failures: 30–50 million VND for an organisation, half that for an individual.
- Leaks: up to 1–5% of Vietnam revenue.

**Localisation:**
- Neither the PDPL nor Decree 356 requires storing data in Vietnam.
- Decree 333 Art. 19 localisation applies to a foreign provider only after a specific MPS decision.
- No local-representative duty was found.

**Not rechecked on 2026-11-20:** amendments after 2026-11-07, and whether A05's online filing portal is live. Both **unverified**.

Effect on #572: the region choices don't remove the TIA duty; any non-Vietnam host triggers it. They change only what goes in the dossier: the list of recipients and countries, which #559's published recipient list already needs.

---

## Unverified

- Whether Twilio Verify's managed sender covers Vietnam without our own brand-name registration (needs a live test or Twilio confirmation).
- Whether an individual (not a company) can get a Vietnamese SMS brand name through any provider. Twilio and AWS both ask for company documents.
- Zalo OA verification needing a business registration, and where ZNS stores data.
- Vonage's Vietnam rules (page refused automated access).
- Apple Translation's supported-language list on an Apple developer page (Vietnamese confirmed only in the Translate app). Real vi↔en quality on device.
- Email deliverability to Vietnamese inboxes.
- APNs data location.
- Twilio Verify's data-region support.
- DeepL API data location (DPA not read) and current DeepL and Google Cloud Translation per-character prices.
- Whether a Supabase project's region can be changed after creation (v1's doc says no; the Supabase regions page is silent).
- Whether `@bsky.app/expo-translate-text` is maintained and works with Expo SDK 58.
- PDPL or Decree 356 changes after 2026-11-07, and the state of A05's online filing.
- How search engines index React Native Web pages from Expo server output, beyond Expo's docs.
- Cloudflare Workers cost for a self-hosted Expo server build.
