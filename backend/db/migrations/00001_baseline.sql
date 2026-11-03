-- +goose Up

-- The baseline: migrations 00001-00028 of the pre-production history, written
-- as one clean schema (#160). Only issho-dev ever ran the old chain, and it was
-- reset onto this (docs/deploy.md). Left out on the way: profiles.reliability
-- (matching works it out from turnout), users.deleted_at (soft delete was ruled
-- out), users.verified_at, venues.tags and venues.discovered_at (never used).
-- Column order follows the old chain, so the generated Go structs only lose
-- fields.
--
-- Migration policy: backend/README.md, "Migrations".

create schema if not exists app;

-- PostGIS goes in `extensions`, where Supabase keeps its extensions (advisor
-- 0014 flags it in `public`). Hosted Supabase's `postgres` role already has
-- `extensions` on its search_path, so the unqualified st_* calls in
-- db/queries resolve; the types below are qualified so this file doesn't depend
-- on any search_path. The schema exists on Supabase; the `create` is for plain
-- Postgres.
create schema if not exists extensions;
create extension if not exists postgis with schema extensions;

-- Cities and their areas (the unit a user picks as "home").
create table app.cities (
  id            text primary key,              -- e.g. 'tokyo'
  name          jsonb not null,                -- {"en": "Tokyo", "ja": "東京"}
  timezone      text not null,
  currency      text not null,
  languages     text[] not null,               -- BCP 47; see the comment below
  -- A matching.Config overlay (weights and timing), merged over the defaults.
  matching      jsonb not null default '{}',
  created_at    timestamptz not null default now(),
  -- Modes build-matrix computes and matching reads. Walking and driving only:
  -- Google's APIs return no transit routes in Japan, and one consistent model
  -- everywhere beats per-country routing vendors. City packs always set it.
  travel_modes  text[] not null
                check (travel_modes <@ array['walk', 'drive']::text[] and cardinality(travel_modes) > 0),
  -- In review mode a city's suggested plans and groups wait as drafts and
  -- proposals until an admin approves them (launching a city by hand). Set by
  -- admins (ModerationService.SetCityReview), not by city packs.
  review        boolean not null default false,
  -- With this on, the matcher uses only approved venues in the city. Set by
  -- admins (CurationService), not by city packs.
  approved_venues_only boolean not null default false,
  -- Ordered {types, radius_m} searches for meeting places (a walk's meeting
  -- point). Empty = the code's defaults.
  meeting_places jsonb not null default '[]'
);

comment on column app.cities.languages is 'Languages commonly used in the city, most common first (suggestions only)';

create table app.areas (
  id        text primary key,                  -- e.g. 'tokyo-kichijoji'
  city_id   text not null references app.cities(id) on delete cascade,
  name      jsonb not null,
  centroid  extensions.geography(point, 4326) not null,
  -- The area's outline from its city pack (e.g. a ward's administrative
  -- boundary). Venue discovery searches all of it and places each venue in the
  -- area that contains it; areas without one use their point only.
  boundary  extensions.geography(multipolygon, 4326)
);
create index areas_city_idx on app.areas(city_id);

create table app.area_travel_times (
  from_area_id text not null references app.areas(id) on delete cascade,
  to_area_id   text not null references app.areas(id) on delete cascade,
  mode         text not null check (mode in ('walk', 'drive')),
  minutes      integer not null check (minutes >= 0),
  computed_at  timestamptz not null default now(),
  primary key (from_area_id, to_area_id, mode)
);
create index area_travel_times_to_area_id_idx on app.area_travel_times(to_area_id);

