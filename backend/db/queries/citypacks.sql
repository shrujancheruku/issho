-- City-pack editor (admin SPA). Each city's areas with whether they have
-- travel times (from them) and venues, to show what still needs Google.
-- name: ListCityPackAreas :many
select a.city_id, a.id,
       exists (select 1 from app.area_travel_times t where t.from_area_id = a.id and t.to_area_id <> a.id) as has_travel_times,
       exists (select 1 from app.venues v where v.area_id = a.id) as has_venues
from app.areas a
order by a.city_id, a.id;

-- name: ListCatalogActivities :many
select id, coalesce(name->>'en', id)::text as name, category from app.activities order by category, id;

-- name: CountCityVenues :one
select count(*)::int from app.venues where city_id = $1;

-- Applying a pack makes the database match it (cities.Apply). The queries
-- below find what a pack no longer lists, count what depends on it and
-- carry people over to the area that replaces it.

-- name: ListAreaCities :many
-- Which city each of these areas belongs to (absent ids are not returned).
select id, city_id from app.areas where id = any(@ids::text[]);

-- name: CountAreaReferences :many
-- What depends on each area: people (home area), crews, plans, and venues
-- an admin curated (approved or rejected, or added by hand), which a delete
-- would lose; discovery would only bring back candidates.
select a.id,
       (select count(*) from app.profiles p where p.home_area_id = a.id)::int as people,
       (select count(*) from app.crews c where c.area_id = a.id)::int as crews,
       (select count(*) from app.plans p where p.area_id = a.id)::int as plans,
       (select count(distinct v.google_place_id) from app.venues v
         where v.area_id = a.id and (v.status <> 'candidate' or v.added_by is not null))::int as curated_venues
from app.areas a
where a.id = any(@ids::text[])
order by a.id;

-- name: MoveProfilesToArea :execrows
update app.profiles set home_area_id = @new_id where home_area_id = @old_id;

-- name: MoveCrewsToArea :execrows
update app.crews set area_id = @new_id where area_id = @old_id;

-- name: MovePlansToArea :execrows
update app.plans set area_id = @new_id where area_id = @old_id;

-- name: DeleteAreas :exec
-- Venues, travel times and walk failures of these areas cascade.
delete from app.areas where id = any(@ids::text[]);

-- name: DeleteCityOverridesExcept :many
-- The city's overrides that the pack no longer lists: the activity falls back
-- to the catalogue's default.
delete from app.city_activity_overrides
where city_id = @city_id and not (activity_id = any(@keep::text[]))
returning activity_id;
