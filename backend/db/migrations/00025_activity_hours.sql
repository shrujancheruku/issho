-- +goose Up
-- How long an activity usually takes a group, so the matching engine can
-- weigh one activity against two in a plan (matching.Activity.Hours). The
-- catalogue (data/activities.json) sets it when seeded; 2 until then.
alter table app.activities
  add column hours numeric(3,1) not null default 2 check (hours > 0 and hours <= 24);

-- +goose Down
alter table app.activities drop column hours;
