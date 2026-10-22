-- name: SetPlanRoute :exec
update app.plans set route_polyline = @polyline, route_seconds = @seconds, route_computed_at = @at
where id = @id;

-- Plans whose route is still needed (open, or with an event still to
-- happen) and missing or older than the refresh age.
-- name: ListPlansNeedingRoute :many
-- The walk starts at the first venue, or the meeting place when the first
-- activity has no venue (a walk ending at a café).
select p.id, coalesce(p.venue_place_id, p.meeting_place_id)::text as from_place, p.follow_up_venue_place_id
from app.plans p
where p.city_id = @city_id and coalesce(p.venue_place_id, p.meeting_place_id) is not null and p.follow_up_venue_place_id is not null
  and (p.route_computed_at is null or p.route_computed_at < @older_than)
  and (p.status = 'open' or exists (select 1 from app.events e where e.plan_id = p.id and e.status = 'scheduled'))
order by p.route_computed_at nulls first, p.id
limit @max;

-- A city's routes past the Maps caching limit, whatever the plan's state.
-- name: ClearStaleRoutes :execrows
update app.plans set route_polyline = null, route_seconds = null, route_computed_at = null
where city_id = @city_id and route_computed_at < @cutoff;

-- Plans in a city whose first activity has no venue and no meeting place
-- yet, still open or with an event to come, with their area's centre.
-- name: ListPlansNeedingMeetingPlace :many
select p.id, st_y(a.centroid::geometry)::float8 as lat, st_x(a.centroid::geometry)::float8 as lng
from app.plans p
join app.areas a on a.id = p.area_id
where p.city_id = @city_id and p.venue_place_id is null and p.meeting_place_id is null
  and (p.status = 'open' or exists (select 1 from app.events e where e.plan_id = p.id and e.status = 'scheduled'))
order by p.created_at
limit @max;

-- name: SetMeetingPlace :exec
update app.plans set meeting_place_id = @place_id where id = @id;
