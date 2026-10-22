-- +goose Up

-- The walk between a plan's two venues (activity → follow-up), from the
-- Google Routes API: the line drawn on the plan's map and the walking
-- time between stops. Routes content may be cached for at most 30 days
-- (Maps terms): the worker recomputes it at 25 days and clears it at 30.
-- All three are null together (no follow-up venue, or not computed yet).
alter table app.plans
  add column route_polyline text,
  add column route_seconds integer check (route_seconds >= 0),
  add column route_computed_at timestamptz,
  add constraint plans_route check (
    (route_polyline is null) = (route_seconds is null) and (route_polyline is null) = (route_computed_at is null));
-- The refresh job finds routes by age.
create index plans_route_age_idx on app.plans(route_computed_at) where route_computed_at is not null;

-- +goose Down
drop index app.plans_route_age_idx;
alter table app.plans
  drop constraint plans_route,
  drop column route_computed_at,
  drop column route_seconds,
  drop column route_polyline;
