-- The signups tables, app.plan_rounds, app.signups and app.signup_ticks; only
-- internal/plans calls these. "Today" is always the city's date, which the
-- caller works out with cityclock.

-- name: GetRound :one
select * from app.plan_rounds where plan_id = $1 and number = $2;

-- A round's dates are derived from the plan's opening day, so whoever stores
-- it first stores the same row. The no-op update makes a lost race return the
-- round the winner stored.
-- name: OpenRound :one
insert into app.plan_rounds (plan_id, number, starts_on, ends_on)
values ($1, $2, $3, $4)
on conflict (plan_id, number) do update set number = excluded.number
returning *;

-- name: UpsertSignup :exec
insert into app.signups (round_id, account_id, created_at, updated_at)
values ($1, $2, $3, $3)
on conflict (round_id, account_id) do update set updated_at = excluded.updated_at;

-- name: DeleteTicks :exec
delete from app.signup_ticks where round_id = $1 and account_id = $2;

-- name: InsertTick :exec
insert into app.signup_ticks (round_id, account_id, tick_date, part)
values ($1, $2, $3, $4);

-- Withdrawing is deleting the signup in the plan's running round; its ticks go
-- with it.
-- name: DeleteSignup :exec
delete from app.signups s
using app.plan_rounds r
where r.id = s.round_id and r.plan_id = $1 and s.account_id = $2 and r.ends_on >= sqlc.arg(today)::date;

-- The reads below take a loose lower bound, one day before the UTC date: a
-- city's today is at most a day ahead of or behind it, and the caller keeps
-- the rounds that haven't ended on the plan's own city clock.

-- A member's signups, the earliest hear-by first.
-- name: ListSignupsOfAccount :many
select r.plan_id, r.ends_on, p.city
from app.signups s
join app.plan_rounds r on r.id = s.round_id
join app.plans p on p.id = r.plan_id
where s.account_id = $1 and r.ends_on >= sqlc.arg(since)::date
order by r.ends_on, r.plan_id;

-- name: ListTicksOfAccount :many
select r.plan_id, r.ends_on, t.tick_date, t.part
from app.signup_ticks t
join app.plan_rounds r on r.id = t.round_id
where t.account_id = $1 and r.ends_on >= sqlc.arg(since)::date
order by r.plan_id, t.tick_date, t.part;

-- How many are signed up to each round of the plans. Never who.
-- name: CountSignups :many
select r.plan_id, r.ends_on, p.city, count(*) as signed_up
from app.signups s
join app.plan_rounds r on r.id = s.round_id
join app.plans p on p.id = r.plan_id
where r.plan_id = any(sqlc.arg(plan_ids)::uuid[]) and r.ends_on >= sqlc.arg(since)::date
group by r.plan_id, r.ends_on, p.city;

-- The ticks of one round from a date on, by person: what forming reads. A
-- signup always has a tick, so a person with none from the date on has none
-- left to form with.
-- name: ListRoundTicks :many
select account_id, tick_date, part
from app.signup_ticks
where round_id = $1 and tick_date >= sqlc.arg(from_date)::date
order by account_id, tick_date, part;

-- A person who is going to an outing is busy then: the same date and part of
-- the day is taken off their signups to every round but the outing's own.
-- name: DeleteOtherTicksAt :exec
delete from app.signup_ticks
where account_id = $1 and tick_date = $2 and part = $3 and round_id <> $4;

-- A signup always has a tick, so one left with none is withdrawn. Only the
-- account's signups to rounds but the outing's own are looked at.
-- name: DeleteEmptySignups :exec
delete from app.signups s
where s.account_id = $1 and s.round_id <> $2
  and not exists (select 1 from app.signup_ticks t where t.round_id = s.round_id and t.account_id = s.account_id);
