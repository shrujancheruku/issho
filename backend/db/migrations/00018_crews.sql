-- +goose Up

-- Crews (M7): a standing group chat for people who got on well at outings,
-- named after what they have in common (an activity they all like, or the
-- area they all live in), where plans that fit are suggested every week or
-- so. The bar is high (matching.FindCrews): everyone has met everyone else
-- at an outing, nobody minded anyone (no "rather not", report, or declining
-- or leaving a crew with them), and each has a mutual meet-again pick in
-- it. It's offered to everyone; it forms with those who accept.

create table app.crews (
  id              uuid primary key default gen_random_uuid(),
  city_id         text not null references app.cities(id) on delete cascade,
  -- What they have in common, which names it: exactly one of the two.
  activity_id     text references app.activities(id),
  area_id         text references app.areas(id),
  -- offered: waiting for answers until offer_until
  -- active:  formed (at least 3 accepted)
  -- lapsed:  too few accepted in time
  -- closed:  fewer than 2 members left
  status          text not null default 'offered' check (status in ('offered', 'active', 'lapsed', 'closed')),
  offer_until     timestamptz not null,
  -- The last plan suggested in its chat, so suggestions stay occasional.
  suggested_at    timestamptz,
  created_at      timestamptz not null default now(),
  formed_at       timestamptz,
  closed_at       timestamptz,
  constraint crews_named check ((activity_id is null) <> (area_id is null))
);
create index crews_city_idx on app.crews(city_id) where status in ('offered', 'active');

create table app.crew_members (
  crew_id      uuid not null references app.crews(id) on delete cascade,
  user_id      uuid not null references app.users(id) on delete cascade,
  -- offered -> accepted or declined; accepted -> left
  response     text not null default 'offered' check (response in ('offered', 'accepted', 'declined', 'left')),
  responded_at timestamptz,
  primary key (crew_id, user_id),
  check ((response = 'offered') = (responded_at is null))
);
create index crew_members_user_idx on app.crew_members(user_id);

-- Why people leave a crew, to learn what makes crews work. Private: the
-- crew isn't told why (or that) someone left.
create table app.crew_leaves (
  id         bigint generated always as identity primary key,
  crew_id    uuid not null references app.crews(id) on delete cascade,
  -- Kept when the account is deleted: the reason is still worth knowing.
  user_id    uuid references app.users(id) on delete set null,
  reason     text not null check (reason in ('uncomfortable', 'not_interested', 'inactive', 'too_many_messages', 'other')),
  details    text not null default '' check (char_length(details) <= 1000),
  created_at timestamptz not null default now()
);
create index crew_leaves_crew_idx on app.crew_leaves(crew_id);

-- The crew's chat. A message is someone's text, or a plan the worker
-- suggested (plan_id, no author).
create table app.crew_messages (
  id         bigint generated always as identity primary key,
  crew_id    uuid not null references app.crews(id) on delete cascade,
  user_id    uuid references app.users(id) on delete cascade,
  body       text not null default '' check (char_length(body) <= 1000),
  plan_id    uuid references app.plans(id) on delete cascade,
  created_at timestamptz not null default now(),
  check ((user_id is null) = (plan_id is not null)),
  check (plan_id is not null or char_length(body) >= 1)
);
create index crew_messages_crew_idx on app.crew_messages(crew_id, id);

-- Reports can be about someone met in a crew rather than at an outing.
alter table app.reports add column crew_id uuid references app.crews(id) on delete set null;
drop index app.reports_once_idx;
create unique index reports_once_idx on app.reports(reporter_id, reported_id, event_id) where event_id is not null;
create unique index reports_crew_once_idx on app.reports(reporter_id, reported_id, crew_id) where crew_id is not null;

-- Notifications about a crew: offered, formed, a plan suggested.
alter table app.notifications
  add column crew_id uuid references app.crews(id) on delete cascade,
  drop constraint notifications_subject,
  add constraint notifications_subject check (num_nonnulls(event_id, plan_id, crew_id) = 1),
  drop constraint notifications_kind_check,
  add constraint notifications_kind_check check (kind in ('event_invited', 'group_confirmed', 'group_cancelled', 'plan_no_group',
    'event_feedback', 'crew_offered', 'crew_formed', 'crew_suggestion'));
create index notifications_crew_idx on app.notifications(crew_id);

-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on app.crews, app.crew_members, app.crew_leaves, app.crew_messages from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
delete from app.notifications where crew_id is not null;
drop index app.notifications_crew_idx;
alter table app.notifications
  drop constraint notifications_kind_check,
  add constraint notifications_kind_check check (kind in ('event_invited', 'group_confirmed', 'group_cancelled', 'plan_no_group', 'event_feedback')),
  drop constraint notifications_subject,
  add constraint notifications_subject check ((event_id is null) <> (plan_id is null)),
  drop column crew_id;
delete from app.reports where crew_id is not null;
drop index app.reports_crew_once_idx;
drop index app.reports_once_idx;
create unique index reports_once_idx on app.reports(reporter_id, reported_id, event_id);
alter table app.reports drop column crew_id;
drop table app.crew_messages;
drop table app.crew_leaves;
drop table app.crew_members;
drop table app.crews;
