-- +goose Up

-- Plan review (M8, owner's decision 2026-10-24): a city can be put in
-- review mode, for launching it by hand. Its suggested plans (walks
-- included) then wait as drafts, seen by nobody, until an admin approves
-- (opening them, with a fresh signup window) or rejects them. Admins can
-- also create plans from scratch, in any city.

-- Set by admins (ModerationService.SetCityReview), not by city packs:
-- seeding leaves it alone.
alter table app.cities add column review boolean not null default false;

-- draft:    waiting for an admin (only in review mode)
-- rejected: an admin turned the draft down; it isn't suggested again
--           until the cool-down passes, like a plan that found no group
alter table app.plans
  drop constraint plans_status_check,
  add constraint plans_status_check check (status in ('draft', 'open', 'closed')),
  drop constraint plans_closed_reason_check,
  add constraint plans_closed_reason_check check (closed_reason in ('done', 'no_group', 'withdrawn', 'rejected')),
  drop constraint plans_closed,
  add constraint plans_closed check ((status = 'closed') = (closed_reason is not null and closed_at is not null)),
  -- An admin who made the plan by hand; null = the matcher suggested it.
  add column created_by uuid,
  -- The admin who approved or rejected a draft, and when.
  add column reviewed_by uuid,
  add column reviewed_at timestamptz;
create index plans_draft_city_idx on app.plans(city_id, created_at) where status = 'draft';

-- +goose Down
-- Drafts were never seen by anyone; rejected drafts become withdrawn.
delete from app.plans where status = 'draft';
update app.plans set closed_reason = 'withdrawn' where closed_reason = 'rejected';
drop index app.plans_draft_city_idx;
alter table app.plans
  drop column reviewed_at,
  drop column reviewed_by,
  drop column created_by,
  drop constraint plans_closed,
  add constraint plans_closed check ((status = 'open') = (closed_reason is null and closed_at is null)),
  drop constraint plans_closed_reason_check,
  add constraint plans_closed_reason_check check (closed_reason in ('done', 'no_group', 'withdrawn')),
  drop constraint plans_status_check,
  add constraint plans_status_check check (status in ('open', 'closed'));
alter table app.cities drop column review;
