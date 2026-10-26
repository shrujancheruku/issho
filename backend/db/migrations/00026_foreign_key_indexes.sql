-- +goose Up
-- Postgres doesn't index foreign keys itself. Without these, joining on
-- them (profiles -> areas when matching loads a city) and every cascade or
-- set-null from the parent row (deleting an account, an event, a plan, a
-- merged activity) scans the whole child table. The tables are still
-- small, so plain (locking) creates inside goose's transaction are fine.
create index profiles_home_area_id_idx on app.profiles (home_area_id);
create index area_travel_times_to_area_id_idx on app.area_travel_times (to_area_id);
create index city_activity_overrides_activity_id_idx on app.city_activity_overrides (activity_id);
create index user_activity_prefs_activity_id_idx on app.user_activity_prefs (activity_id);
create index venues_category_id_idx on app.venues (category_id);
create index walk_failures_activity_id_idx on app.walk_failures (activity_id);
create index reports_crew_id_idx on app.reports (crew_id);
create index reports_event_id_idx on app.reports (event_id);
create index crews_activity_id_idx on app.crews (activity_id);
create index crews_area_id_idx on app.crews (area_id);
create index crew_leaves_user_id_idx on app.crew_leaves (user_id);
create index crew_messages_plan_id_idx on app.crew_messages (plan_id);
create index crew_messages_user_id_idx on app.crew_messages (user_id);

-- +goose Down
drop index app.profiles_home_area_id_idx;
drop index app.area_travel_times_to_area_id_idx;
drop index app.city_activity_overrides_activity_id_idx;
drop index app.user_activity_prefs_activity_id_idx;
drop index app.venues_category_id_idx;
drop index app.walk_failures_activity_id_idx;
drop index app.reports_crew_id_idx;
drop index app.reports_event_id_idx;
drop index app.crews_activity_id_idx;
drop index app.crews_area_id_idx;
drop index app.crew_leaves_user_id_idx;
drop index app.crew_messages_plan_id_idx;
drop index app.crew_messages_user_id_idx;
