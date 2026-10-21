-- +goose Up

-- Plans, signups, events and groups (M6). An outing happens plan first, time
-- second (internal/matching):
--   plan    an activity in an area, in a language, with no date yet; open
--           for signups in rounds
--   signup  one person's "I'm in" for a plan's current round, with the times
--           that suit them
--   event   a plan round given a date (one per round), once its signups can
--           make a good group or the round's signup deadline passes
--   group   the people who go together on an event; an event can have
--           several groups, each confirming separately
--
-- Ids are uuids because they reach the app, where sequential ids would reveal
-- how many plans and events exist.

create table app.plans (
  id                 uuid primary key default gen_random_uuid(),
  city_id            text not null references app.cities(id) on delete cascade,
  -- Areas and activities are only ever deleted by city-pack seeding (merged
  -- activities). Default "no action" makes that fail while plans still use
  -- them, rather than silently deleting history; deleting the whole city
  -- still cascades through both in one statement.
  area_id            text not null references app.areas(id),
  activity_id        text not null references app.activities(id),
  follow_up_id       text references app.activities(id), -- e.g. ramen after bouldering
  -- The plan's languages are its events' languages: primary_language is the
  -- one most members speak; secondary_language only for a city's language
  -- pairs (matching.Config.LanguagePairs), e.g. a JP-ES exchange.
  primary_language   text not null check (primary_language ~ '^[a-z]{2,3}$'),
  secondary_language text check (secondary_language ~ '^[a-z]{2,3}$'),
  -- Written by Claude (template fallback) and prefilled in English plus the
  -- plan's languages: {"en": "...", "ja": "..."}. null until written.
  description        jsonb,
  -- From matching.Suggest, for diagnostics and the admin review (M8).
  audience           integer not null check (audience >= 0),
  score              real not null,
  -- The current round. Signups not placed when a round is scheduled roll
  -- over to the next one, with a new window and deadline.
  round              smallint not null default 1 check (round > 0),
  window_start       date not null,                -- in the city's timezone
  window_days        smallint not null check (window_days between 1 and 60),
  signup_deadline    timestamptz not null,         -- the round is scheduled with whoever can go (force)
  status             text not null default 'open' check (status in ('open', 'closed')),
  -- Why a plan closed, shown to the people who signed up:
  --   done       scheduled, and nobody was left waiting for another round
  --   no_group   the deadline passed and no group could form
  --   withdrawn  taken down (activity disabled, admin, ...)
  closed_reason      text check (closed_reason in ('done', 'no_group', 'withdrawn')),
  closed_at          timestamptz,
  created_at         timestamptz not null default now(),
  constraint plans_languages_distinct check (secondary_language <> primary_language),
  constraint plans_closed check ((status = 'open') = (closed_reason is null and closed_at is null))
);
-- Every worker pass and every feed reads a city's open plans.
create index plans_open_city_idx on app.plans(city_id) where status = 'open';
create index plans_area_idx on app.plans(area_id);
create index plans_activity_idx on app.plans(activity_id);
create index plans_follow_up_idx on app.plans(follow_up_id) where follow_up_id is not null;

