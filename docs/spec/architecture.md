# Architecture and stack

From [Architecture and stack](https://github.com/shrujancheruku/issho/issues/572), with facts from [What the v2 spec asks of the stack](https://github.com/shrujancheruku/issho/issues/571) and two research notes: [models on Vertex AI and forecast sources](https://github.com/shrujancheruku/issho/blob/research/vertex-and-forecast/docs/research/vertex-and-forecast.md) and [on-device translation options](https://github.com/shrujancheruku/issho/blob/research/on-device-translation/docs/research/on-device-translation.md). Hosting, backups, releases and the other running concerns are decided in [Technical operations](https://github.com/shrujancheruku/issho/issues/573).

## What v2 is built from

| Part | Decision |
|---|---|
| Member app | One Expo project, `apps/app`, for iOS, Android and the web. |
| The web | The same project, server-rendered (`web.output: "server"`) at `joinissho.com`. |
| Backend | Go, one module, two commands (`api` and `worker`) from one image. |
| API | ConnectRPC over protobuf. One generated TypeScript client, `packages/api-client`, used by the app, the Expo server and the admin. |
| Database | Postgres on Supabase. goose owns migrations, sqlc generates queries, app tables are in the `app` schema, and the Data API is off. No PostGIS. |
| Sign-in | Supabase Auth: an email code, with Apple and Google from the soft launch. The Go API verifies tokens against the project's JWKS. No phone number ([Privacy](privacy.md#no-phone-number-and-the-sign-in-log)). |
| Background work | River jobs in Postgres, run by `worker`. |
| Chat | A WebSocket served by the Go API. Postgres is the store, and `LISTEN/NOTIFY` fans messages out. |
| Admin | A separate Vite React SPA, `apps/admin`, at `admin.joinissho.com`, on the same Go API. |
| File storage | None. The spec has no photos or uploads. |

The repo keeps v1's top-level shape: `apps/app`, `apps/admin`, `backend`, `proto`, `packages/api-client`. The commit scope `mobile` becomes `app`.

## The rule for outside services

Google Cloud is the default for the model API and for any need that is still open and easy to replace, so that billing stays in one account. Another vendor is used only when a case is made for it. The cases made so far:

| Vendor | The case |
|---|---|
| Supabase | Email-code sign-in, which Firebase lacks, with Postgres beside it. |
| Resend | Google has no transactional email service. |
| Expo push | One integration and one token type for both stores. |
| Sentry | One tool for the app, the Expo server and the API. Google's equivalent for the app is Crashlytics, which brings Firebase's SDK into an app that promises no trackers. |
| Open-Meteo | Google's Weather API has no forecasts for Vietnam. |

## The web at one address

- The Expo server renders the public pages with their data: the front page, the plans list, plan pages and the five other pages ([The site](site.md)). It calls the Go API's unauthenticated RPCs.
- Everything signed-in renders in the browser. The Supabase session is held by the client, as on a phone.
- The server holds no tokens, cookies or personal data, so public pages can be cached.
- Expo's own API routes are not used. Every read and write goes to the Go API.
- A retired plan's page answers 410 at once.
- **Check in the build's first web ticket:** a plan page's HTML and link card read correctly without JavaScript. If they don't, the public pages alone move to a separate web framework at the same address.

## The backend's modules

One Go package per area of the spec, named in the glossary's words. Each owns its tables.

| Package | Owns |
|---|---|
| `people` | Accounts, profiles, standing, join links, consent |
| `cities` | City settings, areas, the calendar, lead sources, requests |
| `pieces` | Pieces, vouches, re-check clocks, doubt |
| `plans` | The editor, a plan's states, rounds, signups, the record |
| `planreview` | The accuracy checks, the model judge's verdicts, approval |
| `leads` | The leads list, suggestions, the lead search, outlines |
| `matching` | Forming groups and setting dates |
| `outings` | Invitations, seats, check-in, attended, feedback, changes |
| `crews` | Crews and crew outings |
| `chat` | Rooms and messages |
| `safety` | Reports, kept apart, holds, warnings, reliability points, flags, the tombstone |
| `notifications` | Message types, quiet hours, push and email |
| `staff` | Staff roles, the queue, the staff log, the scorecard |
| `retention` | Deletion and each record's lifetime |

### The seams

- **No package reads another's tables.** A package reaches another only through its exported Go API. CI checks the imports.
- **Matching is a pure function.** It has no database and no clock. It takes a round's signups, the kept-apart pairs, what the seat order needs and the city's calendar, and returns groups and dates. `plans` and `outings` load its inputs and store its results. The owner's approval of groups in the Pilot is a look at its output before it is stored.
- **The rules' numbers are constants.** One `policy` package holds every number in the spec (3 to 6, 48 hours, 7-day rounds, the reliability points), each with a comment linking its document. None is in the database and none varies by city, except the walk and ride limits a city may override ([What varies by city](city-settings.md)). A number changes by a PR that edits the document and the constant together.
- **One city clock.** Every time on the city's clock ("09:00 on the day", quiet hours, a round's end) is computed by one helper from the city's time zone. Code takes an injected clock and never asks the system for the time.
- **One queue.** `staff` owns the queue's table. A package opens an item with a due time and its written default, and a River job applies the default when it falls due ([Running Issho](running-issho.md)).
- **Jobs are enqueued in the transaction that causes them.**
- **Outside services sit behind interfaces.** Each has one Go interface in its own package and one shared in-memory fake that every test uses. Real implementations are wired only in `cmd/`. A domain package never calls an outside service directly.

