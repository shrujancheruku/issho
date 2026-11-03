-- name: GetCityTravelModes :one
select travel_modes from app.cities where id = $1;

-- name: GetCityLanguages :one
select languages from app.cities where id = $1;

-- name: ListCityAreas :many
select id, st_y(centroid::geometry)::float8 as lat, st_x(centroid::geometry)::float8 as lng,
       coalesce(st_asgeojson(boundary), '')::text as boundary
from app.areas
where city_id = $1
order by id;

-- One statement for a whole rebuild (the arrays are parallel, as in
-- InsertPushTickets): a city's matrix is thousands of rows, and a round trip
-- each made the rebuild minutes long (#185). Every row gets the same
-- computed_at (now() is the transaction's start), which
-- DeleteTravelTimesNotRefreshed relies on. A pair may appear only once per
-- statement (on conflict can't touch a row twice); SaveMatrix sees to it.
-- name: UpsertTravelTimes :exec
insert into app.area_travel_times (from_area_id, to_area_id, mode, minutes, computed_at)
select u.from_area_id, u.to_area_id, u.mode, u.minutes, now()
from (select unnest(@from_area_ids::text[]) as from_area_id, unnest(@to_area_ids::text[]) as to_area_id,
             unnest(@modes::text[]) as mode, unnest(@minutes::int[]) as minutes) u
on conflict (from_area_id, to_area_id, mode) do update set
  minutes = excluded.minutes, computed_at = excluded.computed_at;

-- name: ListCityVenueCategories :many
-- Only categories needed by an activity enabled in the city, so discovery
-- doesn't pay for venues nothing will use (e.g. temples in Austin).
select c.id, c.google_types, c.search_radius_m
from app.venue_categories c
where exists (
  select 1
  from app.activities a
  left join app.city_activity_overrides o on o.activity_id = a.id and o.city_id = @city_id
  where c.id = any(a.venue_categories) and coalesce(o.enabled, true)
)
order by c.id;

-- name: UpsertVenue :exec
-- Re-discovering a venue only bumps last_seen_at; curation status
-- is kept. A place found in a new category takes the status an admin (or
-- a closure) already gave the place: curation is per place (00023).
insert into app.venues (google_place_id, city_id, area_id, category_id, status)
values (@google_place_id, @city_id, @area_id, @category_id, coalesce(
  (select v.status from app.venues v where v.google_place_id = @google_place_id
   order by v.status = 'rejected' desc, v.status = 'approved' desc limit 1), 'candidate'))
on conflict (google_place_id, category_id) do update set last_seen_at = now();

-- A full rebuild replaces the city's matrix: rows it didn't write (a pair
-- that lost its route, a mode the city dropped) go at once. now() is the
-- transaction's start, the same value every upsert in it stored, so this
-- removes exactly the rows not refreshed by this transaction.
-- name: DeleteTravelTimesNotRefreshed :execrows
delete from app.area_travel_times t
using app.areas a
where a.id = t.from_area_id and a.city_id = @city_id and t.computed_at < now();

-- name: NewestTravelTime :one
select max(t.computed_at)::timestamptz
from app.area_travel_times t
join app.areas a on a.id = t.from_area_id
where a.city_id = $1;

-- A venue Google reports as permanently closed is never picked again.
-- name: RejectVenue :exec
update app.venues set status = 'rejected' where google_place_id = $1;

-- UpsertVenue for a batch of discovered venues (parallel arrays), one
-- statement instead of a round trip each (#185). The status a place already
-- has is read from the rows before this statement, as the one-by-one inserts
-- would have found it: a new row in the same batch only ever copies it. A
-- (place, category) pair may appear only once per statement; SaveVenues
-- sees to it.
-- name: UpsertVenues :exec
insert into app.venues (google_place_id, city_id, area_id, category_id, status)
select u.google_place_id, @city_id, u.area_id, u.category_id, coalesce(
  (select v.status from app.venues v where v.google_place_id = u.google_place_id
   order by v.status = 'rejected' desc, v.status = 'approved' desc limit 1), 'candidate')
from (select unnest(@google_place_ids::text[]) as google_place_id, unnest(@area_ids::text[]) as area_id,
             unnest(@category_ids::text[]) as category_id) u
on conflict (google_place_id, category_id) do update set last_seen_at = now();
