# Deploying the backend

The Go API and the River worker run on one VPS, deployed with **Kamal 2** from a single image (`backend/Dockerfile`). Cloudflare sits in front of the API; Supabase (database and auth) stays hosted. The admin SPA and the web build are separate static sites on Cloudflare Workers ([../apps/admin/README.md](../apps/admin/README.md)).

```
app / admin SPA ──HTTPS──▶ Cloudflare ──HTTPS (origin certificate)──▶ kamal-proxy ──▶ api container ─┐
                                                                                      worker container ─┴──▶ Supabase Postgres
```

**Status:** live since 2026-10-26 at `https://api.joinissho.com` (first `kamal setup` from a dev machine), against the `issho-dev` Supabase project. Deploys are automatic on merge (see *Automatic deploys*); the backup accessory is off until its R2 bucket exists. The origin certificate (15 years, `api.joinissho.com` only) is in `~/.config/issho/` on the deploying machine. Mind that a PEM pasted from the dashboard has no blank lines inside it, or Kamal's proxy can't read the key.

## Domain: joinissho.com

Registered at Porkbun (2026-10-26); its DNS is moved to Cloudflare, which also serves the sites.

| Host | What | Where |
|---|---|---|
| `joinissho.com`, `www.joinissho.com` | marketing page (`site/`) | Worker `issho-site` |
| `app.joinissho.com` | the app's web build | Worker `issho-web` |
| `admin.joinissho.com` | admin SPA | Worker `issho-admin` |
| `api.joinissho.com` | Go API | the VPS, through Cloudflare (proxied) |

The Workers' `wrangler.jsonc` files claim their host names as custom domains, so a deploy creates their DNS records. The `workers.dev` addresses keep working.

**Moving DNS to Cloudflare (owner, once):** in Cloudflare, *Add a domain* → `joinissho.com`, Free plan; delete the imported Porkbun parking records; at Porkbun, *Domain → Authoritative nameservers* → Cloudflare's two (and switch off Porkbun's DNSSEC first, if on). The zone is active once Cloudflare sees the change (usually under an hour). Then SSL/TLS mode **Full (strict)**.

## One-time setup

