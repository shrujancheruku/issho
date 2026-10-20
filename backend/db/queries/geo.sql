-- name: GetCityTravelModes :one
select travel_modes from app.cities where id = $1;

-- name: GetCityTimezone :one
select timezone from app.cities where id = $1;

-- name: GetCityLanguages :one
select languages from app.cities where id = $1;

-- name: ListCityAreas :many
select id, st_y(centroid::geometry)::float8 as lat, st_x(centroid::geometry)::float8 as lng
from app.areas
where city_id = $1
order by id;

-- name: UpsertTravelTime :exec
insert into app.area_travel_times (from_area_id, to_area_id, mode, minutes, computed_at)
values ($1, $2, $3, $4, now())
on conflict (from_area_id, to_area_id, mode) do update set
  minutes = excluded.minutes, computed_at = excluded.computed_at;

-- Google Maps terms only allow temporary caching of Routes content, so travel
-- times older than the cutoff are deleted rather than kept around. A pair that
-- no longer has a route (or a mode the city dropped) also ages out this way.
-- name: DeleteStaleTravelTimes :execrows
delete from app.area_travel_times t
using app.areas a
where a.id = t.from_area_id and a.city_id = @city_id and t.computed_at < @cutoff;

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
-- Re-discovering a venue only bumps last_seen_at; curation status and tags are kept.
insert into app.venues (google_place_id, city_id, area_id, category_id)
values ($1, $2, $3, $4)
on conflict (google_place_id, category_id) do update set last_seen_at = now();
