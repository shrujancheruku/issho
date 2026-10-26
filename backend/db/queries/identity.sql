-- Creates the app user on first sight of a Supabase user. On later calls it only
-- refreshes the email, and keeps the stored one when the token carries none
-- (some providers, like Apple with private relay, can omit it).
-- name: UpsertUser :one
insert into app.users (id, email)
values ($1, $2)
on conflict (id) do update set email = coalesce(excluded.email, app.users.email)
returning id, email, verified_at, deleted_at, created_at;

-- name: IsSuspended :one
select exists (select 1 from app.suspensions where user_id = $1 and lifted_at is null);

-- No row = not onboarded yet. With the home city, so the app can load the
-- city's catalogue straight after GetMe instead of waiting for GetProfile.
-- name: GetDisplayLanguages :one
select p.primary_language, p.secondary_language, a.city_id
from app.profiles p join app.areas a on a.id = p.home_area_id
where p.user_id = $1;

-- Explicit delete for databases without the auth.users foreign key (CI, plain
-- Postgres). On Supabase the row is usually already gone via the cascade.
-- name: DeleteUser :exec
delete from app.users where id = $1;