1. **The server** (done 2026-10-26): Compute Engine VM `issho-api` in project `issho-dev-512213`, `asia-northeast1-b` (Tokyo, near the database in Seoul), `e2-small` (2 GB), Ubuntu 24.04, 20 GB disk, static address `35.243.76.67` (reserved as `issho-api`). Its startup script installs Docker, adds a 2 GB swap file and unattended upgrades, and puts the `kamal` user (created from the SSH key in the instance metadata) in the `docker` group: Compute Engine has no root login, so Kamal connects as `kamal` (`ssh.user`). Firewall `issho-api-web-from-cloudflare` lets 80 and 443 in only from [Cloudflare's addresses](https://www.cloudflare.com/ips/) (tag `issho-api`); SSH is the project's default rule.
2. **The API's domain on Cloudflare:** an `A` record for `api.joinissho.com` pointing at the VPS, **proxied** (orange cloud). SSL/TLS mode **Full (strict)**.
3. **A Cloudflare origin certificate** for that host (SSL/TLS → Origin Server → Create certificate). Save the certificate and key outside the repository, e.g. `~/.config/issho/origin-cert.pem` and `origin-key.pem`.
4. **A registry token:** a GitHub personal access token (classic) with `write:packages`, exported as `KAMAL_REGISTRY_PASSWORD` when deploying. The image is pushed to `ghcr.io/shrujancheruku/issho`; keep the package private.
5. **A production Supabase project**, configured as in [supabase-hosted.md](supabase-hosted.md), and its values in a gitignored `.env.issho-prod` (same names as `.env.issho-dev`). `DATABASE_URL` must be a **session-mode** connection (direct, or the session pooler): the API listens for chat messages and the worker for River jobs.
6. **Fill in `config/deploy.yml`:** every line marked `TODO` (server address, API host, registry account, `SUPABASE_URL`, `ALLOWED_ORIGINS`).
7. **Secrets:** `cp .kamal/secrets.example .kamal/secrets` (gitignored). It reads each value from `.env.issho-prod` and the certificate files, so the file itself holds no secrets. See [secrets.md](secrets.md).
8. **Install Kamal** (Kamal 2 needs Ruby 3.2+; macOS ships 2.6, so the image is simpler; add `-v ~/.config/issho:/root/.config/issho:ro` so the secrets file finds the certificate, and on OrbStack or Docker Desktop pass the SSH agent as `-v /run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock -e SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock`): `gem install kamal`, or run it from its image: `alias kamal='docker run -it --rm -v "$PWD:/workdir" -v "$SSH_AUTH_SOCK:/ssh-agent" -e SSH_AUTH_SOCK=/ssh-agent -v /var/run/docker.sock:/var/run/docker.sock ghcr.io/basecamp/kamal:v2.12.0'`.
9. **First deploy:** `kamal setup` (installs Docker, starts kamal-proxy, then deploys as below).

## Deploying

From a clean checkout of the commit to ship (Kamal builds from a fresh clone of `HEAD`, so uncommitted changes are never shipped):

```sh
kamal build push        # build the amd64 image (cross-compiled, fast on a Mac) and push it
kamal migrate           # apply new migrations with that image (alias for `app exec --primary --roles=web "tool migrate up"`, so it runs once)
kamal deploy --skip-push
```

Migrations run **before** the new code, so the old code must work with the new schema for the minute in between; keep migrations additive, as they already are. kamal-proxy only switches traffic once the new API container's `GET /up` answers 200, which needs the database, so a broken `DATABASE_URL` never replaces a working container.

Then check:

```sh
curl https://<api host>/up                                     # ok
curl -X POST -H 'Content-Type: application/json' -d '{}' https://<api host>/issho.v1.HealthService/Check   # the commit's version
kamal logs-api          # or logs-worker
```

**Checking Sentry:** `kamal app exec --primary "tool sentry-test"` sends one test error to the `issho-api` project (Sentry organisation `shrujan`, EU region), as the API and worker would.

**Seeding and Google tools** run the same way, inside the image: `kamal app exec --primary "tool seed /data/citypacks/tokyo.json"` (the catalogue is seeded too).

**Rolling back:** `kamal rollback <version>` (versions are commit hashes; `kamal app containers` lists them). Migrations aren't rolled back.

## Automatic deploys

`.github/workflows/deploy-api.yml` deploys every change to `backend/`, `data/`, `config/deploy.yml` or `config/server/` on main (or on demand), without production secrets in GitHub:

1. The job builds and pushes the image with Kamal, using its own short-lived `GITHUB_TOKEN` (the workflow created the `issho` package, so it's linked to the repository; the image's `org.opencontainers.image.source` label says so too).
2. It connects as `deploy@<server>` with the `DEPLOY_SSH_KEY` secret (host key pinned by the `DEPLOY_HOST_KEY` variable) and writes `<commit> <token>` to it. That key's `authorized_keys` line is `restrict,command="sudo -n -u kamal /opt/issho/deploy.sh"`, and `/etc/sudoers.d/issho-deploy` allows exactly that, so the key can do nothing else.
3. [`config/server/deploy.sh`](../config/server/deploy.sh) (installed root-owned at `/opt/issho/deploy.sh`) refuses a commit that isn't on `origin/main`, checks it out in `/opt/issho/repo`, and runs Kamal from its image against this server (`kamal` connects to itself with `~kamal/.ssh/id_self`, registered in the instance's SSH-key metadata): `registry login`, `migrate`, `deploy --skip-push`, then logs out of the registry.
4. The job checks that `HealthService.Check` reports the new commit.

