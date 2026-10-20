-- +goose Up
create extension if not exists postgis;
create schema if not exists app;

-- Cities and their areas (the unit a user picks as "home").
create table app.cities (
  id            text primary key,              -- e.g. 'tokyo'
  name          jsonb not null,                -- {"en": "Tokyo", "ja": "東京"}
  timezone      text not null,
  currency      text not null,
  languages     text[] not null,               -- default languages, BCP 47
  matching_weights jsonb not null default '{}', -- per-city overrides of engine weights
  created_at    timestamptz not null default now()
);

create table app.areas (
  id        text primary key,                  -- e.g. 'tokyo-kichijoji'
  city_id   text not null references app.cities(id) on delete cascade,
  name      jsonb not null,
  centroid  geography(point, 4326) not null
);
create index areas_city_idx on app.areas(city_id);

create table app.area_travel_times (
  from_area_id text not null references app.areas(id) on delete cascade,
  to_area_id   text not null references app.areas(id) on delete cascade,
  mode         text not null check (mode in ('transit', 'walk', 'drive', 'bicycle')),
  minutes      integer not null check (minutes >= 0),
  computed_at  timestamptz not null default now(),
  primary key (from_area_id, to_area_id, mode)
);

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

-- App users. id equals the Supabase auth.users id (FK added below when present).
create table app.users (
  id           uuid primary key,
  email        text,
  verified_at  timestamptz,
  deleted_at   timestamptz,
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
  reliability      real not null default 1.0,
  updated_at       timestamptz not null default now()
);

create table app.user_languages (
  user_id  uuid not null references app.users(id) on delete cascade,
  lang     text not null,
  fluent   boolean not null,                   -- false = learning
  level    smallint check (level between 1 and 3), -- for learners: beginner..advanced
  primary key (user_id, lang)
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

-- On Supabase, tie app users to auth users and keep app tables off the public API.
-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_namespace where nspname = 'auth') then
    alter table app.users
      add constraint users_auth_fk foreign key (id) references auth.users(id) on delete cascade;
  end if;
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on schema app from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
drop schema app cascade;
