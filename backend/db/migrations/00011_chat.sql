-- +goose Up

-- Group chat for the people going on an event (M6): one chat per group,
-- open from 48 hours before the start until 24 hours after. There are no
-- other messages: no 1:1 chat (the product's platonic rules). Messages are
-- deleted 30 days after the event (the worker), and with the sender's
-- account.
create table app.chat_messages (
  id         bigint generated always as identity primary key,
  group_id   uuid not null references app.event_groups(id) on delete cascade,
  user_id    uuid not null references app.users(id) on delete cascade,
  body       text not null check (char_length(body) between 1 and 1000),
  created_at timestamptz not null default now()
);
-- Reading a chat after a given message.
create index chat_messages_group_idx on app.chat_messages(group_id, id);
create index chat_messages_user_idx on app.chat_messages(user_id);

-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on app.chat_messages from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
drop table app.chat_messages;
