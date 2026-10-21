-- Creates the app user on first sight of a Supabase user. On later calls it only
-- refreshes the email, and keeps the stored one when the token carries none
-- (some providers, like Apple with private relay, can omit it).
-- name: UpsertUser :one
insert into app.users (id, email)
values ($1, $2)
on conflict (id) do update set email = coalesce(excluded.email, app.users.email)
returning id, email, verified_at, deleted_at, created_at;

-- No row = not onboarded yet.
-- name: GetDisplayLanguages :one
select primary_language, secondary_language from app.profiles where user_id = $1;

-- Explicit delete for databases without the auth.users foreign key (CI, plain
-- Postgres). On Supabase the row is usually already gone via the cascade.
-- name: DeleteUser :exec
delete from app.users where id = $1;
