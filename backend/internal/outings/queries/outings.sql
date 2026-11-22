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
select o.id as outing_id, o.plan_id, o.date, o.part, o.starts_at, o.languages, o.state as outing_state,
       s.state, s.invited_at, s.invitation_ends_at
from app.seats s
join app.outings o on o.id = s.outing_id
where s.account_id = $1
order by o.date, o.starts_at, o.id;

-- One of the account's own seats, the same columns.
-- name: GetSeatOfAccount :one
select o.id as outing_id, o.plan_id, o.date, o.part, o.starts_at, o.languages, o.state as outing_state,
       s.state, s.invited_at, s.invitation_ends_at
from app.seats s
join app.outings o on o.id = s.outing_id
where s.account_id = $1 and o.id = $2;

-- How many in each outing have said yes: those going and those whose yes waits
-- for the outing to be set. A number, never who.
-- name: CountYeses :many
select outing_id, count(*)::int as yeses
from app.seats
where outing_id = any(sqlc.arg(outing_ids)::uuid[]) and state in ('accepted', 'going')
group by outing_id;

-- Who is going to each outing: for the first names a person going sees.
-- name: ListGoing :many
select outing_id, account_id
from app.seats
where outing_id = any(sqlc.arg(outing_ids)::uuid[]) and state = 'going'
order by outing_id, account_id;

-- Answering is one at a time per outing: the row lock is held to the end of the
-- transaction, so three people saying yes at once are counted one after the
-- other and the third sets the outing, once.
-- name: LockOuting :one
select id, plan_id, round_id, date, part, state from app.outings where id = $1 for update;

-- name: GetSeat :one
select state, invited_at, invitation_ends_at from app.seats where outing_id = $1 and account_id = $2;

-- name: SetSeatState :exec
update app.seats set state = $3 where outing_id = $1 and account_id = $2;

-- name: SetOutingState :exec
update app.outings set state = $2 where id = $1;

-- Setting the outing makes every seat that said yes going; the accounts come
-- back so their time can be taken off their other signups.
-- name: PromoteAcceptedSeats :many
update app.seats set state = 'going' where outing_id = $1 and state = 'accepted' returning account_id;

-- A seat still invited when its 48 hours end is recorded as unanswered.
-- name: ExpireUnansweredSeats :exec
update app.seats set state = 'unanswered'
where outing_id = $1 and state = 'invited' and invitation_ends_at <= $2;

-- When the last of an outing's invitations ends.
-- name: LastInvitationEnd :one
select max(invitation_ends_at)::timestamptz as ends from app.seats where outing_id = $1;