-- Global activity catalogue, localised and toggled per city.
create table app.activities (
  id              text primary key,            -- e.g. 'photo-walk'
  name            jsonb not null,
  category        text not null,
  indoor          boolean not null,
  energy          smallint not null check (energy between 1 and 3),
  cost_level      smallint not null check (cost_level between 0 and 3),
  conversational  boolean not null,
  skill_dependent boolean not null default false,
  min_group_size  smallint not null,
  max_group_size  smallint not null,
  -- Venue categories the activity needs; empty = no venue (a walk that only
  -- needs a meeting point).
  venue_categories text[] not null default '{}',
  -- Parts of the day it suits (matching step 2). Venue opening hours can't be
  -- stored (Google Maps terms), so this is our own coarse rule; live hours are
  -- checked when an event is created.
  blocks          text[] not null default '{morning,afternoon,evening}'
                  check (cardinality(blocks) > 0 and blocks <@ array['morning', 'afternoon', 'evening']::text[]),
  -- What a walking activity's stops are about, in English, for the model
  -- (data/activities.json "walk_theme"). null = not a walk.
  walk_theme      text,
  -- How long it usually takes a group (matching.Activity.Hours); the catalogue
  -- sets it when seeded.
  hours           numeric(3,1) not null default 2 check (hours > 0 and hours <= 24),
  check (min_group_size <= max_group_size)
);

create table app.city_activity_overrides (
  city_id     text not null references app.cities(id) on delete cascade,
  activity_id text not null references app.activities(id) on delete cascade,
  enabled     boolean not null default true,
  name        jsonb,                           -- local display name override
  months      smallint[],                      -- seasonality; null = all year
  primary key (city_id, activity_id)
);
create index city_activity_overrides_activity_id_idx on app.city_activity_overrides(activity_id);

-- Kinds of place an activity happens at (cafe, park, karaoke box ...), mapped
-- to Google Places (New) types. Keeping our own ids means the provider can
-- change without touching activities.
create table app.venue_categories (
  id            text primary key,              -- e.g. 'cafe'
  name          jsonb not null,
  google_types  text[] not null check (cardinality(google_types) > 0),
  search_radius_m integer not null default 1500 check (search_radius_m between 100 and 50000)
);

