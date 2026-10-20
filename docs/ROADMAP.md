# Issho: Build Plan & Status

> **This is the source of truth for what's planned, built and pending.** CI (`roadmap` job) fails any push or PR that changes code without updating this file. When you change code, update the status lines below. If the change made a decision, add it to the Decision log.

**Last updated:** 2026-10-20 · **Version:** 0.1.0 (pre-release; [releases](https://github.com/shrujancheruku/issho/releases)) · **Current milestone:** M1 Foundations (nearly done) → M2 Auth

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

### M1 · Foundations (week 1): 🟡 nearly done
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

### M2 · Authentication (week 2): ⬜
- ⬜ Hosted Supabase project (Free tier for dev). Asymmetric signing keys enabled.
- ⬜ Supabase Auth providers: Sign in with Apple, Google, email OTP. A custom SMTP relay (Resend/SES free tier) for OTP email.
- ⬜ Expo sign-in screens: native Apple/Google → `signInWithIdToken`, and email OTP. Session stored in `expo-secure-store`.
- ⬜ API client sends the Supabase access token; auth-gated navigation (onboarding vs main app).
- ⬜ Account deletion (Supabase Admin API + cascade).

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
| 2026-10-20 | Logo polish: black-outlined parts (sun, thread) layered over gold-outlined parts; wordmark in **Dela Gothic One** | The serif wordmark was bland. A chunky, Japanese-designed display face gives the name personality next to the brush-like kanji. |
| 2026-10-20 | One logo version everywhere: no dark variant. Contrasting outlines instead. | Brand consistency. Gold outlines on black and black outlines on gold/red keep it legible on light and dark backgrounds. |
| 2026-10-20 | pnpm dependency build scripts deny-by-default (`allowBuilds` in `pnpm-workspace.yaml`) | pnpm 12 fails installs on unapproved install scripts. Each package is approved or denied explicitly; `unrs-resolver` is denied because its script only fetches fallback binaries. |
| 2026-10-20 | Commit `apps/mobile/src/types/expo.d.ts` | Expo's generated `expo-env.d.ts` is gitignored by convention, so fresh clones and CI couldn't typecheck CSS imports. |
| 2026-10-20 | CORS deny-by-default via an explicit origin allowlist | rs/cors treats an empty `AllowedOrigins` as allow-all. |

## 5. Known gaps & risks
- **Not enough nearby users** is the main product risk. The admin proposal override lets the first city be hand-curated.
- **Google Maps terms** restrict caching (store only place IDs long-term, fetch details live). Re-check the Maps Service Terms before M3 ships.
- **`postgis/postgis` has no arm64 image.** CI (amd64) is unaffected. On Apple Silicon, use `supabase start` (arm64) or run that image with `--platform linux/amd64`.
- **Disk space on dev machines:** Supabase images take several GB. OrbStack's VM disk can report "no space left" on its first launch; retrying fixes it.
