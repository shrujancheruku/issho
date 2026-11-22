-- Join links (app.join_links) and admission (app.admissions); only
-- internal/people calls these.

-- Locks the member's account row for the length of the transaction, so two
-- links made at once count one another.
-- name: LockAccount :one
select id from app.accounts where id = $1 for update;

-- A member's links that count against their 3: used, or outstanding and not
-- yet expired. An expired or cancelled one has returned. $2 is the oldest
-- created_at that has not expired.
-- name: CountHeldJoinLinks :one
select count(*) from app.join_links
where member_id = $1
  and cancelled_at is null
  and (redeemed_at is not null or created_at > $2);

-- name: InsertJoinLink :one
insert into app.join_links (member_id, code, created_at)
values ($1, $2, $3)
returning *;

-- The member's own links that haven't been cancelled, newest first.
-- name: ListJoinLinks :many
select * from app.join_links
where member_id = $1 and cancelled_at is null
order by created_at desc, id;

-- Only the member's own link, only while it is outstanding: not used, not
-- cancelled and not expired ($4 is the oldest created_at that has not expired).
-- name: CancelJoinLink :one
update app.join_links set cancelled_at = $3
where id = $1 and member_id = $2
  and cancelled_at is null and redeemed_at is null and created_at > $4
returning id;

-- Uses a link: the one statement that decides one use, so two accounts holding
-- the same code can't both win it.
-- name: UseJoinLink :one
update app.join_links set redeemed_at = $2
where code = $1
  and cancelled_at is null and redeemed_at is null and created_at > $3
returning id, member_id;

-- name: IsAdmitted :one
select exists (select 1 from app.admissions where account_id = $1);

-- name: AdmitByJoinLink :exec
insert into app.admissions (account_id, admitted_at, via, brought_by, link_id)
values ($1, $2, 'join-link', $3, $4);

-- The seed's admission: no link and nobody who brought them. Running it again
-- changes nothing.
-- name: AdmitBySeed :exec
insert into app.admissions (account_id, admitted_at, via)
values ($1, $2, 'seed')
on conflict (account_id) do nothing;

-- name: GetBroughtInBy :one
select brought_by from app.admissions where account_id = $1;

-- name: GetAccountIDByEmail :one
select id from app.accounts where email = $1;
