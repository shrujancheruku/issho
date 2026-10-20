-- name: UpsertUser :one
insert into app.users (id, email)
values ($1, $2)
on conflict (id) do update set email = coalesce(excluded.email, app.users.email)
returning id, email, verified_at, deleted_at, created_at;

-- name: HasProfile :one
select exists (select 1 from app.profiles where user_id = $1);
