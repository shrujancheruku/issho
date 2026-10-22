-- +goose Up

-- Where a plan's people meet when its first activity has no venue (a photo
-- walk, a city walk): a place near the area's centre, a station first, then
-- a park, found by the worker with Places Nearby Search. Only the Google
-- place id is stored (Maps terms); its name and address are fetched live.
-- null for plans that start at a venue (they meet there) or before the
-- worker has found one.
alter table app.plans add column meeting_place_id text;

-- +goose Down
alter table app.plans drop column meeting_place_id;
