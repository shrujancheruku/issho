-- The accounts table, app.accounts; only internal/people calls these.

-- Idempotent on the id: a second first-call returns the row as it is.
-- name: InsertAccount :one
insert into app.accounts (id, email, city, display_language, created_at)
values ($1, $2, $3, $4, $5)
on conflict (id) do update set id = excluded.id
returning *;

-- name: GetAccount :one
select * from app.accounts where id = $1;
