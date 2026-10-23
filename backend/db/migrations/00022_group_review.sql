-- +goose Up

-- Group review (M8, owner's decision 2026-10-24): in a city under review
-- (cities.review), a round the scheduler is ready to schedule is saved as a
-- proposal: the event and its groups, but nobody is invited or told, and
-- the plan stays in its round with everyone still signed up. An admin can
-- move people between groups, take someone out, change the start time and
-- set each group's meeting point, then approve (the invitations go out, as
-- if just scheduled) or reject it (it's deleted, and the plan isn't
-- scheduled again for a day).
alter table app.events
  drop constraint events_status_check,
  add constraint events_status_check check (status in ('proposed', 'scheduled', 'done', 'cancelled')),
  -- The admin who approved a proposal, and when.
  add column reviewed_by uuid,
  add column reviewed_at timestamptz;
-- The scheduler skips plans with a proposal waiting.
create index events_proposed_idx on app.events(plan_id) where status = 'proposed';

-- When an admin last rejected a proposal for the plan: the scheduler leaves
-- it alone for a day, or it would propose the same groups an hour later.
alter table app.plans add column proposal_rejected_at timestamptz;

-- +goose Down
alter table app.plans drop column proposal_rejected_at;
delete from app.events where status = 'proposed';
drop index app.events_proposed_idx;
alter table app.events
  drop column reviewed_at,
  drop column reviewed_by,
  drop constraint events_status_check,
  add constraint events_status_check check (status in ('scheduled', 'done', 'cancelled'));
