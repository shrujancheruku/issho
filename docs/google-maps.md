# Google Maps Platform

Issho uses Google Maps Platform for two things, both behind the `internal/maps` interfaces (`Router`, `PlaceSearcher`):

| What | API | Used by | When |
|---|---|---|---|
| Area-to-area travel times | **Routes API**, Compute Route Matrix | `tool build-matrix` | per city, refreshed at least every 30 days |
| Venue discovery | **Places API (New)**, Nearby Search | `tool discover-venues` | per city, occasionally |
| Maps in the app (M6) | Maps SDK for iOS / Android | Expo app | at runtime |
| On-demand translation | **Cloud Translation API** (Basic, v2) | API server (`internal/translate/google`) | when someone needs a language that isn't cached yet |

Tests never call Google: they use `httptest` fakes. Only the two `tool` commands need a real key.

## 1. Project and billing

One Google Cloud project per environment (`issho-dev` now, `issho-prod` before launch), so keys, quotas and bills stay separate.

1. [console.cloud.google.com](https://console.cloud.google.com) → **New project** → `issho-dev`.
2. **Billing** → link a billing account. Maps Platform requires one even within the free monthly usage.
3. **APIs & Services → Library** → enable **Routes API**, **Places API (New)** and **Cloud Translation API**. Don't enable the legacy *Places API* or *Distance Matrix API*: we don't use them.
4. Later (M6): enable **Maps SDK for Android** and **Maps SDK for iOS**.

## 2. API keys

**APIs & Services → Credentials → Create credentials → API key.** Create the keys below and restrict each one immediately:

| Key | Application restriction | API restriction | Where it lives |
|---|---|---|---|
| `issho-dev-server` | None during development; **IP addresses** (the VPS) in production | Routes API, Places API (New) | `GOOGLE_MAPS_API_KEY` in your shell or a gitignored `.env.issho-dev`; Kamal secrets in production |
| `issho-dev-translate` | Same as the server key | Cloud Translation API | `GOOGLE_TRANSLATE_API_KEY`, for the API server |
| `issho-dev-android` (M6) | Android apps: package `com.joinissho.app` + signing SHA-1 (from `eas credentials`) | Maps SDK for Android | `EXPO_PUBLIC_*` (public, see below) |
| `issho-dev-ios` (M6) | iOS apps: bundle ID `com.joinissho.app` | Maps SDK for iOS | `EXPO_PUBLIC_*` (public) |

- The **server key is a secret**. Never commit it, paste it into an issue, or put it in an `EXPO_PUBLIC_*` variable. The client sends it in the `X-Goog-Api-Key` header, never in URLs, so it doesn't end up in logs or error messages.
- The **mobile keys are public by nature** (they ship inside the app binary). Their protection is the app restriction, plus the API restriction to the Maps SDKs only.
- gitleaks' default rules detect Google API keys (`AIza…`), so the pre-commit hook and CI catch an accidental commit. If a key leaks, **delete it in the console first**, then create a new one.

## 3. Quotas and budget

Cap usage so that a bug (e.g. a loop calling discovery) can't run up a bill. Quota names vary a little in the console; set the per-minute and per-day limits for each API:

1. **Billing → Budgets & alerts → Create budget**: scope the `issho-dev` project, budget **$10/month**, alerts at 50%, 90% and 100%. Budgets only alert; they don't stop spending. The quotas below are the hard stop.
2. **APIs & Services → Routes API → Quotas**: set *Compute Route Matrix elements per minute* to **1,000** (a full Tokyo build is ~170 elements).
3. **APIs & Services → Places API (New) → Quotas**: set *Search Nearby requests per day* to **1,500** (a full city discovery is ~500 requests).

## 4. Running the tools

```sh
export GOOGLE_MAPS_API_KEY=…   # or put it in .env.issho-dev and use ENV_FILE=
make matrix CITY=tokyo ARGS=-dry-run     # prints requests and billed elements; no Google calls
make matrix CITY=tokyo
make venues CITY=tokyo ARGS=-dry-run
make venues CITY=tokyo ARGS='-categories cafe,park'   # a subset first
make venues CITY=tokyo
```

Always dry-run first: it reads the city from the database and prints exactly what a real run would call.

**Costs at MVP scale** (check current [pricing](https://developers.google.com/maps/billing-and-pricing/pricing); each SKU has a free monthly usage allowance):
- `build-matrix`: one billed element per origin/destination pair. Only walking and driving are calculated (Google has no transit routes in Japan). Tokyo (12 areas) = 166 elements, Austin (10 areas) = 102. Same-area pairs are never requested, and walking is only requested between areas less than 4 km apart.
- **The worker's monthly rebuild** (`refresh_maps`, daily check): the same elements as `build-matrix`, once every 25 days per city.
- **Venue names on plans** (API, `GetPlan` / `GetEvent`): Place Details with `displayName`, `shortFormattedAddress`, `googleMapsUri`, `location` (the Pro SKU), up to two calls per venue (one per display language) each time a plan or event screen is opened. This is the cost that grows with users: watch it.
- **Plan routes** (worker, `RefreshRoutes` in each city pass): one Routes `computeRoutes` call (walking, by place id; duration, distance and polyline only) per plan with two venues, when it opens and then every 25 days while it's open or has an event to come. Negligible.
- **Walks** (worker, when a walking plan opens; at most 4 per city pass, and an area that failed rests 7 days): Text Search (New) with `id`, `displayName`, `primaryType`, `types`, `location`, `businessStatus` (the Pro SKU; the English name is compared with the model's in memory, since Text Search returns its nearest match even for a place that doesn't exist, and is never stored or passed on) for each proposed place (about 7–9), and one or two Routes `computeRoutes` calls through every stop with `optimizeWaypointOrder` (the Advanced SKU), twice if the model retries. About $0.25 a walk with Gemini. A walk's route is then refreshed like other routes, without reordering.
- **Walk benchmark** (`tool walk-bench`, run by hand): the same lookups and routes as composing walks, for every case, variant and rep (about $0.35 a walk at list price; a 12-case pilot of 6 variants is ~72 walks).
- **Review sheet maps** (`walk-bench review`): links to Google Maps directions (Maps URLs, no key, no charge); embedded routes use the Maps Embed API (free, unlimited), which must be enabled on the project and its key restricted to it.
- **Opening hours** (worker, when a round is about to be scheduled): Place Details with `businessStatus`, `regularOpeningHours` (the Enterprise SKU, the dearest), one call per venue per scheduling pass. Not checked on every hourly pass, which would be ~2,400 calls a day at 50 plans.
- **Venue curation** (admin SPA, by hand): the Venues page fetches each listed place's English name with Place Details (Pro SKU, 50 a page, each time it's shown); a name search is one Text Search (New) per area searched (Pro SKU: `id`, `displayName`, `primaryType`, `types`, `location`, `businessStatus`, `shortFormattedAddress`, `googleMapsUri`; up to 20 places), so a Tokyo-wide search is 12 calls (~$0.40 at list price); adding a place is one Place Details call to check it. Names and addresses are shown, never stored.
- `discover-venues`: one Nearby Search request per area × venue category, except wide-radius categories (≥ 10 km, e.g. beaches, trails), which are searched once per city. The field mask asks only for id, types and location.

## 5. Terms: what we may store

The Google Maps Platform terms restrict caching of Maps content. How Issho follows them:

- **Place IDs** may be stored indefinitely. `app.venues` stores only the place id plus **our own** data (category, nearest area, curation status, tags). Names, addresses, hours, ratings and photos are **not** stored; they're fetched live when shown.
- **Place locations** are used in memory to assign each venue to its nearest area, and are never written to the database.
- **Travel times** (Routes content) are cached for at most **30 days**. A rebuild (`build-matrix` or the worker) replaces a city's whole matrix and deletes every row it didn't write. The worker's daily `refresh_maps` job rebuilds a city once its matrix is 25 days old; without `GOOGLE_MAPS_API_KEY` it can't, so it deletes rows past 30 days instead (and logs an error), because staying within the terms matters more than keeping stale data.
- **Plan routes** (the walk between a plan's two venues, or through a walk's stops: polyline and duration, Routes content) are cached on the plan for at most **30 days**: recomputed at 25 days, cleared at 30 (`app.plans.route_*`, migration 00012), also without a Maps key. A walk's legs (`app.plan_stops.walk_seconds`) are cleared with it.
- **Walks** store only place ids. Nothing from Google is given to the model: it proposes from its own knowledge, the worker checks its places with Places and Routes, and the feedback for a retry names only the model's own places and our rules ("no content may be created from Google Maps content"). **The benchmark** (owner's decision, 2026-10-22): the Maps terms forbid using Google Maps content "to improve machine learning and artificial intelligence models, including to train, test, validate or fine-tune the models". The benchmark runs the production checks unchanged and keeps only their outcome (place ids and counts of places dropped); no Google content reaches a model or judge, and no model is trained or changed. The owner judged that the clause doesn't apply. Keep it that way: never put Places or Routes content in a prompt, a judge's input or a training set. Gemini's *Grounding with Google Maps* was ruled out: its answers may only be shown to the person who asked and not stored beyond 90 days for display tuning.
- **Opening hours and business status** are fetched when needed and never stored. A venue Google reports as permanently closed gets our own curation status `rejected` (that's our data, not Google's).

- **Attribution:** wherever Places data (venue names, addresses) is shown without a Google map, the app shows the text **Google Maps** (`components/google-maps-attribution.tsx`): never translated, 12sp, #5E5E5E on light backgrounds and white on dark, the only colours Google allows. Today that's under the plan and event timelines. Don't restyle it to the app's greys. On a Google map, the map carries its own logo.

Re-check the current [Service Specific Terms](https://cloud.google.com/maps-platform/terms/maps-service-terms) and the [Routes](https://developers.google.com/maps/documentation/routes/policies) and [Places](https://developers.google.com/maps/documentation/places/web-service/policies) policies before launch. Attribution rules apply wherever Google content is displayed in the app.

## 6. Cloud Translation

Our own content is stored in English only; other languages are machine-translated on demand and cached in `app.translations` ([architecture](architecture.md#languages)). The API server calls **Cloud Translation Basic (v2)** with `GOOGLE_TRANSLATE_API_KEY`:

- Each English string is translated once per language, then served from the database to everyone. The whole activity catalogue is about 3,000 characters per language.
- Basic has a free monthly character allowance, then bills per million characters (check current [pricing](https://cloud.google.com/translate/pricing)). Add a quota on *characters per day* (e.g. 200,000) as a hard stop.
- Basic can't take context, so one-word labels are translated without knowing what they are. If quality suffers, the `Translator` interface allows switching to Advanced (v3, glossaries; needs a service account rather than an API key) or another engine. Cached rows record which translator produced them.
- Without the key, the app still works: text appears in the languages already stored.

