-- The outings tables, app.outings and app.seats; only internal/outings calls
-- these.

-- Forming a round is one at a time: two runs for one round queue behind this
-- lock, held to the end of the transaction, so the second sees the first's
-- seats and forms nothing twice. The key is the round's id.
-- name: LockRound :exec
select pg_advisory_xact_lock(hashtextextended(sqlc.arg(round_id)::text, 0));

-- Everyone who already has a seat in a round, whatever has become of it.
-- name: ListSeatedInRound :many
select account_id from app.seats where round_id = $1;

-- name: InsertOuting :one
insert into app.outings (plan_id, round_id, date, part, starts_at, languages, state, created_at)
values ($1, $2, $3, $4, $5, $6, $7, $8)
returning id;

-- name: InsertSeat :exec
insert into app.seats (outing_id, round_id, account_id, state, invited_at, invitation_ends_at)
values ($1, $2, $3, $4, $5, $6);

-- An account's own seats with what their outings say, earliest date first. It
-- is by account and reads no other person's seat.
-- name: ListSeatsOfAccount :many
select o.id as outing_id, o.plan_id, o.date, o.part, o.starts_at, o.languages,
       s.state, s.invited_at, s.invitation_ends_at
from app.seats s
join app.outings o on o.id = s.outing_id
where s.account_id = $1
order by o.date, o.starts_at, o.id;
