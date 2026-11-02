# Research: moving PostGIS to the `extensions` schema on hosted Supabase

Issue [#141](https://github.com/shrujancheruku/issho/issues/141). Researched 2026-11-02 against main at `24609b6` (migrations up to 00028). This branch is never merged; anything adopted moves into a real migration, `docs/ROADMAP.md` and the READMEs.

## Answer in brief

- **Use one goose migration (00029) that drops PostGIS and creates it again in `extensions`, keeping the data by converting the two PostGIS columns to `bytea` (EWKB) first and back afterwards.** It runs as the `postgres` role on Supabase, in goose's normal transaction (no `-- +goose NO TRANSACTION`), and it's the same migration for local, CI, `issho-dev` and production. Tested end to end on Supabase's Postgres image (below).
- `ALTER EXTENSION postgis SET SCHEMA` does **not** work: PostGIS isn't relocatable since 2.3. PostGIS's own workaround edits `pg_extension`, which `postgres` isn't allowed to do on Supabase.
- Only **`app.areas.centroid`** and **`app.areas.boundary`** depend on PostGIS. There are no spatial indexes, views, triggers or SQL functions that use it, and no other extension depends on it on Supabase.
- **The API needs no change.** On Supabase, the `postgres` role's `search_path` already includes `extensions`, so the unqualified `st_x`, `geography` and similar names in `backend/db/queries` keep working. **sqlc needs one change:** the override has to name `extensions.geography`.
- **CI's image needs a change.** The migration fails on `postgis/postgis:17-3.5`, because that image also installs `postgis_topology` and `postgis_tiger_geocoder`, has no `extensions` schema and doesn't put it on the `search_path`. The owner decides how to fix this (see open questions).
- **The migration locks `app.areas` (`ACCESS EXCLUSIVE`) for the whole transaction: about 0.3 s at today's size**, mostly spent in `CREATE EXTENSION`. With 20,000 areas with boundaries (53 MB), it takes 2.8 s. The size of the rest of the database doesn't matter.
- Restart the API and the worker after it runs (a normal Kamal deploy does). Connections opened before the move fail for any SRID they haven't used yet.
- Asking Supabase Support to move the extension (Supabase's "Option 1") also works, but only on a hosted project. Local, CI and new databases would still need the migration, so it doesn't replace the migration.

## What depends on PostGIS today

From the repo and from `pg_depend` on a freshly migrated and seeded database:

| Object | Kind | Where it comes from |
|---|---|---|
| `create extension if not exists postgis` (no schema, so it lands in `public`) | extension | [00001_init.sql](../../backend/db/migrations/00001_init.sql) line 2 |
| `app.areas.centroid geography(point, 4326) not null` | column | 00001 |
| `app.areas.boundary geography(multipolygon, 4326)` (nullable) | column | [00027_area_boundaries.sql](../../backend/db/migrations/00027_area_boundaries.sql) |
| Spatial (GiST) indexes | none | `areas` has only `areas_pkey` and `areas_city_idx` (btree on `city_id`) |
| Views, SQL functions or triggers using PostGIS | none | `pg_proc` and `pg_views` outside the extension: no matches |
| Extensions depending on `postgis` | none on Supabase | the KB's dependency query returns no rows |

(Migration 00002_geo, which the ticket mentions, uses no PostGIS types or functions. It only adds travel modes, venue categories and venues.)

These functions are used, all unqualified, so they're resolved through `search_path`:

- `st_x`, `st_y` and `::geometry` in [geo.sql](../../backend/db/queries/geo.sql), [catalog.sql](../../backend/db/queries/catalog.sql), [routes.sql](../../backend/db/queries/routes.sql), [review.sql](../../backend/db/queries/review.sql) and [curation.sql](../../backend/db/queries/curation.sql)
- `st_asgeojson` in geo.sql and curation.sql. [internal/geo/store.go](../../backend/internal/geo/store.go) parses its output.
- `st_setsrid`, `st_makepoint`, `::geography` and `st_geogfromtext` in [seed.sql](../../backend/db/queries/seed.sql)
- `st_point` in curation/service_test.go

The `search_path` setup today:

- [platform/db/db.go](../../backend/internal/platform/db/db.go) sets no `search_path`. The pool uses the role's default. App tables are always written as `app.`, goose uses `goose.db_version` and River uses its `river` schema, so only PostGIS names rely on the path.
- The hosted database URL is the **session pooler**, as role `postgres.<ref>` (see [supabase-hosted.md](../supabase-hosted.md)). That connects as `postgres`, so the role-level setting applies.
- `supabase/config.toml` `extra_search_path = ["public", "extensions"]` is for the Data API (PostgREST), which is disabled. It doesn't affect pgx.
- [sqlc.yaml](../../backend/sqlc.yaml) overrides `db_type: geography` to `string`.

## Findings

### 1. `ALTER EXTENSION postgis SET SCHEMA` doesn't work, and PostGIS's workaround is closed to us

- Postgres only allows `ALTER EXTENSION … SET SCHEMA` for extensions that are relocatable, or that list the schema they need in their control file ([Postgres 17: packaging related objects into an extension, "Extension Relocatability"](https://www.postgresql.org/docs/17/extend-extensions.html#EXTEND-EXTENSIONS-RELOCATION); [ALTER EXTENSION](https://www.postgresql.org/docs/17/sql-alterextension.html)).
- PostGIS turned relocation off in 2.3; its upgrade scripts refer to the install schema through `@extschema@` ([PostGIS tip: move PostGIS schema](https://postgis.net/documentation/tips/tip-move-postgis-schema/)).
- PostGIS's documented workaround is `UPDATE pg_extension SET extrelocatable = true`, then `SET SCHEMA`, then a forced `ALTER EXTENSION postgis UPDATE TO "ANY"` (3.5+) or `"3.4.3next"` (3.4 and older), then `UPDATE` ([same tip](https://postgis.net/documentation/tips/tip-move-postgis-schema/)). It writes to a system catalog.
- **Tested** on `public.ecr.aws/supabase/postgres:17.11.0.003`, the image `supabase start` runs here. There `postgres` isn't a superuser:
  - the catalog update fails with `permission denied for table pg_extension`;
  - `alter extension postgis set schema …` fails with `extension "postgis" does not support SET SCHEMA`.
- Supabase's KB says the same: "PostGIS doesn't support moving to another schema", and the generic PostGIS procedure isn't one you can run yourself ([Supabase KB: Security Advisor flags spatial_ref_sys and other PostGIS objects in the public schema](https://supabase.com/kb/troubleshooting/security-advisor-flags-spatial_ref_sys-and-other-postgis-objects-in-the-public-schema)).

### 2. Supabase's guidance, the advisor and roles

- The advisor lint is `0014_extension_in_public`. Its generic fix is `alter extension <ext> set schema <schema>` and it doesn't mention PostGIS ([Database Advisors, lint 0014](https://supabase.com/docs/guides/database/database-advisors?queryGroups=lint&lint=0014_extension_in_public)). For PostGIS, the KB above is the authoritative page. It lists these findings: Extension in Public, RLS Disabled on `public.spatial_ref_sys`, SECURITY DEFINER `st_estimatedextent`, and GraphQL visibility. It says they "aren't false positives" while PostGIS stays in `public`.
- **Ownership:** "Supabase creates extensions as the `supabase_admin` role, so `supabase_admin` owns `spatial_ref_sys` and every other PostGIS object" (KB). That's why `postgres` can't revoke their grants. This matches decision-log entry 2026-10-20 in the ROADMAP.
- **Does creating or dropping `postgis` need `supabase_admin`?** No. `postgis` is in `supautils.privileged_extensions`, so `postgres` can create and drop it, and supautils creates it as the superuser. Locally (same image): `postgis 3.3.7 | owner supabase_admin`, and `postgres` ran `drop extension postgis` and `create extension postgis with schema extensions` without errors. The KB's "Option 2" assumes the same.
- **The KB's two options:**
  - **Option 1, ask Support.** They move it "without dropping it … in a single transaction", keeping columns, data and indexes, with no restart. This is presumably the catalog trick, run as a superuser.
  - **Option 2, drop and re-create it yourself.** The KB warns: "Dropping PostGIS deletes every column that uses a PostGIS type … Use this method only if your tables have no PostGIS columns, or if you can restore them from a backup." Its own script is `drop extension postgis cascade; create extension postgis with schema extensions;`, inside a transaction, after copying `spatial_ref_sys`.
- **Our version of Option 2 doesn't lose the columns.** Converting `centroid` and `boundary` to `bytea` (EWKB) before the drop removes their dependency on PostGIS. The columns survive with their position and `not null`. A plain `drop extension postgis` (no `cascade`) then fails safely if anything else still depends on it.
- **After either option** (KB): restart the connection pool. Connections that used PostGIS before still look for `public.spatial_ref_sys`. `grant usage on schema extensions to supabase_functions_admin` is only needed for Database Webhooks, which Issho doesn't use. To prevent the problem on new databases, use `create extension … with schema extensions`.
- `anon` has `USAGE` on `extensions` (checked locally). That's harmless: the Data API is disabled, and `extensions` isn't an exposed schema anyway (KB).

### 3. `search_path`: pgx pool, sqlc, migrations

- **Supabase roles** (checked locally with `pg_db_role_setting`): `postgres` has `search_path = "$user", public, extensions`, and `supabase_admin` has `"$user", public, auth, extensions`. The KB confirms "The postgres role and the Data API include extensions in their search path". The API, the worker, `cmd/tool` and goose all connect as `postgres` through the session pooler, so **unqualified PostGIS names keep resolving and the pgx pool needs no `search_path` change.**
  - Only roles we create ourselves with `login` would need `alter role … set search_path = "$user", public, extensions` (KB). We have none.
  - We don't need to set `search_path` in pgx's `RuntimeParams` either. It would also be untested through Supavisor.
- **Already-open connections:** PostGIS caches each backend's spatial_ref_sys lookups.
  - **Tested:** a session opened while PostGIS was in `public`, then the migration ran. In that session, `st_y`, `st_asgeojson`, geography `st_distance` and `st_transform(…, 3857)` still worked, because those SRIDs were cached or built in. `st_transform(…, 32654)`, an SRID it hadn't used yet, failed with `relation "public.spatial_ref_sys" does not exist`, as the KB says.
  - pgx's prepared statements re-planned without errors.
  - So restart the API and the worker after the migration.
- **sqlc** reads the migrations as its schema.
  - **Tested:** with the column types written as `extensions.geography(...)`, `make gen` changes `AppArea.Centroid` from `string` to `interface{}`, because the override `db_type: geography` stops matching.
  - Changing the override to `db_type: extensions.geography` restores the generated code byte for byte, and `sqlc vet` and `sqlc compile` pass.
  - Queries need no change: sqlc doesn't type-check PostGIS function calls.
  - `AppArea.Boundary` is `interface{}` before and after. It's nullable and the override isn't, which is unrelated to this change.
- **Migrations:** write the new types and functions schema-qualified (`extensions.`), so the migration doesn't depend on any `search_path` (CI's has no `extensions`).

### 4. One goose migration, transaction, lock and duration

- **One migration, inside goose's default transaction.** `DROP EXTENSION` and `CREATE EXTENSION` are transactional, so if any statement fails, everything rolls back and PostGIS stays in `public`. That's exactly what we want, so `-- +goose NO TRANSACTION` would be a mistake here ([goose annotations](https://pressly.github.io/goose/documentation/annotations/)).
- **Locks:**
  - `ALTER TABLE … ALTER COLUMN … TYPE` takes `ACCESS EXCLUSIVE` on `app.areas` and rewrites it and its indexes ([ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html); [table-level locks](https://www.postgresql.org/docs/17/explicit-locking.html#LOCKING-TABLES)). Observed in `pg_locks`: `AccessExclusiveLock` and `ShareLock` on `app.areas` until commit.
  - `DROP EXTENSION` takes `ACCESS EXCLUSIVE` on PostGIS's own relations (`spatial_ref_sys`, the views).
  - The locks last until commit. Every query that reads `areas` (catalog, matching, routes, review, curation, geo) waits that long. Nothing else is blocked.
- **`lock_timeout`:** an `ACCESS EXCLUSIVE` request that waits behind a long reader makes every later query on `areas` queue behind it. `set local lock_timeout = '5s'` makes the migration fail and roll back instead ([lock_timeout](https://www.postgresql.org/docs/17/runtime-config-client.html#GUC-LOCK-TIMEOUT)).
- **Measured** with goose on the Supabase image, where it's reported as the migration's time:

| Data in `app.areas` | Up |
|---|---|
| empty (fresh database) | 0.25 s |
| 37 areas, 15 with boundaries (all three city packs seeded; about today's `issho-dev`) | 0.26–0.33 s |
| 20,037 areas, 20,000 with a 65-vertex boundary (53 MB) | 2.76 s |

- At today's size, the time is spent in `CREATE EXTENSION` (functions plus 8,500 `spatial_ref_sys` rows), not in our data.
- `areas` is reference data that grows with cities, not with users, so a launch-sized database is still in the first two rows. The 23 MB size of `issho-dev` (users, plans, chat, River) doesn't matter.

### 5. CI: `postgis/postgis` differs from Supabase

- **Tested** on `postgis/postgis:17-3.5`, CI's image:
  - The image's init installs `postgis 3.5.2`, `postgis_topology`, `postgis_tiger_geocoder` and `fuzzystrmatch`.
  - Its `search_path` is `"$user", public, topology, tiger`, and `postgres` is a superuser.
  - The migration fails at `drop extension postgis` with `cannot drop extension postgis because other objects depend on it`, from topology and tiger.
  - Even without those extensions, it has no `extensions` schema, and the API's unqualified queries wouldn't find PostGIS.
- The two images already differ in other ways: PostGIS 3.5.2 in CI against 3.3.7 locally (and probably on hosted), and CI runs as a superuser.
- `public.ecr.aws/supabase/postgres:17.11.0.003` starts with only `POSTGRES_PASSWORD` set, and it's multi-arch (amd64 and arm64). It's 1.75 GB, against 889 MB for `postgis/postgis`.
- With 00029 in place, every migration ran up → down → up on the Supabase image, and every pack seeded. All `-run Integration` packages passed: geo, profile, plans, crews, chat, chatws, moderation, translations, notifications, cities, citypacks and curation.

### 6. Backups and restores

- After the move, `pg_dump --schema=app` writes `extensions.geography(Point,4326)` (tested). Before the move, dumps say `public.geography`.
- The drill recipe in [deploy.md](../deploy.md) (`create extension postgis schema extensions`) matches dumps taken **after** the move. A full restore of an older dump needs PostGIS in `public`.
- The planned M10 path is schema from migrations, then `pg_restore --data-only --schema=app`. It works either way: geography's text form is hex EWKB, whatever the schema.

## Recommended procedure

1. **Add the migration** `backend/db/migrations/00029_postgis_extensions.sql`. This is the tested version, plus `lock_timeout` and comments:

   ```sql
   -- +goose Up
   -- PostGIS moves from public to extensions (Supabase advisor 0014, ROADMAP
   -- known gap). It can't be relocated in place (not relocatable since 2.3, and
   -- postgres can't edit pg_extension on Supabase), so it is dropped and created
   -- again. The two PostGIS columns are kept as EWKB meanwhile, so they keep their
   -- data, position and not-null; dropping without cascade fails (and rolls
   -- back) if anything else still depends on PostGIS.
   set local lock_timeout = '5s';
   alter table app.areas
     alter column centroid type bytea using st_asewkb(centroid::geometry),
     alter column boundary type bytea using st_asewkb(boundary::geometry);
   drop extension postgis;
   create extension postgis with schema extensions;
   alter table app.areas
     alter column centroid type extensions.geography(point, 4326)
       using extensions.st_geomfromewkb(centroid)::extensions.geography,
     alter column boundary type extensions.geography(multipolygon, 4326)
       using extensions.st_geomfromewkb(boundary)::extensions.geography;

   -- +goose Down
   set local lock_timeout = '5s';
   alter table app.areas
     alter column centroid type bytea using extensions.st_asewkb(centroid::extensions.geometry),
     alter column boundary type bytea using extensions.st_asewkb(boundary::extensions.geometry);
   drop extension postgis;
   create extension postgis with schema public;
   alter table app.areas
     alter column centroid type public.geography(point, 4326)
       using public.st_geomfromewkb(centroid)::public.geography,
     alter column boundary type public.geography(multipolygon, 4326)
       using public.st_geomfromewkb(boundary)::public.geography;
   ```

   (The `lock_timeout` line wasn't part of the timed runs. It's standard Postgres and is scoped to goose's transaction by `set local`.)

   - **Leave 00001 alone.** It's applied everywhere. Fresh databases get PostGIS in `public` from 00001 and then have it moved by 00029, so every environment follows the same history.
   - The `spatial_ref_sys` copy in the KB isn't needed. We use only SRID 4326 and never added SRIDs, and the re-created table is the pristine one PostGIS ships. It would also re-add any rows written through the Data API before it was disabled on 2026-10-20.

2. **sqlc:** change the override in `backend/sqlc.yaml` to `db_type: extensions.geography`, then run `make gen`. There should be no diff in `gen/`.
3. **CI:** make the `backend` job's database match Supabase (see open question 1).
4. **Docs:**
   - ROADMAP: remove the known gap and add a decision-log row.
   - supabase-hosted.md: the advisors should now be clean.
   - deploy.md: the restore note about dumps taken before and after the move.
   - architecture.md: "Postgres + PostGIS (`extensions` schema)".
5. **Roll out to `issho-dev`** with a normal deploy (`kamal migrate` runs it, then new containers start). Old containers keep running for about a minute in between; the app only uses SRID 4326, which is already cached on warm connections, so the risk is low. Then:
   - run the KB check `select extnamespace::regnamespace from pg_extension where extname = 'postgis'`, which should return `extensions`;
   - rerun the Security Advisor; `extension_in_public` should be gone;
   - check one geo read, e.g. the Cities page.
6. **Production (M10):** if the production project is migrated from scratch and then gets a data-only restore (as deploy.md plans), 00029 runs on an empty database in about 0.25 s, before any user data exists.

## Open questions for the owner

1. **CI database image:** which fix?
   - **Switch the `backend` job to `public.ecr.aws/supabase/postgres:<same tag as the CLI>`** (recommended). It matches hosted: non-superuser `postgres`, the `extensions` schema and `search_path`, and the PostGIS version. It's arm64 too, which closes the "no arm64 image" known gap. It costs a 1.75 GB pull against 889 MB, partly hidden by the background start.
   - **Keep `postgis/postgis` and prepare it in a step:** drop `postgis_tiger_geocoder`, `postgis_topology` and `fuzzystrmatch`, `create schema extensions`, `alter role postgres set search_path = "$user", public, extensions`. Smaller, but it's a hand-made imitation that can drift.
2. **Do it ourselves (migration 00029) or ask Support (KB Option 1)?** This research recommends the migration: it's reproducible in every environment, tested, and loses no data. Support would only cover a hosted project.
3. **Timing:** land it before seeding Da Nang and Hoi An on `issho-dev`, or after? It doesn't matter technically. It has to be before M10.
4. **The PostGIS version on hosted projects.** Local is 3.3.7 and CI is 3.5.2. Run `select extversion from pg_extension where extname = 'postgis'` on `issho-dev` once. The migration has no version-specific steps, unlike PostGIS's own workaround.
5. **Optional cleanup:** keep `public` and `extensions` in `supabase/config.toml`'s `extra_search_path`? It only matters to the disabled Data API, so it can stay as is.

## Sources

- Supabase KB: [Security Advisor flags spatial_ref_sys and other PostGIS objects in the public schema](https://supabase.com/kb/troubleshooting/security-advisor-flags-spatial_ref_sys-and-other-postgis-objects-in-the-public-schema)
- Supabase: [Database Advisors, lint 0014_extension_in_public](https://supabase.com/docs/guides/database/database-advisors?queryGroups=lint&lint=0014_extension_in_public); [Postgres extensions overview](https://supabase.com/docs/guides/database/extensions)
- PostGIS: [Tip: move PostGIS schema](https://postgis.net/documentation/tips/tip-move-postgis-schema/)
- PostgreSQL 17: [Extension relocatability](https://www.postgresql.org/docs/17/extend-extensions.html#EXTEND-EXTENSIONS-RELOCATION); [ALTER EXTENSION](https://www.postgresql.org/docs/17/sql-alterextension.html); [ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html); [Table-level locks](https://www.postgresql.org/docs/17/explicit-locking.html#LOCKING-TABLES); [lock_timeout](https://www.postgresql.org/docs/17/runtime-config-client.html#GUC-LOCK-TIMEOUT)
- goose: [annotations (NO TRANSACTION)](https://pressly.github.io/goose/documentation/annotations/)
- Experiments, run 2026-11-02 in throwaway containers, now removed:
  - `public.ecr.aws/supabase/postgres:17.11.0.003`: role settings, privileges, the migration up, down and up again, timings, a session that stayed open across the move, sqlc, and the integration suite;
  - `postgis/postgis:17-3.5` (amd64): the CI failure.
