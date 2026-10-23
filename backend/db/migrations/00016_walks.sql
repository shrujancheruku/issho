-- +goose Up

-- Walks as itineraries (owner's request, 2026-10-22): a walking plan
-- (temples, a photo walk, a food tour) is 2-4 hours from a meeting place
-- (a station or a café), through several stops that fit its theme, to a
-- place to sit and talk at the end. The worker composes one when it opens
-- the plan (internal/walks): Gemini proposes the outing from its own
-- knowledge, Places checks that every stop exists, Routes measures the walk.

-- What a walking activity's stops are about, in English, for the model
-- (data/activities.json "walk_theme"). null = not a walk: the plan is the
-- activity at a venue, as before.
alter table app.activities add column walk_theme text;

-- A composed walk's title, written in English by the model and translated
-- on demand like other text: {"en": "..."}. null for plans whose title is
-- composed from their activities' names. Its summary goes in the
-- description column reserved for this in 00007.
alter table app.plans add column title jsonb;

-- A composed walk's stops in order. Only Google place ids are stored (Maps
-- terms); names, addresses and locations are fetched live when shown.
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

-- +goose Down
drop table app.walk_failures;
drop table app.plan_stops;
alter table app.plans drop column title;
alter table app.activities drop column walk_theme;
