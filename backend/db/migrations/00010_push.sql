-- +goose Up

-- Devices to send push notifications to (Expo push tokens, M6). A person
-- can have several; a token belongs to the account last signed in on that
-- device, so registering it again moves it.
create table app.push_tokens (
  token        text primary key check (token like 'ExponentPushToken[%]' or token like 'ExpoPushToken[%]'),
  user_id      uuid not null references app.users(id) on delete cascade,
  platform     text not null check (platform in ('ios', 'android', 'web')),
  created_at   timestamptz not null default now(),
  last_seen_at timestamptz not null default now()
);
create index push_tokens_user_idx on app.push_tokens(user_id);

-- Notifications to send, written in the same transaction as the change that
-- causes them (an outbox): a round scheduled, a group going ahead or
-- cancelled. A worker job sends them and marks them sent, so a change that
-- rolls back never notifies anyone, and a failed send is retried.
create table app.notifications (
  id         bigint generated always as identity primary key,
  user_id    uuid not null references app.users(id) on delete cascade,
  kind       text not null check (kind in ('event_invited', 'group_confirmed', 'group_cancelled')),
  event_id   uuid not null references app.events(id) on delete cascade,
  created_at timestamptz not null default now(),
  sent_at    timestamptz,
  attempts   smallint not null default 0
);
-- The sender reads only what's still to send.
create index notifications_pending_idx on app.notifications(created_at) where sent_at is null;
create index notifications_user_idx on app.notifications(user_id);
create index notifications_event_idx on app.notifications(event_id);

-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on app.push_tokens, app.notifications from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
drop table app.notifications;
drop table app.push_tokens;