The city clock, the queue and `policy` are built before any domain package.

## Where things live

| Thing | Where | Changed by |
|---|---|---|
| City settings, areas, the calendar, lead sources | The database | The owner in the admin; the calendar and lead sources also by insiders in the Making tab. Each change records who and why. A fixture file seeds development and tests only. |
| Staff roles | Rows in `staff`'s table, an insider's scoped to a city | The owner. The API reads the role on each staff RPC, so removing one takes effect on the next request. The token says only who the person is and whether the second step was done. |
| The second sign-in step for staff | A TOTP factor in Supabase Auth | The API requires `aal2` on every staff RPC. |
| The app's words | JSON files in the repo, one per display language | A PR. The paid reader reads a preview build before each stage ([Languages](languages.md)). The Go API renders push and email bodies from the same files. |
| Plan text and its translations | The database | Written in the editor, translated at approval. |
| The rules' numbers | The `policy` package | A PR. |

## Models

Models are called through Vertex AI. Vertex has no one API across model families, so the swap is ours: one Go interface per use, one adapter per family (Gemini, Claude), and config that names the model for each use. An eval command runs a fixed set of cases through any configured model.

| Use | Interface | Starts on | May be sent |
|---|---|---|---|
| The plan judge | `models.Judge` | A Gemini model, chosen by the eval against the twelve plans the owner judged in the [prototype](plan-bar.md#what-the-prototype-showed) | The plan and its pieces |
| Plan translation | `models.PlanTranslator` | A Gemini model | The plan's text and fields |
| The lead search | `models.LeadSearcher` | Claude with its web search tool, in Vertex's Google-managed offering | The city, its gaps and its lead sources |

- **No personal data goes to a model.** Each interface accepts only the types in its row, so the code cannot pass a person's data.
- Models are called on Vertex's `global` endpoint. With no personal data, where they run doesn't matter, and global has the newest models.
- Google's own grounding (Google Search, Web Grounding for Enterprise, Maps) is never used for the lead search: its terms forbid storing results or building an index from its links ([Plan knowledge](plan-knowledge.md)).
- **Check in the build's first model ticket:** an individual, not a company, can enable Claude on Google Cloud. If not, the lead search uses the same tool on Anthropic's own API through the same adapter.

## Outside services

| Need | Service | Notes |
|---|---|---|
| The moderator's translation of reports, messages and feedback | Google Cloud Translation, behind `translate.Translator` | The only place personal data is translated. Text is held in memory only and never trained on. A model is not used: its abuse logging can keep flagged prompts for 90 days. Sent the text alone, with no names or IDs. |
| A place's identity, pin, live hours and closed flag | Google Places, behind `places.Places` | Called only from the Go API, by place ID, shown live and never stored. |
| Checking a maker's stated leg minutes | Google Routes | At review only. Nothing is kept but pass or fail. |
| Rain in the forecast | Open-Meteo's paid API, behind `forecast.Forecast` | An hourly chance of rain up to 16 days ahead. Its data is CC BY 4.0, so "rain forecast for these hours" is stored with attribution. |
| Severe-weather warnings | Google's Weather API alerts | Vietnam's official warnings; the only weather data Google has for Vietnam. |
| Push | Expo's push service, behind `push.Sender` | |
| Email | Resend, behind `email.Sender` | Supabase's sign-in emails go through Resend's SMTP; the product's own go through its API from `worker`. |
| Errors | Sentry, EU region | No request bodies or headers. |

Who receives personal data is the recipient list in [Privacy](privacy.md#who-receives-personal-data). Models, Places, Routes, Open-Meteo and the weather alerts are not on it.

**Checks for the build ticket that adds the forecast:** Open-Meteo's paid price (running its open-source server ourselves is the fallback), and how long Google's alert content may be kept.

## On-device translation

The chat's translate toggle ([Chat](chat.md#translation)) is a local Expo module of our own in `apps/app/modules/`, started from an existing wrapper's source where its licence allows.

| Platform | Engine |
|---|---|
| iOS 18 and later | Apple's Translation framework |
| Android | Google ML Kit, with Google credited beside a translated message |
| Desktop Chrome and Edge | The browser's `Translator` API |
| Everything else (older iPhones, phone browsers, Safari, Firefox) | The toggle isn't shown |

- The module answers "can this device translate this pair?" before the toggle is shown.
- ML Kit is not used on iOS, and no translation model ships in the app.
- If a platform's engine goes away, the toggle hides there. The server never translates chat.
