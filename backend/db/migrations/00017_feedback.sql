-- +goose Up

-- After an outing (M7): each person who confirmed says whether they went,
-- rates the plan and the group, and privately says who they'd meet again
-- and who they'd rather not. Only mutual "again" picks count: they become
-- matching links (internal/matching Relations.Mutual), so those two are
-- more likely to be grouped again. "Rather not" keeps the two apart for
-- good (Relations.Blocked) and out of each other's crews. Nobody is ever
-- told who picked them, either way.
--
-- (Numbered 00017: 00016 is the walks migration on its own branch.)

-- One answer per person per group; sending it again replaces it.
create table app.event_feedback (
  group_id     uuid not null,
  user_id      uuid not null,
  -- Said they went. Counts as turning up for reliability, like a check-in:
  -- people forget to check in, and a no-show has no reason to claim this
  -- (it only makes them likelier to be invited).
  attended     boolean not null,
  -- 1-5; only from people who went.
  plan_rating  smallint check (plan_rating between 1 and 5),
  group_rating smallint check (group_rating between 1 and 5),
  created_at   timestamptz not null default now(),
  updated_at   timestamptz not null default now(),
  primary key (group_id, user_id),
  foreign key (group_id, user_id) references app.group_members(group_id, user_id) on delete cascade,
  constraint event_feedback_ratings check (attended = (plan_rating is not null and group_rating is not null))
);
create index event_feedback_user_idx on app.event_feedback(user_id);

-- "I'd meet them again" or "I'd rather not be in a group with them again",
-- private to the picker. Part of their feedback for that group, and only
-- of someone else in it. No row = no view either way.
create table app.mate_picks (
  group_id   uuid not null,
  picker_id  uuid not null,
  picked_id  uuid not null,
  pick       text not null check (pick in ('again', 'avoid')),
  created_at timestamptz not null default now(),
  primary key (group_id, picker_id, picked_id),
  foreign key (group_id, picker_id) references app.event_feedback(group_id, user_id) on delete cascade,
  foreign key (group_id, picked_id) references app.group_members(group_id, user_id) on delete cascade,
  check (picker_id <> picked_id)
);
-- Matching reads every pick a person made (their return rate) and whether
-- it was returned.
create index mate_picks_picker_idx on app.mate_picks(picker_id, picked_id);
create index mate_picks_picked_idx on app.mate_picks(picked_id);

-- Reports about someone met through an outing, for moderation (M8). A report
-- also keeps the two out of each other's groups from then on.
create table app.reports (
  id          uuid primary key default gen_random_uuid(),
  -- Kept when the reporter deletes their account: what happened still
  -- matters for moderation, and the pair can no longer meet anyway.
  reporter_id uuid references app.users(id) on delete set null,
  reported_id uuid not null references app.users(id) on delete cascade,
  event_id    uuid references app.events(id) on delete set null,
  reason      text not null check (reason in ('dating', 'harassment', 'unsafe', 'no_show', 'other')),
  details     text not null default '' check (char_length(details) <= 1000),
  status      text not null default 'open' check (status in ('open', 'actioned', 'dismissed')),
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),
  reviewed_at timestamptz,
  check (reporter_id <> reported_id)
);
-- One report per person per outing; reporting again updates it.
create unique index reports_once_idx on app.reports(reporter_id, reported_id, event_id);
create index reports_reported_idx on app.reports(reported_id);
create index reports_open_idx on app.reports(created_at) where status = 'open';

-- Signals for moderators that need a person to look, not an automatic
-- action. At most one open flag of each kind per person; it's updated while
-- open and stays until a moderator resolves it.
create table app.moderation_flags (
  id          bigint generated always as identity primary key,
  user_id     uuid not null references app.users(id) on delete cascade,
  -- pick_gender_lean: their meet-again picks lean heavily towards one gender
  -- compared with who they met (internal/matching.PickLean).
  kind        text not null check (kind in ('pick_gender_lean')),
  details     jsonb not null,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),
  resolved_at timestamptz
);
create unique index moderation_flags_open_idx on app.moderation_flags(user_id, kind) where resolved_at is null;

-- "How was it?", once per event, to the people who confirmed.
alter table app.notifications
  drop constraint notifications_kind_check,
  add constraint notifications_kind_check check (kind in ('event_invited', 'group_confirmed', 'group_cancelled', 'plan_no_group', 'event_feedback'));

-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on app.event_feedback, app.mate_picks, app.reports, app.moderation_flags from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
delete from app.notifications where kind = 'event_feedback';
alter table app.notifications
  drop constraint notifications_kind_check,
  add constraint notifications_kind_check check (kind in ('event_invited', 'group_confirmed', 'group_cancelled', 'plan_no_group'));
drop table app.moderation_flags;
drop table app.reports;
drop table app.mate_picks;
drop table app.event_feedback;
