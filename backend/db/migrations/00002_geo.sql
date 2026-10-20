-- +goose Up

-- Travel modes Issho calculates with: walking and driving only. Public
-- transport was dropped: Google's APIs return no transit routes in Japan, and
-- one consistent model everywhere beats per-country routing vendors.
-- build-matrix computes only a city's modes, and matching reads only these.
-- The default exists only to backfill existing rows; city packs always set it.
alter table app.cities
  add column travel_modes text[] not null default '{walk,drive}'
  check (travel_modes <@ array['walk', 'drive']::text[] and cardinality(travel_modes) > 0);
alter table app.cities alter column travel_modes drop default;

-- 00001 allowed transit and bicycle times too; restrict to the same two modes.
delete from app.area_travel_times where mode not in ('walk', 'drive');
alter table app.area_travel_times
  drop constraint area_travel_times_mode_check,
  add constraint area_travel_times_mode_check check (mode in ('walk', 'drive'));

-- Kinds of place an activity happens at (cafe, park, karaoke box ...), mapped to
-- Google Places (New) types. Keeping our own ids means the provider can change
-- without touching activities.
create table app.venue_categories (
  id            text primary key,              -- e.g. 'cafe'
  name          jsonb not null,
  google_types  text[] not null check (cardinality(google_types) > 0),
  search_radius_m integer not null default 1500 check (search_radius_m between 100 and 50000)
);

-- Venue categories each activity needs; empty = no venue (e.g. a walk that only
-- needs a meeting point).
alter table app.activities
  add column venue_categories text[] not null default '{}';

-- Venues found by `tool discover-venues`. Google Maps terms allow storing place
-- ids indefinitely but restrict caching other place content (name, address,
-- location, hours, ratings), so only the id and our own data live here; details
-- are fetched live when needed.
create table app.venues (
  google_place_id text not null,
  city_id         text not null references app.cities(id) on delete cascade,
  area_id         text not null references app.areas(id) on delete cascade, -- area it was found from
  category_id     text not null references app.venue_categories(id) on delete cascade,
  tags            text[] not null default '{}',   -- our own curation tags, e.g. 'good-for-groups'
  status          text not null default 'candidate'
                  check (status in ('candidate', 'approved', 'rejected')), -- admin curation (M8)
  discovered_at   timestamptz not null default now(),
  last_seen_at    timestamptz not null default now(), -- last time discovery returned it
  -- One row per place and category: a karaoke bar is both 'bar' and 'karaoke'.
  primary key (google_place_id, category_id)
);
create index venues_area_category_idx on app.venues(area_id, category_id);
create index venues_city_idx on app.venues(city_id);

-- Supabase's default privileges grant new tables to anon/authenticated. Schema
-- usage is already revoked (00001); this is defence in depth.
-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on all tables in schema app from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
alter table app.area_travel_times
  drop constraint area_travel_times_mode_check,
  add constraint area_travel_times_mode_check check (mode in ('transit', 'walk', 'drive', 'bicycle'));
drop table app.venues;
alter table app.activities drop column venue_categories;
drop table app.venue_categories;
alter table app.cities drop column travel_modes;