-- Venues found by `tool discover-venues`. Google Maps terms allow storing place
-- ids indefinitely but restrict caching other place content (name, address,
-- location, hours, ratings), so only the id and our own data live here.
-- Admins approve or reject places (all of a place's category rows together)
-- and add places discovery missed. The *_by columns hold the admin's user id
-- without a foreign key: an admin needn't have an app account.
create table app.venues (
  google_place_id text not null,
  city_id         text not null references app.cities(id) on delete cascade,
  area_id         text not null references app.areas(id) on delete cascade, -- area it was found from
  category_id     text not null references app.venue_categories(id) on delete cascade,
  status          text not null default 'candidate'
                  check (status in ('candidate', 'approved', 'rejected')),
  last_seen_at    timestamptz not null default now(), -- last time discovery returned it
  -- The admin who last approved or rejected the place, and when (null for a
  -- place Google reported closed for good, or never reviewed).
  reviewed_by     uuid,
  reviewed_at     timestamptz,
  -- The admin who added the place by hand; null = discovery found it.
  added_by        uuid,
  -- One row per place and category: a karaoke bar is both 'bar' and 'karaoke'.
  primary key (google_place_id, category_id)
);
create index venues_area_category_idx on app.venues(area_id, category_id);
create index venues_city_idx on app.venues(city_id);
create index venues_city_status_idx on app.venues(city_id, status);
create index venues_category_id_idx on app.venues(category_id);

-- App users. id equals the Supabase auth.users id (FK added at the end when
-- that schema exists).
create table app.users (
  id           uuid primary key,
  email        text,
  created_at   timestamptz not null default now()
);

create table app.profiles (
  user_id          uuid primary key references app.users(id) on delete cascade,
  display_name     text not null,
  birth_year       smallint not null check (birth_year between 1900 and 2100),
  gender           text not null check (gender in ('woman', 'man', 'nonbinary', 'unspecified')),
  same_gender_only boolean not null default false,
  home_area_id     text not null references app.areas(id),
  max_travel_min   smallint not null check (max_travel_min between 5 and 180),
  energy           smallint not null check (energy between 1 and 3),
  budget           smallint not null check (budget between 0 and 3),
  size_pref        text not null check (size_pref in ('small', 'medium', 'large')),
  updated_at       timestamptz not null default now(),
  -- Display languages: the app is shown in the primary one, with an optional
  -- secondary beside it (internal/platform/i18n). A script variant is allowed
  -- where one spoken language is written two ways (zh-Hant beside zh).
  -- Separate from app.user_languages, which records every language a person
  -- can *speak* (used for matching).
  primary_language   text not null check (primary_language ~ '^[a-z]{2,3}(-Hant)?$'),
  secondary_language text check (secondary_language ~ '^[a-z]{2,3}(-Hant)?$'),
  -- The code of conduct version accepted at the end of onboarding, so a
  -- revision can ask everyone again.
  conduct_version     smallint not null check (conduct_version > 0),
  conduct_accepted_at timestamptz not null default now(),
  constraint profiles_languages_distinct check (secondary_language <> primary_language)
);
create index profiles_home_area_id_idx on app.profiles(home_area_id);

create table app.user_languages (
  user_id  uuid not null references app.users(id) on delete cascade,
  lang     text not null,
  fluent   boolean not null,                   -- false = learning
  level    smallint check (level between 1 and 3), -- for learners: beginner..advanced
  primary key (user_id, lang),
  -- Spoken languages stay ISO 639-1: matching treats Chinese as one language.
  constraint user_languages_code check (lang ~ '^[a-z]{2,3}$'),
  -- A level only makes sense for a language someone is learning, and a
  -- learner always has one.
  constraint user_languages_level check ((fluent and level is null) or (not fluent and level is not null))
);

create table app.availability_slots (
  user_id  uuid not null references app.users(id) on delete cascade,
  weekday  smallint not null check (weekday between 0 and 6), -- 0 = Sunday
  block    text not null check (block in ('morning', 'afternoon', 'evening')),
  primary key (user_id, weekday, block)
);

create table app.user_activity_prefs (
  user_id     uuid not null references app.users(id) on delete cascade,
  activity_id text not null references app.activities(id) on delete cascade,
  weight      smallint not null check (weight between 0 and 2), -- 0 open to try, 1 like, 2 love
  primary key (user_id, activity_id)
);
create index user_activity_prefs_activity_id_idx on app.user_activity_prefs(activity_id);

-- Machine translations made on demand. Our own content is stored in English
-- only; any other language is translated the first time someone needs it,
-- stored here, and reused for everyone. Text that already exists in a language
-- is never translated.
--
-- Keyed by a hash of the English source plus its context, so editing the
-- English text naturally produces a fresh translation, and the same word in
-- two contexts (a "Bar" venue vs. a bar activity) can differ.
create table app.translations (
  source_hash text not null,                 -- hex sha256 of source + context
  lang        text not null check (lang ~ '^[a-z]{2,3}(-Hant)?$'),
  source      text not null,                 -- English source, for review
  context     text not null default '',      -- what the text is, given to the translator
  text        text not null,
  model       text not null,                 -- which translator produced it
  reviewed    boolean not null default false, -- a person checked it (admin)
  created_at  timestamptz not null default now(),
  -- A corrected machine translation: reviewed rows are never overwritten
  -- (InsertTranslation does nothing on conflict).
  reviewed_by uuid,
  reviewed_at timestamptz,
  primary key (source_hash, lang)
);

-- Plans, signups, events and groups. An outing happens plan first, time second
-- (internal/matching):
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
  -- plan's languages: {"en": "...", "ja": "..."}. null until written. For a
  -- composed walk, its summary.
  description        jsonb,
  -- From matching.Suggest, for diagnostics and the admin review.
  audience           integer not null check (audience >= 0),
  score              real not null,
  -- The current round. Signups not placed when a round is scheduled roll
  -- over to the next one, with a new window and deadline.
  round              smallint not null default 1 check (round > 0),
  window_start       date not null,                -- in the city's timezone
  window_days        smallint not null check (window_days between 1 and 60),
  signup_deadline    timestamptz not null,         -- the round is scheduled with whoever can go (force)
  -- draft:    waiting for an admin (only in a city under review)
  -- open:     taking signups
  -- closed:   see closed_reason
  status             text not null default 'open' check (status in ('draft', 'open', 'closed')),
  -- Why a plan closed, shown to the people who signed up:
  --   done       scheduled, and nobody was left waiting for another round
  --   no_group   the deadline passed and no group could form
  --   withdrawn  taken down (activity disabled, admin, ...)
  --   rejected   an admin turned the draft down; it isn't suggested again
  --              until the cool-down passes, like a plan that found no group
  closed_reason      text check (closed_reason in ('done', 'no_group', 'withdrawn', 'rejected')),
  closed_at          timestamptz,
  created_at         timestamptz not null default now(),
  -- Each plan names its places from the start: people sign up for a concrete
  -- outing ("climbing at B-PUMP, then dinner at X"), not a kind of place. The
  -- worker picks them when it opens the plan. Only Google place ids are
  -- stored (Maps terms); names, addresses and hours are fetched live. Groups
  -- of one event share these venues; app.event_groups.venue_place_id is for a
  -- group sent elsewhere (a full venue, admin override). null = the activity
  -- needs only a meeting point, or no venue was found when the plan opened.
  venue_place_id           text,
  follow_up_venue_place_id text,
  -- The walk between the two venues, from the Google Routes API: the line
  -- drawn on the plan's map and the walking time. Routes content may be cached
  -- for at most 30 days (Maps terms): the worker recomputes it at 25 days and
  -- clears it at 30. All three are null together.
  route_polyline     text,
  route_seconds      integer check (route_seconds >= 0),
  route_computed_at  timestamptz,
  -- Where people meet when the first activity has no venue (a photo walk): a
  -- place near the area's centre found by the worker. Google place id only.
  meeting_place_id   text,
  -- A composed walk's title, written in English by the model and translated on
  -- demand: {"en": "..."}. null for plans titled from their activities' names.
  title              jsonb,
  -- An admin who made the plan by hand; null = the matcher suggested it.
  created_by         uuid,
  -- The admin who approved or rejected a draft, and when.
  reviewed_by        uuid,
  reviewed_at        timestamptz,
  -- When an admin last rejected a proposed round for the plan: the scheduler
  -- leaves it alone for a day, or it would propose the same groups again.
  proposal_rejected_at timestamptz,
  constraint plans_languages_distinct check (secondary_language <> primary_language),
  constraint plans_closed check ((status = 'closed') = (closed_reason is not null and closed_at is not null)),
  constraint plans_follow_up_venue check (follow_up_venue_place_id is null or follow_up_id is not null),
  constraint plans_route check (
    (route_polyline is null) = (route_seconds is null) and (route_polyline is null) = (route_computed_at is null))
);
-- Every worker pass and every feed reads a city's open plans.
create index plans_open_city_idx on app.plans(city_id) where status = 'open';
create index plans_area_idx on app.plans(area_id);
create index plans_activity_idx on app.plans(activity_id);
create index plans_follow_up_idx on app.plans(follow_up_id) where follow_up_id is not null;
-- The route refresh job finds routes by age.
create index plans_route_age_idx on app.plans(route_computed_at) where route_computed_at is not null;
create index plans_draft_city_idx on app.plans(city_id, created_at) where status = 'draft';

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

