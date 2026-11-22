-- +goose Up

-- A city and the plans in it. `cities` owns the first two tables and `plans`
-- the rest (docs/spec/architecture.md, "The seams"); each is read only through
-- its package's Go API. In development and tests a fixture seeds the rows
-- (backend/fixtures); nothing in a migration or in production inserts one.

-- A city: its slug, its time zone and currency (two of the ten city settings,
-- docs/spec/city-settings.md) and its state. The other settings, the areas and
-- the calendar arrive with their own tickets as tables of their own.
create table app.cities (
  slug      text primary key check (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
  -- docs/spec/cities.md, "A city's states". Only open cities admit people.
  state     text not null check (state in ('planned', 'open', 'closed')),
  -- An IANA name; cityclock.New is what rejects one it can't load.
  time_zone text not null check (time_zone <> ''),
  -- ISO 4217. Prices are in the city's currency and never converted.
  currency  text not null check (currency ~ '^[A-Z]{3}$')
);

-- A city's name in each language it is shown in: cities have real exonyms and
-- their names are translated, never machine-translated on the fly
-- (docs/spec/cities.md, "Areas"). A row per language, so a city that opens with
-- a new language is no migration.
create table app.city_names (
  city     text not null references app.cities (slug),
  language text not null check (language <> ''),
  name     text not null check (name <> ''),
  primary key (city, language)
);

-- An account's city is a listed city. #608 left it a plain slug.
alter table app.accounts
  add constraint accounts_city_fkey foreign key (city) references app.cities (slug);

-- A plan, with the fields that must be true on the day. Its prose is in
-- plan_texts, one row per language, so a translation can't change a field
-- (docs/spec/languages.md, "Plans").
create table app.plans (
  id              uuid primary key,
  city            text not null references app.cities (slug),
  -- Per person, in the city's currency, in whole units (a currency with minor
  -- units would need a column of its own).
  price           integer not null check (price >= 0),
  -- The meeting point's street address, which is a place's real local name and
  -- so is never translated. What to look for there is plan_texts.meeting_point.
  meeting_address text not null check (meeting_address <> ''),
  -- The parts of the day the plan can run in, in the city's clock
  -- (docs/spec/plans.md, "What a maker sets").
  parts_of_day    text[] not null check (
    cardinality(parts_of_day) > 0
    and parts_of_day <@ array['morning', 'afternoon', 'evening']
  ),
  -- Wall-clock "HH:MM" in the city's time zone; the date comes from the round.
  -- A plan doesn't cross midnight (cityclock.Span).
  starts_at       text not null check (starts_at ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
  ends_at         text not null check (ends_at ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$' and ends_at > starts_at)
);

create index plans_city_idx on app.plans (city);

-- A plan's prose in one language.
create table app.plan_texts (
  plan_id       uuid not null references app.plans (id) on delete cascade,
  language      text not null check (language <> ''),
  title         text not null check (title <> ''),
  description   text not null,
  -- What to look for at the meeting address ("the bridge's east end").
  meeting_point text not null check (meeting_point <> ''),
  what_to_bring text[] not null,
  primary key (plan_id, language)
);

-- A plan's stops in order. A leg is the way from this stop to the next, so the
-- last stop has none; each leg's mode and minutes are fields, not prose
-- (docs/spec/plans.md).
create table app.plan_stops (
  plan_id      uuid not null references app.plans (id) on delete cascade,
  position     integer not null check (position >= 1),
  -- Minutes spent at the stop.
  stay_minutes integer not null check (stay_minutes > 0),
  leg_mode     text check (leg_mode in ('walk', 'ride', 'transit')),
  leg_minutes  integer check (leg_minutes > 0),
  primary key (plan_id, position),
  -- Both or neither.
  constraint plan_stops_leg_check check ((leg_mode is null) = (leg_minutes is null))
);

-- A stop's name (a place's real name, in the language the plan is shown in)
-- and its note.
create table app.plan_stop_texts (
  plan_id  uuid not null,
  position integer not null,
  language text not null check (language <> ''),
  name     text not null check (name <> ''),
  note     text not null,
  primary key (plan_id, position, language),
  foreign key (plan_id, position) references app.plan_stops (plan_id, position) on delete cascade
);

-- +goose Down

drop table app.plan_stop_texts;
drop table app.plan_stops;
drop table app.plan_texts;
drop table app.plans;
alter table app.accounts drop constraint accounts_city_fkey;
drop table app.city_names;
drop table app.cities;
