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
insert into app.outings (plan_id, round_id, date, part, starts_at, ends_at, time_zone, languages, state, created_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
returning id;

-- name: InsertSeat :exec
insert into app.seats (outing_id, round_id, account_id, state, invited_at, invitation_ends_at)
values ($1, $2, $3, $4, $5, $6);

-- An account's own seats with what their outings say, earliest date first. It
-- is by account and reads no other person's seat.
-- name: ListSeatsOfAccount :many
select o.id as outing_id, o.plan_id, o.date, o.part, o.starts_at, o.ends_at, o.time_zone, o.languages,
       o.state as outing_state, s.state, s.invited_at, s.invitation_ends_at, s.checked_in_at
from app.seats s
join app.outings o on o.id = s.outing_id
where s.account_id = $1
order by o.date, o.starts_at, o.id;

-- One of the account's own seats, the same columns.
-- name: GetSeatOfAccount :one
select o.id as outing_id, o.plan_id, o.date, o.part, o.starts_at, o.ends_at, o.time_zone, o.languages,
       o.state as outing_state, s.state, s.invited_at, s.invitation_ends_at, s.checked_in_at
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
select id, plan_id, round_id, date, part, starts_at, ends_at, time_zone, state from app.outings where id = $1 for update;

-- name: GetSeat :one
select state, invited_at, invitation_ends_at, going_at, left_going_at, checked_in_at, handle
from app.seats where outing_id = $1 and account_id = $2;

-- name: SetSeatState :exec
update app.seats set state = $3 where outing_id = $1 and account_id = $2;

-- A seat becomes going: its history starts here.
-- name: MakeSeatGoing :exec
update app.seats set state = 'going', going_at = $3 where outing_id = $1 and account_id = $2;

-- A seat going is given up: its history ends here.
-- name: LeaveSeat :exec
update app.seats set state = $3, left_going_at = $4 where outing_id = $1 and account_id = $2;

-- One tap. A second changes nothing, the instant included.
-- name: CheckInSeat :exec
update app.seats set checked_in_at = $3
where outing_id = $1 and account_id = $2 and checked_in_at is null;

-- name: SetOutingState :exec
update app.outings set state = $2 where id = $1;

-- Setting the outing makes every seat that said yes going; the accounts come
-- back so their time can be taken off their other signups.
-- name: PromoteAcceptedSeats :many
update app.seats set state = 'going', going_at = $2 where outing_id = $1 and state = 'accepted' returning account_id;

-- A seat still invited when its 48 hours end is recorded as unanswered.
-- name: ExpireUnansweredSeats :exec
update app.seats set state = 'unanswered'
where outing_id = $1 and state = 'invited' and invitation_ends_at <= $2;

-- When the last of an outing's invitations ends.
-- name: LastInvitationEnd :one
select max(invitation_ends_at)::timestamptz as ends from app.seats where outing_id = $1;

-- A person's time is one at a time: whoever is about to become going, or to be
-- turned down because another outing took their time, holds this lock on the
-- person's date and part of the day to the end of the transaction. Two outings
-- that share a person can't then both make them going. The locks are taken in
-- account order after the outing's own row, so two answers queue rather than
-- wait on each other; the key is a 64-bit hash, so two different keys can still
-- collide, and then Postgres may end one answer with a deadlock (40P01).
-- name: LockAccountTime :exec
select pg_advisory_xact_lock(hashtextextended(
  sqlc.arg(account_id)::text || '|' || sqlc.arg(date)::date::text || '|' || sqlc.arg(part)::text, 0));

-- Who has said yes to an outing that isn't set yet: those the third yes makes
-- going, whose times are locked before it does.
-- name: ListAcceptedAccounts :many
select account_id from app.seats where outing_id = $1 and state = 'accepted' order by account_id;

-- Whether the account is going to another outing on that date and part of the
-- day.
-- name: IsGoingElsewhere :one
select exists (
  select 1 from app.seats s join app.outings o on o.id = s.outing_id
  where s.account_id = $1 and s.state = 'going' and o.date = $2 and o.part = $3 and o.id <> $4
) as going;

-- A person going to an outing is a no to every other invitation of theirs for
-- that date and part of the day that is still open (invited or waiting on its
-- outing): a seat going is never touched, and an invitation whose 48 hours have
-- ended is left for the expiring job to record.
-- name: DeclineOtherOpenSeats :exec
update app.seats s set state = 'declined'
from app.outings o
where o.id = s.outing_id and s.account_id = $1 and o.date = $2 and o.part = $3 and o.id <> $4
  and s.state in ('invited', 'accepted') and s.invitation_ends_at > $5;

-- Who of an outing has checked in: for the first names the people going see.
-- name: ListCheckedIn :many
select outing_id, account_id
from app.seats
where outing_id = any(sqlc.arg(outing_ids)::uuid[]) and state = 'going' and checked_in_at is not null
order by outing_id, account_id;

-- Everyone who was going at the start of an outing, by account: a seat that
-- became going by then and had not left going by then. It is read from the
-- seat's history, so a person who cancelled after the start is among them.
-- name: ListGoingAtStart :many
select account_id, handle
from app.seats
where outing_id = $1 and going_at <= sqlc.arg(starts_at)::timestamptz
  and (left_going_at is null or left_going_at > sqlc.arg(starts_at)::timestamptz)
order by account_id;

-- name: GetFeedback :one
select went, answered_at, updated_at from app.feedback where outing_id = $1 and account_id = $2;

-- name: UpsertFeedback :exec
insert into app.feedback (outing_id, account_id, went, answered_at, updated_at)
values ($1, $2, $3, $4, $4)
on conflict (outing_id, account_id) do update set went = excluded.went, updated_at = excluded.updated_at;

-- Answers are replaced whole: whatever the person ticked and picked before is
-- cleared and what they now say is written.
-- name: ClearUnticks :exec
delete from app.feedback_unticks where outing_id = $1 and rater_id = $2;

-- name: InsertUntick :exec
insert into app.feedback_unticks (outing_id, rater_id, subject_id) values ($1, $2, $3);

-- name: ListUnticks :many
select subject_id from app.feedback_unticks where outing_id = $1 and rater_id = $2 order by subject_id;

-- name: ClearPicks :exec
delete from app.picks where outing_id = $1 and picker_id = $2;

-- name: InsertPick :exec
insert into app.picks (outing_id, picker_id, picked_id, created_at) values ($1, $2, $3, $4);

-- The caller's own picks. Nothing reads picks by who was picked.
-- name: ListPicksBy :many
select picked_id from app.picks where outing_id = $1 and picker_id = $2 order by picked_id;

-- Whether the account attended: checked in or said so in feedback, and their own
-- "I didn't go" wins over a check-in.
-- name: IsAttended :one
select coalesce(f.went, s.checked_in_at is not null)::boolean as attended
from app.seats s
left join app.feedback f on f.outing_id = s.outing_id and f.account_id = s.account_id
where s.outing_id = $1 and s.account_id = $2;
