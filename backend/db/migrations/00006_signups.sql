-- +goose Up

-- A signup: a person's ticks on a plan, in one round of it. `plans` owns these
-- tables; no other package reads them (docs/spec/architecture.md, "The seams").
-- A signup commits a person to nothing, so it can be changed or withdrawn
-- until matching reads the round (docs/spec/core-loop.md).

-- The day a plan opened on its city's calendar. A plan's rounds are 7-day
-- blocks counted from it (docs/spec/matching.md, "Rounds"), so which round a
-- date falls in never depends on who signed up when. A plan that was already
-- there opened on the day this migration ran, on its city's clock.
alter table app.plans add column opens_on date;
update app.plans p set opens_on = (now() at time zone c.time_zone)::date
from app.cities c where c.slug = p.city;
alter table app.plans alter column opens_on set not null;

-- A plan's round: a fixed 7 days, the same for everyone signed up, and the
-- last day is the day they hear by. Round `number` of a plan covers
-- opens_on + 7 * number to opens_on + 7 * number + 6, so its dates are
-- derived; the row is only stored, at the first signup into the round, to hang
-- signups on. Rounds on the real clock, with a state of their own, are
-- milestone 4's and will add columns here, not a second table.
create table app.plan_rounds (
  id        uuid primary key default gen_random_uuid(),
  plan_id   uuid not null references app.plans (id) on delete cascade,
  number    integer not null check (number >= 0),
  -- City calendar dates, derived from the plan's opening day and the number.
  -- The last is the hear-by day.
  starts_on date not null,
  ends_on   date not null check (ends_on > starts_on),
  -- Two first signups racing to store the same round store one.
  constraint plan_rounds_plan_id_number_key unique (plan_id, number)
);

-- One signup per person per plan round. Matching leaves out a person whose two
-- entries differ rather than guessing between them (docs/spec/matching.md and
-- the matching package), so the database never lets a round hold two.
create table app.signups (
  round_id   uuid not null references app.plan_rounds (id) on delete cascade,
  account_id uuid not null references app.accounts (id) on delete cascade,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  primary key (round_id, account_id)
);

-- The parts of the day a person ticked on real dates, in the city's clock
-- hours. Whether a plan allows a part, and how far ahead a date is, are the
-- API's to check: both move with the clock, which a constraint can't read.
create table app.signup_ticks (
  round_id   uuid not null,
  account_id uuid not null,
  tick_date  date not null,
  part       text not null check (part in ('morning', 'afternoon', 'evening')),
  primary key (round_id, account_id, tick_date, part),
  foreign key (round_id, account_id) references app.signups (round_id, account_id) on delete cascade
);

-- The member's own signups are read by account.
create index signups_account_id_idx on app.signups (account_id);

-- +goose Down

drop table app.signup_ticks;
drop table app.signups;
drop table app.plan_rounds;
alter table app.plans drop column opens_on;