**Gated on CI.** Both deploy workflows (`deploy-api`, `deploy-web`) trigger on `workflow_run` of the `ci` workflow for main and run only when it concluded `success` for a push; when CI fails they show as skipped and nothing ships. They deploy `workflow_run.head_sha` (printed in the job log as `deploying <sha>`), the commit CI tested, not whatever main is by then. Since `workflow_run` has no path filter, each workflow's `changes` job diffs that commit against the last commit the target actually deployed (the newest successful run on main whose deploy job succeeded, found with `gh run list` and `gh run view`; the commit is in the run's title, `deploy <sha>`). A red commit's changes are therefore deployed with the next green one; with no earlier deploy it deploys everything (the first run after this change, whose title has no commit, does too). Runs that skipped (red CI, main moved on, or `DEPLOY_HOST` / `API_URL` unset) deployed nothing and don't move the base. `workflow_dispatch` deploys everything, at the commit it is run on. `deploy-web` does the same for the Workers (`site/`, `apps/`, `packages/`), with one base for the site and one for the apps. A failed apps deploy fails the whole run, so the site's base falls back too and the site redeploys next time (safe). If main has moved on by the time a run starts, it skips: the newer commit's run deploys the same changes.

**Changing a production secret:** edit `.env.issho-prod` (or the certificate), then copy it to the server, which deploys with its own copy: `grep -v '^CLOUDFLARE_DNS_TOKEN=' .env.issho-prod | ssh kamal@<server> 'cat > /opt/issho/env'` (and `.kamal/secrets` to `/opt/issho/kamal-secrets`, the certificate files to `/opt/issho/certs/`), then rerun the workflow. **Changing `deploy.sh`:** it runs from `/opt/issho/deploy.sh`, not the checkout, so reinstall it after merging: `scp config/server/deploy.sh kamal@<server>:/tmp/ && ssh kamal@<server> 'sudo install -o root -m 755 /tmp/deploy.sh /opt/issho/ && rm /tmp/deploy.sh'`.

**Manual deploys** from a dev machine still work as above; Kamal then leaves your registry login on the server, so log it out afterwards (`ssh kamal@<server> docker logout ghcr.io`).

## Off-site backups

A Kamal accessory, `backup` (`config/deploy.yml`), runs `config/backup/backup.sh` in the `ghcr.io/shrujancheruku/issho-backup:17` image (`config/backup/Dockerfile`: `postgres:17-alpine`, the database's major version, with curl baked in so the container starts without Alpine's mirrors). Every night at `BACKUP_AT` (UTC hour, default 3) it `pg_dump`s the `app`, `goose`, `river` and `auth` schemas in the custom format and uploads the file to a Cloudflare R2 bucket, signed with `curl --aws-sigv4`. Live sessions and one-time tokens are left out, so a leaked dump can't open anyone's account. A failed night is logged and retried the next night.

**Alert:** each run checks in to a Sentry cron monitor, `issho-backup` (in the `issho-api` project), with `in_progress` first, then `ok` or `error`, using the server's `SENTRY_DSN` and curl. The first check-in of a run carries the monitor's config (daily at `BACKUP_AT` UTC, 60 minutes' margin, 60 minutes' maximum runtime), so Sentry creates and updates the monitor itself. A failed run (`error`), a run past its maximum, or a night with no check-in (missed) opens a monitor issue, and the alert rule below emails it. A failed check-in is logged and ignored: Sentry being unreachable never fails a backup. With no `SENTRY_DSN`, there are no check-ins.

