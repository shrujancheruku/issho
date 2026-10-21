-- +goose Up

-- Parts of the day an activity suits (matching step 2): bars in the evening,
-- hikes and museums in daylight. Venue opening hours can't be stored (Google
-- Maps terms), so this is our own coarse rule; live hours are checked when an
-- event is created. Most activities suit any time, hence the default.
alter table app.activities
  add column blocks text[] not null default '{morning,afternoon,evening}'
  check (cardinality(blocks) > 0 and blocks <@ array['morning', 'afternoon', 'evening']::text[]);

-- +goose Down
alter table app.activities drop column blocks;
