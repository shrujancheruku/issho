-- +goose Up

-- Venue curation (M8). Discovery keeps every place whose primary type fits
-- a category, which lets through some that don't suit a group outing
-- (Japanese snack bars are typed `bar`). Admins approve or reject places
-- (all of a place's category rows together) and can add places discovery
-- missed, found by name.

-- With this on, the matcher uses only approved venues in the city:
-- picking one when a plan opens, and counting them to decide what's
-- feasible. Set by admins (CurationService), not by city packs.
alter table app.cities add column approved_venues_only boolean not null default false;

alter table app.venues
  -- The admin who last approved or rejected the place, and when (null for
  -- a place Google reported closed for good, or never reviewed).
  add column reviewed_by uuid,
  add column reviewed_at timestamptz,
  -- The admin who added the place by hand; null = discovery found it.
  add column added_by uuid;
create index venues_city_status_idx on app.venues(city_id, status);

-- +goose Down
drop index app.venues_city_status_idx;
alter table app.venues
  drop column added_by,
  drop column reviewed_at,
  drop column reviewed_by;
alter table app.cities drop column approved_venues_only;