**Owner steps to turn the alert on** (the image is built and pushed by hand, and the Sentry side is the owner's):

1. Build and push the image from the repo root (a classic PAT with `write:packages`, as for the first manual deploy; log out afterwards): `docker buildx build --platform linux/amd64 -t ghcr.io/shrujancheruku/issho-backup:17 --push config/backup`. Pushing again rebuilds on the latest `postgres:17-alpine`, so do it now and then.
2. Make sure `SENTRY_DSN` (the `issho-api` project's) is in `.env.issho-prod`, and in the server's copy of the env.
3. On the machine whose `.kamal/secrets` holds `SENTRY_DSN` (the one that deploys by hand): pulling the new private GHCR package needs `KAMAL_REGISTRY_PASSWORD` set to a PAT with `read:packages`. Run `kamal registry login` if the reboot doesn't log in itself, then `kamal accessory reboot backup` (it pulls the new image and mounts the new script). The check-in below creates the monitor.
4. Check once: `kamal accessory exec backup "env BACKUP_ONCE=1 sh /backup/backup.sh"`. Sentry, *Insights > Crons*, now lists `issho-backup` with an `ok` check-in.
5. Create the alert: in the monitor's page, *Alerts > Create alert* (or *Alerts > Create Alert > Issues*, project `issho-api`, filter *issue category is Cron*, monitor slug `issho-backup`), action *Send a notification to Member*, you, by email. Check that the monitor shows the issue-alert rule as attached.
6. Test the failure: `kamal accessory exec backup "env BACKUP_ONCE=1 R2_SECRET_ACCESS_KEY=wrong sh /backup/backup.sh"` ends in an `error` check-in and an email. A missed night is the same alert (to try it, stop the accessory past `BACKUP_AT` plus the margin: `kamal accessory stop backup`, then `start` it again).

**Live since 2026-10-26:** bucket `issho-backups` (30-day lifecycle rule) in the Cloudflare account, `BACKUP_AT` 18 (03:00 in Tokyo). The server's copy of the env holds the R2 keys. `config/deploy.yml` changes to the accessory, or to `backup.sh` or its Dockerfile, need `kamal accessory reboot backup`; the deploy workflow doesn't touch accessories.

**One-time setup** (needs the Cloudflare account):

1. Create an R2 bucket (e.g. `issho-backups`) and an **R2 API token** with *Object Read & Write* on that bucket only. Put its key pair in `.env.issho-prod` as `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` (`.kamal/secrets` reads them), and the account id and bucket in `config/deploy.yml` (`TODO`s).
2. In the bucket's settings, add a **lifecycle rule** deleting objects after 30 days: that's the retention.
3. `kamal accessory boot backup`, then check the setup once: `kamal accessory exec backup "env BACKUP_ONCE=1 sh /backup/backup.sh"`, which should print `backup: uploaded issho-….dump`. `kamal accessory logs backup` shows each night.

**Restore drill** (2026-10-26): the first dump (380 KB) was downloaded from R2 and restored in full into a throwaway `postgis/postgis:17-3.5` container (amd64 only: `--platform linux/amd64` on a Mac; use the Supabase image CI runs, `public.ecr.aws/supabase/postgres`, which is multi-arch and already has the `extensions` schema). That dump predates the baseline reset and holds PostGIS in `public`; later dumps hold it in `extensions`. It needs the extensions first: `create schema extensions; create extension postgis schema extensions; create extension pgcrypto schema extensions;`, then `pg_restore --no-owner --no-privileges -d postgres issho-….dump`. All 68 tables in `app`, `goose`, `river` and `auth` came back with the live row counts (7,139 rows), except the session tables, which are left out on purpose, and one River job queued after the dump. The drill found `auth.mfa_amr_claims` failing its foreign key without the sessions, so it's now left out too. Delete the dump afterwards: it holds people's data.

**Restoring into a new Supabase project** (not yet rehearsed): download a dump from R2, then into an empty project with the same migrations applied:

```sh
pg_restore --list issho-….dump                       # what it holds
pg_restore --no-owner --no-privileges --data-only --schema=auth --table=users --table=identities -d "$NEW_DATABASE_URL" issho-….dump
pg_restore --no-owner --no-privileges --data-only --schema=app -d "$NEW_DATABASE_URL" issho-….dump
```

Supabase creates `auth` itself, so only its data comes back (people's accounts; they sign in again). `goose` and `river` hold bookkeeping that the new project's own migrations recreate.

## Resetting a database onto the baseline migration

[#160](https://github.com/shrujancheruku/issho/issues/160) replaced migrations `00001`-`00028` with one baseline (`00001_baseline.sql`, then River's `00002_river.go`). A database that ran the old history is **reset**, not migrated: goose would see version 28 and the baseline's `00001` as already applied. Merging the PR triggers the automatic deploy, whose `migrate` step applies the baseline, so the reset happens **before the merge** and the deploy follows right after it. The API is down in between.

**issho-dev was reset with the short variant below on 2026-11-03, the owner's call: nothing was carried over.**

Run SQL with `supabase db query --linked "…"` or `psql "$DATABASE_URL"` (a session-mode URL, as everywhere else). Keep a scratch file of the output: the PostGIS version goes in the PR.

### When nothing in the database needs keeping

1. Stop the API and worker (`kamal app stop`), then drop the old schemas in one transaction. `auth.users` is not touched, so admin and translator roles (`auth.users.raw_app_meta_data`) survive:
   ```sql
   begin;
   drop schema app cascade;
   drop schema river cascade;
   drop schema goose cascade;
   drop extension postgis cascade;   -- from public
   commit;
   ```
2. Merge the PR. The deploy workflow runs `kamal migrate` (which applies `00001_baseline` and `00002_river` from nothing, creating PostGIS in `extensions`) and `kamal deploy`. `kamal app exec --primary "tool migrate status"` should show both applied.
3. Seed the packs: `kamal app exec --primary "tool seed /data/citypacks/tokyo.json /data/citypacks/austin.json /data/citypacks/da-nang-hoi-an.json"`. With `GOOGLE_MAPS_API_KEY` set this also builds the travel times; if the log says they were skipped, run `kamal app exec --primary "tool build-matrix <city>"` for each city.
4. Run the checks in *Verify* below, and record `select extversion from pg_extension where extname = 'postgis';` in the PR.

### When the data matters (production with active users)

Keeps the old rows, renaming `app` to a scratch schema and copying the reference data across.

1. **Final dump, kept.** Take one on demand and copy it somewhere the 30-day lifecycle rule won't delete it:
   ```sh
   kamal accessory exec --reuse backup "env BACKUP_ONCE=1 sh /backup/backup.sh"   # prints: backup: uploaded issho-….dump
   `--reuse` runs it in the accessory's running container; without it Kamal starts a fresh container from the configured image, which may not be pushed yet.
   In R2, copy that object to `pre-baseline/issho-….dump` and check the lifecycle rule doesn't cover the prefix (or turn it into an object with its own retention). Delete it when the reset is verified: it holds people's data. If the backup accessory isn't running yet, `pg_dump --format=custom --no-owner --no-privileges --schema=app --schema=goose --schema=river --schema=auth` with the same exclusions as `config/backup/backup.sh`, to a file outside the repo.
2. **Stop the API and worker**, so nothing writes while the schemas are swapped: `kamal app stop`. The API stays down from here until the PR merges and deploys (step 4), so do steps 2 to 4 in one sitting and have the PR approved and green first.
3. **Empty the database, keeping `app`'s old rows for step 6.** In one transaction:
   ```sql
   begin;
   -- The old app schema stays, renamed, as the scratch schema for step 6. Its two
   -- PostGIS columns go first so PostGIS can be dropped (areas are re-seeded).
   alter schema app rename to app_old;
   alter table app_old.areas drop column centroid, drop column boundary;
   drop schema river cascade;
   drop schema goose cascade;
   drop extension postgis;            -- from public; no cascade, so it fails safely if anything else still uses it
   commit;
   ```
   `auth.users` is not touched. Admin and translator roles live in `auth.users.raw_app_meta_data` (`admin`, `translator_langs`), so they survive as they are.
4. **Deploy the baseline, right after step 3.** Merge the PR: the deploy workflow runs `kamal migrate` (which applies `00001_baseline` and `00002_river` from nothing, creating PostGIS in `extensions`) and `kamal deploy`. Or run `kamal build push && kamal migrate && kamal deploy --skip-push` from the PR's commit. `kamal app exec --primary "tool migrate status"` should show both migrations applied.
5. **Seed every city pack and rebuild the travel times.** `tool seed` builds the matrix of new areas when `GOOGLE_MAPS_API_KEY` is set (about 554 elements across the three packs):
   ```sh
   kamal app exec --primary "tool seed /data/citypacks/tokyo.json /data/citypacks/austin.json /data/citypacks/da-nang-hoi-an.json"
   kamal app exec --primary "tool build-matrix tokyo"          # only if the seed log says the travel times were skipped; same for austin, da-nang-hoi-an
   ```
6. **Carry the reference data over from `app_old`**, then drop it. Venues keep their curation status and who set it and when; `added_by` is kept only where that user still exists in `auth.users`. Translations keep their review. Accounts, profiles, plans, crews, chats, notifications and reports are **not** carried over.
   ```sql
   begin;
   insert into app.venues (google_place_id, city_id, area_id, category_id, status, last_seen_at, reviewed_by, reviewed_at, added_by)
   select v.google_place_id, v.city_id, v.area_id, v.category_id, v.status, v.last_seen_at, v.reviewed_by, v.reviewed_at,
          case when exists (select 1 from auth.users u where u.id = v.added_by) then v.added_by end
   from app_old.venues v
   where exists (select 1 from app.areas a where a.id = v.area_id and a.city_id = v.city_id)
     and exists (select 1 from app.venue_categories c where c.id = v.category_id)
   on conflict do nothing;

   insert into app.translations (source_hash, lang, source, context, text, model, reviewed, created_at, reviewed_by, reviewed_at)
   select source_hash, lang, source, context, text, model, reviewed, created_at, reviewed_by, reviewed_at
   from app_old.translations
   on conflict do nothing;

   -- Run everything above up to and including this select, and compare the counts. Only then run `commit;`.
   select (select count(*) from app_old.venues) as venues_before, (select count(*) from app.venues) as venues_after,
          (select count(*) from app_old.translations) as translations_before, (select count(*) from app.translations) as translations_after;
   -- Venues in cities or areas that no longer exist in the packs are the only ones allowed to be missing.
   ```
   If the counts are right, run `commit;`, then `drop schema app_old cascade;`. If not, run `rollback;` and look again; `app_old` is untouched.
   Roles need nothing: check that the admins and translators you expect are still there, and re-run `tool set-admin <email>` / `tool set-translator <email> <langs>` for any that aren't (the dump from step 1 lists them: `auth.users.raw_app_meta_data`):
   ```sql
   select email, raw_app_meta_data->>'admin' as admin, raw_app_meta_data->>'translator_langs' as translator_langs
   from auth.users where raw_app_meta_data ? 'admin' or raw_app_meta_data ? 'translator_langs';
   ```
7. **Record the PostGIS version** and put the result in the PR: `select extversion from pg_extension where extname = 'postgis';`
8. **Verify** (also the last step of the short variant):
   - goose is at the baseline: `select version_id from goose.db_version order by id desc limit 1;` is `2`, and `app_old` and `public.app_goose_db_version` don't exist.
   - `anon` and `authenticated` have no access: `select has_schema_privilege('anon', 'app', 'usage'), has_schema_privilege('authenticated', 'river', 'usage')` is `f, f`, and `select count(*) from information_schema.role_table_grants where grantee in ('anon','authenticated') and table_schema in ('app','river')` is `0`. (`extensions` tables such as `spatial_ref_sys` are owned by `supabase_admin`; with the Data API off they aren't reachable.)
   - The Data API still answers 503: the `curl` in [supabase-hosted.md](supabase-hosted.md).
   - `supabase db advisors --linked` is clean: `0014_extension_in_public` is gone.
   - `curl https://api.joinissho.com/up`, and `HealthService/Check` reports `"databaseOk": true`.
   - A sign-in from the app creates the `app.users` row again.

**Backups from before the reset don't restore onto the baseline.** They hold the old `app` schema (`public.geography` columns, the dropped columns) and goose's 28-step history, so neither `pg_restore` of the whole dump nor a `--data-only --schema=app` restore into a database migrated to the baseline works: the column lists differ. To read old data, restore into a scratch Postgres with PostGIS in `public` and copy rows out. Dumps taken after the reset restore as described under *Off-site backups*, with the extensions in the `extensions` schema.

## To check on the first real deploy

- Group chat's WebSocket (`/ws/chat`) through Cloudflare and kamal-proxy: open a group chat in the app and send a message from another account.
- The API sees Cloudflare's address, not the client's (nothing uses client addresses yet; if something does, read `CF-Connecting-IP`).
- Point the app's and the sites' API URL at the new host: `EXPO_PUBLIC_API_URL`, and `scripts/deploy-web.sh https://<api host>`.
