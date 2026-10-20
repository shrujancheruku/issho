# Issho: Build Plan & Status

> **This is the source of truth for what's planned, built and pending.** CI (`roadmap` job) fails any push or PR that changes code without updating this file. When you change code, update the status lines below. If the change made a decision, add it to the Decision log.

**Last updated:** 2026-10-20 · **Version:** 0.1.0 (pre-release; [releases](https://github.com/shrujancheruku/issho/releases)) · **Current milestone:** M2 Authentication (in progress)

Legend: ✅ done · 🟡 in progress / partial · ⬜ not started · ⏸ deferred (post-MVP)

---

## 1. Product

Issho proposes concrete outings to small groups of nearby people aged 18–40 who share interests. An outing is a plan such as *"Sunday photo walk from Kichijoji station, ending at Café X, 5 people"*. Groups are matched on interests, availability, a shared language and preferred group size.

After each event, attendees **privately** pick who they'd meet again. **Mutual** picks make those people more likely to be grouped again. Groups that click are offered a recurring **crew**.

- **Platonic only.** These rules discourage one-sided romantic pursuit:
  - only mutual picks count
  - no profile browsing and no 1:1 messages
  - photos are shown only to confirmed attendees
  - gender-composition rules
  - a "here for dating" report option
- **Location-agnostic.** Everything local (areas, travel times, languages, venues, activity names) is data in a **city pack**, not code. The first city is Tokyo.
- **North-star metric:** weekly attendees who have been to more than one event.

## 2. Architecture at a glance

See [architecture.md](architecture.md) for details.

```
Expo app (iOS/Android) ──┐                          ┌─ Supabase Postgres + PostGIS (app schema)
Admin SPA (browser) ─────┼─ ConnectRPC (HTTP/JSON) ─┤  Go API (cmd/api) ─┼─ Google Maps Platform (Places, Routes)
                         │                          │  Go worker (River) ┼─ Claude API (tagging, plan copy)
Supabase Auth ───────────┘  (JWT, verified via JWKS)│                    └─ Expo Push
```

## 3. Milestones

### M1 · Foundations (week 1): ✅ done
- ✅ Monorepo: pnpm workspaces + Turborepo (TypeScript), a Go module with pinned tools (`go tool` buf/sqlc/goose/protoc-gen-doc), and a `Makefile` as the single entry point.
- ✅ API contract in protobuf (buf + ConnectRPC). Generates the Go server, the TypeScript client and an [API reference](generated/api.md).
- ✅ Go API skeleton: config, pgx pool, goose migrations embedded in the binary, Supabase JWKS auth interceptor (deny-by-default), CORS deny-by-default.
- ✅ Services: `HealthService.Check`, `IdentityService.GetMe`, `CatalogService.ListActivities`.
- ✅ Schema v1: cities, areas, travel times, activity catalogue, city overrides, users, profiles, languages, availability, activity preferences.
- ✅ Starter data: 38 activities (English/Japanese) and the Tokyo pack (12 areas). Loaded with `cmd/tool seed`.
- ✅ Local Supabase stack (`supabase start`) with ES256 signing keys. Verified end to end: sign-up → API with a real token.
- ✅ Expo SDK 58 app wired to the generated client (health check on the home screen).
- ✅ CI (green from a clean checkout; actions on Node 24 majors): secret scan, roadmap check, proto lint/breaking, codegen drift, Go vet/test, migration round-trip + seed on PostGIS, TypeScript typecheck, commitlint.
- ✅ Releases: Conventional Commits + SemVer via release-please. Needs the repo setting Settings → Actions → General → "Allow GitHub Actions to create and approve pull requests" (enabled 2026-10-20). Can be re-run manually from the Actions tab.
- ✅ Secrets management: gitleaks (custom rules), pre-commit hook, CI scan, gitignore patterns. See [secrets.md](secrets.md).
- ✅ Onboarding documentation.
- ✅ Brand: logo and mark (一緒 with gold thread and hinomaru sun), one version for every background (black outlined in gold, gold and red outlined in black), generated reproducibly from Noto Serif JP (`assets/brand/`). App icon and splash still use Expo defaults; wire them up in M9.
- ⬜ Sentry and PostHog SDKs. *Moved to M9: nothing worth tracking yet.*

### M2 · Authentication (week 2): 🟡 in progress
- ✅ **Email OTP sign-in** (local): 6-digit code emails (EN/JA templates, 10-minute expiry). The Expo sign-in screen has an email step and a code step (with iOS autofill and auto-submit). Verified end to end with supabase-js against local Supabase.
- ✅ **Secure session storage:** Keychain/Keystore through a chunked `expo-secure-store` adapter (sessions are ~2.2 KB, over SecureStore's ~2 KB limit). Per-write generation IDs, so a torn write can only sign the user out. Covered by unit tests (Jest).
- ✅ **API client sends the Supabase access token.** Auth-gated navigation (`Stack.Protected`): sign-in → onboarding placeholder → tabs. Rejected sessions, e.g. after deletion on another device, sign out automatically.
- ✅ **Account deletion:** `IdentityService.DeleteAccount` (Supabase Admin API, cascade to app data, safe to retry). Sign out and delete in the app. Stale tokens of deleted users get `unauthenticated: account deleted`.
- ✅ **API error hygiene:** internal errors are logged server-side and returned as a generic `internal error`. Database details no longer leak.
- ✅ **Hosted `issho-dev` project**, configured with the Supabase CLI ([supabase-hosted.md](supabase-hosted.md)):
  - config pushed from `config.toml` with `[remotes.issho-dev]` overrides
  - Data API off, SSL enforced
  - Resend SMTP with the code templates
  - legacy keys disabled, ES256 signing
  - schema and seed applied
  - verified: DB state, Data API 503, advisors (only the known PostGIS warning), Go API health against the project
- ⬜ **Email OTP expiry set to 600 s in the dashboard.** `config push` (CLI 2.121) doesn't manage it.
- ⬜ **End-to-end sign-in against `issho-dev`** (code delivered by Resend to the account owner's inbox).
- 🟡 **SMTP:** Resend is configured for `issho-dev` (free tier, `onboarding@resend.dev`, delivers only to the owner's address). Before real users, it needs a **verified sending domain**.
- ⬜ **On-device test of the sign-in flow.** *Needs a phone with Expo Go, or Xcode (not installed on the dev machine).*

### M3 · Cities, catalogue & geo (weeks 2–3): 🟡
- ✅ City pack format and seeding tool.
- 🟡 Activity catalogue: 38 of a target ~200 activities.
- ⬜ Google Cloud project with restricted API keys (one server key, plus iOS and Android keys restricted to the app), per-API quotas and a budget alert.
- ⬜ `tool build-matrix`: area-to-area travel-time matrix from the Google Routes API (transit/walk/drive). Stored in `area_travel_times`.
- ⬜ `tool discover-venues`: Places API (New) Nearby Search per area × category, using field masks. Stores `google_place_id` plus our own tags.
- ⬜ A synthetic second city pack (car-based) to prove the app works in other cities without code changes.

### M4 · Matching engine (weeks 3–5): ⬜
- ⬜ `internal/matching` as a pure package (no I/O) with table-driven tests.
- ⬜ Step 1, bucketing: group users by area cluster (travel matrix), overlapping availability and a shared fluent language. Language-exchange buckets need complementary language pairs.
- ⬜ Step 2, candidate plans: activity combinations ranked by demand × feasibility (venue open, group size fits, time of day suits the activity).
- ⬜ Step 3, group assembly: greedy placement, then local search.
  - Score: interest overlap, size fit, mutual picks (at most 2 people who already know each other), mixing in newcomers, reliability.
  - Hard constraints: same-gender-only preferences, nobody is the only person of their gender, blocks.
- ⬜ Step 4: invite 1–2 more people than needed; the event confirms once enough people accept. Weights can be tuned per city pack.
- ⬜ A synthetic-user simulator (500 users × 2 cities).

### M5 · Onboarding & profile (weeks 5–6): ⬜
- ⬜ Profile RPCs and screens: city/area plus maximum travel time, languages, availability grid, activity tiles (like/love/open to try), group size, energy, budget, gender plus same-gender-only, birth year (18+), code of conduct.
- ⬜ Free-text interests → activity tags with Claude (Haiku). The user confirms the suggested tags. Includes a tagging eval set (JA/EN/ES).

### M6 · Proposals & events (weeks 6–8): ⬜
- ⬜ The River worker (`cmd/worker`):
  - weekly matching, daily top-ups
  - expiry and quorum checks
  - meeting-point reveal 24h before the event
  - chat opens 48h before the event
  - push via Expo Push
- ⬜ Proposals feed with accept/decline. Event screen: attendees, meeting point after the reveal, Google map, WebSocket group chat, check-in, share with a trusted contact.
- ⬜ Plan descriptions written by Claude in each user's language, with a template fallback.

### M7 · Feedback loop & crews (weeks 8–9): ⬜
- ⬜ Post-event feedback: rate the plan, rate the group, pick who you'd meet again (private), report.
- ⬜ Mutual picks recorded as matching links. A person's picks count for less when they're rarely returned.
- ⬜ Flag people whose picks lean heavily towards one gender, for moderation. Crews form from groups that rated each other highly.

### M8 · Admin SPA (weeks 9–11): ⬜
- ⬜ Vite + React app on Cloudflare Pages, signed in with Supabase Auth (admin role claim).
- ⬜ City-pack editor, venue/route curation, proposal review/override (build groups by hand for the first city), moderation queue, domain metrics.

### M9 · Release prep (weeks 11–12): ⬜
- ⬜ i18n pass (JA/EN).
- ⬜ Sentry (crashes and errors, mobile and Go) and PostHog (product analytics, feature flags).
- ⬜ Production deploy:
  - Dockerfiles for `api`/`worker`
  - **Kamal 2 (with kamal-proxy for TLS/routing)** to one VPS behind Cloudflare
  - Supabase upgraded to Pro
  - nightly `pg_dump` → Cloudflare R2 as an off-site copy
- ⬜ EAS Build/Submit → TestFlight and Play internal testing.
- ⬜ Stripe Identity verification behind a feature flag, for the public launch.

### Launch (after M9): ⬜
- ⬜ Waitlist page (Cloudflare Pages + PostHog) with counters per area. An area unlocks at a density threshold.
- ⬜ 5–10 hosts and partner venues per city. Seed users from university, language-exchange, newcomer and coworking communities.

### Post-MVP (deferred): ⏸
- ⏸ Sign in with Apple and Google (native, `signInWithIdToken`). Email codes are enough for the MVP. If any social login is added on iOS, Sign in with Apple becomes mandatory.
- ⏸ Paid tier (per-event fee or membership).
- ⏸ OTA updates (EAS Update).
- ⏸ Learned matching weights.
- ⏸ OSM provider as a fallback for Google.
- ⏸ Grafana Cloud / OpenTelemetry (see the decision log).
- ⏸ Neon branching.

## 4. Decision log

| Date | Decision | Why |
|---|---|---|
| 2026-10-20 | React Native + **Expo** for the app | Default for TypeScript teams; New Architecture closed the performance gap; EAS covers builds and signing. |
| 2026-10-20 | **Go** modular monolith, **ConnectRPC** + protobuf | One contract for the Go server and TS client; works over HTTP/1.1 from mobile. |
| 2026-10-20 | pgx + **sqlc** + **goose** + **River** | Type-safe SQL; migrations embedded in the binary; Postgres-backed jobs with no Redis. |
| 2026-10-20 | **Supabase** (DB + Auth + Storage) instead of Neon + Clerk | Open source; users live in our own Postgres; free auth up to 50k MAU; one vendor. Neon's better branching wasn't worth a second vendor. |
| 2026-10-20 | **Google Maps Platform** instead of OSM/Valhalla/OpenTripPlanner | Built-in transit; much better venue data (hours, ratings, "good for groups"); no routing servers to run. Fits the free tier at MVP scale. |
| 2026-10-20 | **Claude (Haiku)** for tagging and plan text instead of self-hosted embeddings | Better multilingual understanding; nothing to host; costs cents. |
| 2026-10-20 | Sentry, PostHog, EAS, Expo Push, Cloudflare, GitHub Actions (free tiers) | Each has a clear advantage over self-hosting for a solo developer. |
| 2026-10-20 | Prefer open source; paid/closed only when clearly better; every vendor behind a Go interface | Project principle. |
| 2026-10-20 | Conventional Commits + SemVer (release-please), first release `0.1.0` (`initial-version`) | Automated changelog and versioning. Without `initial-version`, release-please makes the first release 1.0.0 even with `bump-minor-pre-major`. |
| 2026-10-20 | Supabase config at repo root (`supabase/`), not `deploy/supabase` | The Supabase CLI expects `./supabase`. goose (not the CLI) owns migrations. |
| 2026-10-20 | `Makefile` as the task entry point; Turborepo runs the TS tasks | `just`/`mise` weren't installed and add nothing over make here. |
| 2026-10-20 | **Redundancy review: Caddy dropped.** | Kamal 2's built-in kamal-proxy already does TLS and routing, and Cloudflare sits in front. |
| 2026-10-20 | **Redundancy review: Grafana Cloud/OpenTelemetry deferred.** | Sentry already covers backend errors and performance tracing, and `kamal app logs` covers logs at MVP scale. Revisit when we need metrics dashboards. |
| 2026-10-20 | **Redundancy review: one error tracker.** | Sentry handles errors and crashes; PostHog's exception autocapture stays **off** so errors don't land in two places. |
| 2026-10-20 | **Redundancy review: no testcontainers.** | CI already runs a PostGIS service container and local dev uses `supabase start`. A third way to get a database adds nothing. |
| 2026-10-20 | **Redundancy review: Zustand and connect-query deferred.** | TanStack Query plus the generated client cover current needs. Add them only when local UI state or query-key boilerplate actually hurts. |
| 2026-10-20 | **Redundancy review: R2 kept, but only for off-site `pg_dump`.** | Supabase Pro does daily backups, but they live with the same vendor. A cheap off-site copy is real disaster recovery, not duplication. Photos use Supabase Storage. |
| 2026-10-20 | Admin SPA and waitlist on **Cloudflare Pages** | Static hosting is free, and with Caddy gone the VPS serves only the API. |
| 2026-10-20 | Logo built from Noto Serif JP outlines by a script (`scripts/brand/build_logo.py`) | Reproducible, OFL-licensed, no font dependency at display time. Colours: red `#BC002D`, gold `#C9A227`, black `#111111`. |
| 2026-10-20 | One logo version everywhere: no dark variant. Contrasting outlines instead. | Brand consistency. Gold outlines on black and black outlines on gold/red keep it legible on light and dark backgrounds. |
| 2026-10-20 | pnpm dependency build scripts deny-by-default (`allowBuilds` in `pnpm-workspace.yaml`) | pnpm 12 fails installs on unapproved install scripts. Each package is approved or denied explicitly; `unrs-resolver` is denied because its script only fetches fallback binaries. |
| 2026-10-20 | Commit `apps/mobile/src/types/expo.d.ts` | Expo's generated `expo-env.d.ts` is gitignored by convention, so fresh clones and CI couldn't typecheck CSS imports. |
| 2026-10-20 | Passwordless **email OTP codes**, not magic links | Links can't reliably open a native app, so the user types a 6-digit code instead (iOS autofills it from Mail). Codes expire in 10 minutes, down from 1 hour. |
| 2026-10-20 | Sessions in **`expo-secure-store`** (chunked), not expo-sqlite/AsyncStorage | Refresh tokens are long-lived credentials and belong in the Keychain/Keystore. Chunking works around the ~2 KB value limit; generation IDs make torn writes detectable. |
| 2026-10-20 | Error-sanitising Connect interceptor | connect-go returns `err.Error()` for plain errors, which leaked SQL constraint names to clients. |
| 2026-10-20 | Deleting an account: auth user first, then app data | If deleting the auth user fails, nothing is lost and the user can retry. The reverse order could leave a live login with no data. |
| 2026-10-20 | **Email OTP is the only sign-in method for the MVP**; Apple/Google deferred | Not needed to validate the product. It avoids OAuth setup and secrets, and the App Store rule that requires Sign in with Apple once any social login exists. |
| 2026-10-20 | **Supabase Data API disabled**; goose's version table moved from `public` to its own `goose` schema | Found while preparing the hosted project: through the Data API, anonymous clients could **write** to `public.app_goose_db_version` (marking migrations as applied) and modify PostGIS's `spatial_ref_sys`. Its grants are owned by `supabase_admin`, so a migration can't revoke them. Auth works without the Data API. CI now checks the version table isn't in `public`. |
| 2026-10-20 | Production `DATABASE_URL` uses the **session pooler** (or direct connection), not the transaction pooler | pgx uses prepared statements by default, and River needs LISTEN/NOTIFY. The transaction pooler supports neither. |
| 2026-10-20 | Hosted Supabase configured **declaratively with the CLI** (`config push`, `[remotes.<name>]` overrides) | Reproducible and reviewable via `config diff`. The diff caught local-only values that would have leaked: confirmations off, a 1 s resend interval, a 2/hour email cap. |
| 2026-10-20 | **Custom SMTP (Resend) is required even in development** | On the Free plan, Supabase blocks custom email templates while its built-in email service is in use, and the built-in templates only contain links, which can't drive code sign-in. |
| 2026-10-20 | Official **Supabase agent skills** in `.claude/skills/` (installed with a pinned `skills` CLI, verified byte-for-byte against upstream) plus a root `CLAUDE.md` | Gives coding agents current Supabase and Postgres guidance. `CLAUDE.md` overrides where the generic skills conflict with this repo (goose migrations, Data API off, MCP not configured). |
| 2026-10-20 | **Dela Gothic One** for standout/hero text in the app (`title`, `subtitle`); system font for body text | Ties the app to the wordmark. Loaded at runtime (`useFonts`) so it works in Expo Go; it has a single weight, so no `fontWeight` on display text. |
| 2026-10-20 | commitlint also runs as a local `commit-msg` hook | The first M2 PR failed CI on over-long commit bodies. Catching that at commit time is cheaper than rewriting history after a push. |
| 2026-10-20 | **jest-expo** for mobile unit tests; runs in CI with lint | Security-critical client code (session storage) needs tests. jest-expo is Expo's standard preset. |
| 2026-10-20 | CORS deny-by-default via an explicit origin allowlist | rs/cors treats an empty `AllowedOrigins` as allow-all. |

## 5. Known gaps & risks
- **Not enough nearby users** is the main product risk. The admin proposal override lets the first city be hand-curated.
- **PostGIS lives in the `public` schema** (migration 00001). With the Data API disabled, no anonymous path reaches it, but Supabase's advisors flag it. Move it into the `extensions` schema with a migration **before production data exists**: PostGIS can't be relocated in place, so the geography column has to be recreated.
- **Google Maps terms** restrict caching (store only place IDs long-term, fetch details live). Re-check the Maps Service Terms before M3 ships.
- **`postgis/postgis` has no arm64 image.** CI (amd64) is unaffected. On Apple Silicon, use `supabase start` (arm64) or run that image with `--platform linux/amd64`.
- **Disk space on dev machines:** Supabase images take several GB. OrbStack's VM disk can report "no space left" on its first launch; retrying fixes it.
