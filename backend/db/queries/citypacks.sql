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
