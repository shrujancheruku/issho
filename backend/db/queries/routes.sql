-- name: SetPlanRoute :exec
update app.plans set route_polyline = @polyline, route_seconds = @seconds, route_computed_at = @at
where id = @id;

-- Plans whose route is still needed (open, or with an event still to
-- happen) and missing or older than the refresh age.
-- name: ListPlansNeedingRoute :many
select p.id, p.venue_place_id, p.follow_up_venue_place_id
from app.plans p
where p.city_id = @city_id and p.venue_place_id is not null and p.follow_up_venue_place_id is not null
  and (p.route_computed_at is null or p.route_computed_at < @older_than)
  and (p.status = 'open' or exists (select 1 from app.events e where e.plan_id = p.id and e.status = 'scheduled'))
order by p.route_computed_at nulls first, p.id
limit @max;

-- A city's routes past the Maps caching limit, whatever the plan's state.
-- name: ClearStaleRoutes :execrows
update app.plans set route_polyline = null, route_seconds = null, route_computed_at = null
where city_id = @city_id and route_computed_at < @cutoff;
