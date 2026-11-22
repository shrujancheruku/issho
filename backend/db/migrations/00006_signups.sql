-- +goose Up

-- A signup: a person's ticks on a plan, in one round of it. `plans` owns these
-- tables; no other package reads them (docs/spec/architecture.md, "The seams").
-- A signup commits a person to nothing, so it can be changed or withdrawn
-- until matching reads the round (docs/spec/core-loop.md).

-- A plan's round: a fixed 7 days, the same for everyone signed up, and the
-- last day is the day they hear by (docs/spec/matching.md, "Rounds"). The thin
-- loop opens a round when someone signs up and the plan has none running;
-- rounds on the real clock, with a state of their own, are milestone 4's and
-- will add columns here, not a second table.
create table app.plan_rounds (
  id        uuid primary key default gen_random_uuid(),
  plan_id   uuid not null references app.plans (id) on delete cascade,
  -- City calendar dates. The first day is the day the round opened and the
  -- last is the hear-by day.
  starts_on date not null,
  ends_on   date not null check (ends_on > starts_on),
  -- Two signups racing to open the same plan's round on the same day open one.
  constraint plan_rounds_plan_id_starts_on_key unique (plan_id, starts_on)
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