-- People waiting for a plan's current round. A signup is removed once its
-- person is invited to a group (app.group_members records it from then on),
-- so the "7 interested" count is simply the rows here.
create table app.plan_signups (
  plan_id    uuid not null references app.plans(id) on delete cascade,
  user_id    uuid not null references app.users(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (plan_id, user_id)
);
create index plan_signups_user_idx on app.plan_signups(user_id);

-- The weekly slots that suit someone for one plan (matching.Signup.Pattern).
-- No rows = their profile availability, which the app pre-fills; an empty
-- custom pattern would mean "never", which isn't a signup.
create table app.plan_signup_slots (
  plan_id  uuid not null,
  user_id  uuid not null,
  weekday  smallint not null check (weekday between 0 and 6), -- 0 = Sunday
  block    text not null check (block in ('morning', 'afternoon', 'evening')),
  primary key (plan_id, user_id, weekday, block),
  foreign key (plan_id, user_id) references app.plan_signups(plan_id, user_id) on delete cascade
);

-- Times within the window someone can't make (matching.Signup.Unavailable):
-- a whole date ('day') or one part of it. Events they're already going to
-- are added by the worker, not stored here.
create table app.plan_signup_unavailable (
  plan_id  uuid not null,
  user_id  uuid not null,
  date     date not null,
  block    text not null check (block in ('day', 'morning', 'afternoon', 'evening')),
  primary key (plan_id, user_id, date, block),
  foreign key (plan_id, user_id) references app.plan_signups(plan_id, user_id) on delete cascade
);

-- A plan round given a time (matching.Schedule).
create table app.events (
  id          uuid primary key default gen_random_uuid(),
  plan_id     uuid not null references app.plans(id) on delete cascade,
  round       smallint not null check (round > 0),
  date        date not null,                       -- in the city's timezone
  block       text not null check (block in ('morning', 'afternoon', 'evening')),
  -- The exact meeting time, set with the venue; until then the block is shown.
  starts_at   timestamptz,
  -- Invitees answer by then; groups without a quorum go ahead if at least
  -- min_attendance confirmed, else they're cancelled.
  confirm_by  timestamptz not null,
  -- forced = scheduled at the signup deadline rather than because a group
  -- reached its target size.
  forced      boolean not null,
  status      text not null default 'scheduled' check (status in ('scheduled', 'done', 'cancelled')),
  created_at  timestamptz not null default now(),
  unique (plan_id, round)
);
-- The worker's deadline passes (confirmations, reveals, chat) scan upcoming events.
create index events_scheduled_date_idx on app.events(date) where status = 'scheduled';

create table app.event_groups (
  id              uuid primary key default gen_random_uuid(),
  event_id        uuid not null references app.events(id) on delete cascade,
  -- From matching.Group: confirm straight away at quorum; at confirm_by, go
  -- ahead with at least min_attendance. Two is the hard minimum for a group.
  quorum          smallint not null,
  min_attendance  smallint not null check (min_attendance >= 2),
  score           real not null,
  status          text not null default 'inviting' check (status in ('inviting', 'confirmed', 'cancelled')),
  -- Each group meets on its own; venues are picked per group so two groups
  -- of one event aren't sent to the same café table. Only the Google place
  -- id is stored (Maps terms); name, address and hours are fetched live.
  venue_place_id  text,
  -- Where exactly to meet, prefilled like plans.description; revealed to
  -- confirmed members 24 h before (the worker decides, not this table).
  meeting_point   jsonb,
  created_at      timestamptz not null default now(),
  check (quorum >= min_attendance),
  -- Lets app.group_members carry the event id, so one person can only be in
  -- one group per event.
  unique (id, event_id)
);
create index event_groups_event_idx on app.event_groups(event_id);

create table app.group_members (
  group_id      uuid not null,
  event_id      uuid not null,
  user_id       uuid not null references app.users(id) on delete cascade,
  -- invited -> confirmed or declined; confirmed -> dropped if they cancel
  -- later. Every change re-checks matching.Violation for the group.
  response      text not null default 'invited'
                check (response in ('invited', 'confirmed', 'declined', 'dropped')),
  responded_at  timestamptz,
  checked_in_at timestamptz,
  primary key (group_id, user_id),
  foreign key (group_id, event_id) references app.event_groups(id, event_id) on delete cascade,
  unique (event_id, user_id),
  check ((response = 'invited') = (responded_at is null)),
  check (checked_in_at is null or response = 'confirmed')
);
-- "My events", and the cascade when an account is deleted.
create index group_members_user_idx on app.group_members(user_id);

-- Supabase's default privileges grant new tables to anon/authenticated; schema
-- usage is already revoked (00001). Defence in depth, as in 00002.
-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on app.plans, app.plan_signups, app.plan_signup_slots, app.plan_signup_unavailable,
      app.events, app.event_groups, app.group_members from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
drop table app.group_members;
drop table app.event_groups;
drop table app.events;
drop table app.plan_signup_unavailable;
drop table app.plan_signup_slots;
drop table app.plan_signups;
drop table app.plans;
