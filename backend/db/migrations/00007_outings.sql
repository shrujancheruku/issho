-- +goose Up

-- An outing: one group on one date for a plan, formed from one round of it
-- (docs/spec/outings.md). `outings` owns these tables; no other package reads
-- them (docs/spec/architecture.md, "The seams"). A group has no life before or
-- after its outing, so the people of one are its seats.
create table app.outings (
  id         uuid primary key default gen_random_uuid(),
  plan_id    uuid not null references app.plans (id) on delete cascade,
  -- The round whose signups made the group. Whoever formed it, forming the
  -- same round twice must not make a second outing, and a seat is held once
  -- per round (below), so the round is stored beside the plan.
  round_id   uuid not null references app.plan_rounds (id) on delete cascade,
  -- The city's calendar date, the part of the day and the plan's start time
  -- ("HH:MM", the city's clock) as they were when the group was formed, so a
  -- later change to a plan can't change what an invitation said.
  date       date not null,
  part       text not null check (part in ('morning', 'afternoon', 'evening')),
  starts_at  text not null check (starts_at ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
  -- Every spoken language all of the group share; never empty, since matching
  -- forms no group without one.
  languages  text[] not null check (cardinality(languages) >= 1),
  -- The thin loop forms an outing already offered: the owner's approval of a
  -- proposed one, and the rest, come with the outing in full.
  state      text not null check (state in ('proposed', 'offered', 'set', 'over', 'not_set', 'cancelled')),
  created_at timestamptz not null,
  -- A seat must be in the round of its outing; see app.seats.
  constraint outings_id_round_id_key unique (id, round_id)
);

-- A seat: one person's place in one outing. It holds their stage, or how they
-- left. An invitation is a seat in the `invited` state, open for a fixed 48
-- hours (docs/spec/outings.md, "Invited").
create table app.seats (
  outing_id          uuid not null,
  round_id           uuid not null,
  account_id         uuid not null references app.accounts (id) on delete cascade,
  state              text not null check (state in (
    'invited', 'going', 'declined', 'unanswered', 'full', 'cancelled', 'late_cancel', 'removed')),
  -- When the invitation went out, and when its 48 hours end.
  invited_at         timestamptz not null,
  invitation_ends_at timestamptz not null check (invitation_ends_at > invited_at),
  primary key (outing_id, account_id),
  foreign key (outing_id, round_id) references app.outings (id, round_id) on delete cascade,
  -- Nobody is in two groups of one round: a person whose seat is already made,
  -- whatever has happened to it since, is not grouped again by a second run of
  -- the same forming (docs/spec/matching.md, "Rounds").
  constraint seats_round_id_account_id_key unique (round_id, account_id)
);

-- A member's own seats are read by account.
create index seats_account_id_idx on app.seats (account_id);

-- +goose Down

drop table app.seats;
drop table app.outings;