-- A composed walk's stops in order (internal/walks). Only Google place ids are
-- stored (Maps terms); names, addresses and locations are fetched live.
create table app.plan_stops (
  plan_id      uuid not null references app.plans(id) on delete cascade,
  position     smallint not null check (position >= 0),
  -- meet: where people gather; stop: a place on the theme; finish: where
  -- the walk ends, sitting down to talk.
  role         text not null check (role in ('meet', 'stop', 'finish')),
  place_id     text not null,
  -- Planned time at the stop.
  minutes      smallint not null check (minutes between 0 and 120),
  -- Why it's worth the stop, by the model: {"en": "..."}.
  note         jsonb not null,
  -- The walk here from the previous stop (Routes content: cached at most
  -- 30 days, so cleared with the plan's route). null for the first stop or
  -- once cleared.
  walk_seconds integer check (walk_seconds >= 0),
  primary key (plan_id, position)
);

-- Areas where a walk couldn't be composed (the model's places didn't check
-- out, or the route was too long), so the worker doesn't pay for the same
-- failing attempt every hour. One row per area and activity, the latest.
create table app.walk_failures (
  area_id     text not null references app.areas(id) on delete cascade,
  activity_id text not null references app.activities(id) on delete cascade,
  failed_at   timestamptz not null,
  reason      text not null,
  primary key (area_id, activity_id)
);
create index walk_failures_activity_id_idx on app.walk_failures(activity_id);

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
  -- proposed: in a city under review, a round the scheduler is ready to
  -- schedule is saved with its groups but nobody is invited or told, and the
  -- plan stays in its round with everyone signed up. An admin approves it
  -- (the invitations go out) or rejects it (it's deleted).
  status      text not null default 'scheduled' check (status in ('proposed', 'scheduled', 'done', 'cancelled')),
  created_at  timestamptz not null default now(),
  -- The admin who approved a proposal, and when.
  reviewed_by uuid,
  reviewed_at timestamptz,
  unique (plan_id, round)
);
-- The worker's deadline passes (confirmations, reveals, chat) scan upcoming events.
create index events_scheduled_date_idx on app.events(date) where status = 'scheduled';
-- The scheduler skips plans with a proposal waiting.
create index events_proposed_idx on app.events(plan_id) where status = 'proposed';

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

