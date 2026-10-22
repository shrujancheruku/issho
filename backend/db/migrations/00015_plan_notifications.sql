-- +goose Up

-- Notifications about a plan rather than an event: "not enough people this
-- time" for whoever was still waiting when a plan closed without a group.
-- Each notification is about exactly one of the two.
alter table app.notifications
  alter column event_id drop not null,
  add column plan_id uuid references app.plans(id) on delete cascade,
  add constraint notifications_subject check ((event_id is null) <> (plan_id is null)),
  drop constraint notifications_kind_check,
  add constraint notifications_kind_check check (kind in ('event_invited', 'group_confirmed', 'group_cancelled', 'plan_no_group'));
create index notifications_plan_idx on app.notifications(plan_id);

-- +goose Down
delete from app.notifications where plan_id is not null;
drop index app.notifications_plan_idx;
alter table app.notifications
  drop constraint notifications_kind_check,
  add constraint notifications_kind_check check (kind in ('event_invited', 'group_confirmed', 'group_cancelled')),
  drop constraint notifications_subject,
  drop column plan_id,
  alter column event_id set not null;
