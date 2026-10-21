-- +goose Up

-- Each plan names its places from the start (owner's decision, 2026-10-22):
-- people sign up for a concrete outing ("climbing at B-PUMP, then dinner at
-- X"), not a kind of place. The worker picks them when it opens the plan.
-- Only Google place ids are stored (Maps terms); names, addresses and hours
-- are fetched live. Groups of one event share these venues;
-- app.event_groups.venue_place_id stays for a group sent elsewhere (a full
-- venue, admin override).
-- null = the activity needs only a meeting point (a walk), or no venue was
-- found when the plan opened.
alter table app.plans
  add column venue_place_id text,
  add column follow_up_venue_place_id text,
  add constraint plans_follow_up_venue check (follow_up_venue_place_id is null or follow_up_id is not null);

-- +goose Down
alter table app.plans
  drop constraint plans_follow_up_venue,
  drop column follow_up_venue_place_id,
  drop column venue_place_id;