-- Devices to send push notifications to (Expo push tokens). A person can have
-- several; a token belongs to the account last signed in on that device, so
-- registering it again moves it.
create table app.push_tokens (
  token        text primary key check (token like 'ExponentPushToken[%]' or token like 'ExpoPushToken[%]'),
  user_id      uuid not null references app.users(id) on delete cascade,
  platform     text not null check (platform in ('ios', 'android', 'web')),
  created_at   timestamptz not null default now(),
  last_seen_at timestamptz not null default now()
);
create index push_tokens_user_idx on app.push_tokens(user_id);

-- Expo's tickets for pushes it accepted, kept until their receipt is checked
-- (about 15 minutes later) so a phone whose app is gone stops getting pushes
-- even when Expo only finds out after accepting one. Deleted with the device
-- token, and after a day (Expo keeps receipts for 24 hours).
create table app.push_tickets (
  id         text primary key,
  token      text not null references app.push_tokens(token) on delete cascade,
  created_at timestamptz not null default now()
);
create index push_tickets_created_idx on app.push_tickets(created_at);
create index push_tickets_token_idx on app.push_tickets(token);

-- Group chat for the people going on an event: one chat per group, open from
-- 48 hours before the start until 24 hours after. There are no other messages
-- (no 1:1 chat: the product's platonic rules). Messages are deleted 30 days
-- after the event (the worker), and with the sender's account.
create table app.chat_messages (
  id         bigint generated always as identity primary key,
  group_id   uuid not null references app.event_groups(id) on delete cascade,
  user_id    uuid not null references app.users(id) on delete cascade,
  body       text not null check (char_length(body) between 1 and 1000),
  created_at timestamptz not null default now()
);
-- Reading a chat after a given message.
create index chat_messages_group_idx on app.chat_messages(group_id, id);
create index chat_messages_user_idx on app.chat_messages(user_id);

-- After an outing: each person who confirmed says whether they went, rates the
-- plan and the group, and privately says who they'd meet again and who they'd
-- rather not. Only mutual "again" picks count: they become matching links
-- (internal/matching Relations.Mutual). "Rather not" keeps the two apart for
-- good (Relations.Blocked) and out of each other's crews. Nobody is ever told
-- who picked them, either way.
-- One answer per person per group; sending it again replaces it.
create table app.event_feedback (
  group_id     uuid not null,
  user_id      uuid not null,
  -- Said they went. Counts as turning up for reliability, like a check-in:
  -- people forget to check in, and a no-show has no reason to claim this
  -- (it only makes them likelier to be invited).
  attended     boolean not null,
  -- 1-5; only from people who went.
  plan_rating  smallint check (plan_rating between 1 and 5),
  group_rating smallint check (group_rating between 1 and 5),
  created_at   timestamptz not null default now(),
  updated_at   timestamptz not null default now(),
  primary key (group_id, user_id),
  foreign key (group_id, user_id) references app.group_members(group_id, user_id) on delete cascade,
  constraint event_feedback_ratings check (attended = (plan_rating is not null and group_rating is not null))
);
create index event_feedback_user_idx on app.event_feedback(user_id);

-- "I'd meet them again" or "I'd rather not be in a group with them again",
-- private to the picker. Part of their feedback for that group, and only of
-- someone else in it. No row = no view either way.
create table app.mate_picks (
  group_id   uuid not null,
  picker_id  uuid not null,
  picked_id  uuid not null,
  pick       text not null check (pick in ('again', 'avoid')),
  created_at timestamptz not null default now(),
  primary key (group_id, picker_id, picked_id),
  foreign key (group_id, picker_id) references app.event_feedback(group_id, user_id) on delete cascade,
  foreign key (group_id, picked_id) references app.group_members(group_id, user_id) on delete cascade,
  check (picker_id <> picked_id)
);
-- Matching reads every pick a person made (their return rate) and whether it
-- was returned.
create index mate_picks_picker_idx on app.mate_picks(picker_id, picked_id);
create index mate_picks_picked_idx on app.mate_picks(picked_id);

-- Crews: a standing group chat for people who got on well at outings, named
-- after what they have in common (an activity they all like, or the area they
-- all live in), where plans that fit are suggested every week or so. The bar
-- is high (matching.FindCrews): everyone has met everyone else at an outing,
-- nobody minded anyone (no "rather not", report, or declining or leaving a
-- crew with them), and each has a mutual meet-again pick in it. It's offered
-- to everyone; it forms with those who accept.
create table app.crews (
  id              uuid primary key default gen_random_uuid(),
  city_id         text not null references app.cities(id) on delete cascade,
  -- What they have in common, which names it: exactly one of the two.
  activity_id     text references app.activities(id),
  area_id         text references app.areas(id),
  -- offered: waiting for answers until offer_until
  -- active:  formed (at least 3 accepted)
  -- lapsed:  too few accepted in time
  -- closed:  fewer than 2 members left
  status          text not null default 'offered' check (status in ('offered', 'active', 'lapsed', 'closed')),
  offer_until     timestamptz not null,
  -- The last plan suggested in its chat, so suggestions stay occasional.
  suggested_at    timestamptz,
  created_at      timestamptz not null default now(),
  formed_at       timestamptz,
  closed_at       timestamptz,
  constraint crews_named check ((activity_id is null) <> (area_id is null))
);
create index crews_city_idx on app.crews(city_id) where status in ('offered', 'active');
create index crews_activity_id_idx on app.crews(activity_id);
create index crews_area_id_idx on app.crews(area_id);

create table app.crew_members (
  crew_id      uuid not null references app.crews(id) on delete cascade,
  user_id      uuid not null references app.users(id) on delete cascade,
  -- offered -> accepted or declined; accepted -> left; lapsed = never
  -- answered by the offer's deadline. responded_at is set when the offer was
  -- decided, which the check below requires.
  response     text not null default 'offered' check (response in ('offered', 'accepted', 'declined', 'left', 'lapsed')),
  responded_at timestamptz,
  primary key (crew_id, user_id),
  check ((response = 'offered') = (responded_at is null))
);
create index crew_members_user_idx on app.crew_members(user_id);

-- Why people leave a crew, to learn what makes crews work. Private: the crew
-- isn't told why (or that) someone left.
create table app.crew_leaves (
  id         bigint generated always as identity primary key,
  crew_id    uuid not null references app.crews(id) on delete cascade,
  -- Kept when the account is deleted: the reason is still worth knowing.
  user_id    uuid references app.users(id) on delete set null,
  reason     text not null check (reason in ('uncomfortable', 'not_interested', 'inactive', 'too_many_messages', 'other')),
  details    text not null default '' check (char_length(details) <= 1000),
  created_at timestamptz not null default now()
);
create index crew_leaves_crew_idx on app.crew_leaves(crew_id);
-- The newest leaves first.
create index crew_leaves_created_idx on app.crew_leaves(created_at desc);
create index crew_leaves_user_id_idx on app.crew_leaves(user_id);

-- The crew's chat. A message is someone's text, or a plan the worker suggested
-- (plan_id, no author).
create table app.crew_messages (
  id         bigint generated always as identity primary key,
  crew_id    uuid not null references app.crews(id) on delete cascade,
  user_id    uuid references app.users(id) on delete cascade,
  body       text not null default '' check (char_length(body) <= 1000),
  plan_id    uuid references app.plans(id) on delete cascade,
  created_at timestamptz not null default now(),
  check ((user_id is null) = (plan_id is not null)),
  check (plan_id is not null or char_length(body) >= 1)
);
create index crew_messages_crew_idx on app.crew_messages(crew_id, id);
create index crew_messages_plan_id_idx on app.crew_messages(plan_id);
create index crew_messages_user_id_idx on app.crew_messages(user_id);

-- Notifications to send, written in the same transaction as the change that
-- causes them (an outbox): a round scheduled, a group going ahead or
-- cancelled. A worker job sends them and marks them sent, so a change that
-- rolls back never notifies anyone, and a failed send is retried. Each is
-- about exactly one of an event, a plan (a plan that closed without a group)
-- or a crew.
create table app.notifications (
  id         bigint generated always as identity primary key,
  user_id    uuid not null references app.users(id) on delete cascade,
  kind       text not null check (kind in ('event_invited', 'group_confirmed', 'group_cancelled', 'plan_no_group',
    'event_feedback', 'crew_offered', 'crew_formed', 'crew_suggestion')),
  event_id   uuid references app.events(id) on delete cascade,
  created_at timestamptz not null default now(),
  sent_at    timestamptz,
  attempts   smallint not null default 0,
  plan_id    uuid references app.plans(id) on delete cascade,
  crew_id    uuid references app.crews(id) on delete cascade,
  constraint notifications_subject check (num_nonnulls(event_id, plan_id, crew_id) = 1)
);
-- The sender reads only what's still to send; rows that ran out of attempts
-- (the sender gives up at 5) leave the index.
create index notifications_pending_idx on app.notifications(created_at) where sent_at is null and attempts < 5;
create index notifications_user_idx on app.notifications(user_id);
create index notifications_event_idx on app.notifications(event_id);
create index notifications_plan_idx on app.notifications(plan_id);
create index notifications_crew_idx on app.notifications(crew_id);

-- Moderation. Admins are Supabase users with app_metadata.admin
-- (internal/platform/auth); the *_by columns hold their user id without a
-- foreign key, because an admin needn't have an app account, and the record
-- should outlive theirs.
--
-- Reports about someone met through an outing or a crew. A report also keeps
-- the two out of each other's groups from then on.
create table app.reports (
  id          uuid primary key default gen_random_uuid(),
  -- Kept when the reporter deletes their account: what happened still
  -- matters for moderation, and the pair can no longer meet anyway.
  reporter_id uuid references app.users(id) on delete set null,
  reported_id uuid not null references app.users(id) on delete cascade,
  event_id    uuid references app.events(id) on delete set null,
  reason      text not null check (reason in ('dating', 'harassment', 'unsafe', 'no_show', 'other')),
  details     text not null default '' check (char_length(details) <= 1000),
  status      text not null default 'open' check (status in ('open', 'actioned', 'dismissed')),
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),
  reviewed_at timestamptz,
  crew_id     uuid references app.crews(id) on delete set null,
  reviewed_by uuid,
  -- The moderator's note on the decision: what they did and why.
  note        text not null default '' check (char_length(note) <= 2000),
  check (reporter_id <> reported_id)
);
-- One report per person per outing (or crew); reporting again updates it.
create unique index reports_once_idx on app.reports(reporter_id, reported_id, event_id) where event_id is not null;
create unique index reports_crew_once_idx on app.reports(reporter_id, reported_id, crew_id) where crew_id is not null;
create index reports_reported_idx on app.reports(reported_id);
create index reports_open_idx on app.reports(created_at) where status = 'open';
create index reports_crew_id_idx on app.reports(crew_id);
create index reports_event_id_idx on app.reports(event_id);

-- Signals for moderators that need a person to look, not an automatic action.
-- At most one open flag of each kind per person; it's updated while open and
-- stays until a moderator resolves it.
create table app.moderation_flags (
  id          bigint generated always as identity primary key,
  user_id     uuid not null references app.users(id) on delete cascade,
  -- pick_gender_lean: their meet-again picks lean heavily towards one gender
  -- compared with who they met (internal/matching.PickLean).
  kind        text not null check (kind in ('pick_gender_lean')),
  details     jsonb not null,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),
  resolved_at timestamptz,
  resolved_by uuid,
  note        text not null default '' check (char_length(note) <= 2000)
);
create unique index moderation_flags_open_idx on app.moderation_flags(user_id, kind) where resolved_at is null;
-- The queue lists open flags oldest first.
create index moderation_flags_queue_idx on app.moderation_flags(created_at) where resolved_at is null;

-- Suspending an account (ModerationService.SuspendPerson): while a suspension
-- is open the API refuses their calls (except to see that they're suspended,
-- sign out and delete their account), the chat socket refuses them, and
-- matching leaves them out. Kept as history: each suspension and its lifting
-- stay on record for the next moderator.
create table app.suspensions (
  id          bigint generated always as identity primary key,
  user_id     uuid not null references app.users(id) on delete cascade,
  -- Why, for moderators only; the person isn't shown it.
  reason      text not null check (char_length(reason) between 1 and 2000),
  created_by  uuid not null,
  created_at  timestamptz not null default now(),
  lifted_at   timestamptz,
  lifted_by   uuid,
  lift_note   text not null default '' check (char_length(lift_note) <= 2000),
  check ((lifted_at is null) = (lifted_by is null))
);
-- At most one open suspension per person; the API's check reads them all.
create unique index suspensions_open_idx on app.suspensions(user_id) where lifted_at is null;
create index suspensions_user_idx on app.suspensions(user_id, created_at desc);

-- On Supabase, tie app users to auth users and keep app tables off the public
-- API. Supabase's default privileges grant new tables to anon/authenticated,
-- so revoke them (the Data API is disabled too; this is defence in depth).
-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_namespace where nspname = 'auth') then
    alter table app.users
      add constraint users_auth_fk foreign key (id) references auth.users(id) on delete cascade;
  end if;
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on schema app from anon, authenticated;
    revoke all on all tables in schema app from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
-- Local work and CI's up -> down -> up only. Production migrates forward: a
-- mistake is fixed with a new migration. Refuse when there are accounts, since
-- dropping app deletes everything. (River's own Down, which runs first, makes
-- the same check before dropping its job tables.)
-- +goose StatementBegin
do $$
begin
  if exists (select 1 from app.users) then
    raise exception 'refusing to drop schema app: app.users has rows';
  end if;
end $$;
-- +goose StatementEnd
drop schema app cascade;
