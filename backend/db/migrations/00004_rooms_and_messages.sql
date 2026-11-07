-- +goose Up

-- One chat room table and one message table for outing and crew chat (#494),
-- replacing app.chat_messages (by group) and app.crew_messages (by crew).
-- Unread counts, moderators reading the room behind a report and client
-- message ids each want one place to hang off.
--
-- A room is written in the same transaction as its group (InsertEventGroup) or
-- its crew (CreateCrew): both queries insert the room in the same statement.
-- Not additive: the old tables are dropped, so deploy the code and the
-- migration together (there are no users yet).

create table app.rooms (
  id         uuid primary key default gen_random_uuid(),
  kind       text not null check (kind in ('outing', 'crew')),
  -- Unique: one room per group and per crew. Deleting either deletes its room
  -- and so its messages.
  group_id   uuid unique references app.event_groups(id) on delete cascade,
  crew_id    uuid unique references app.crews(id) on delete cascade,
  created_at timestamptz not null default now(),
  constraint rooms_owner check (
    (kind = 'outing' and group_id is not null and crew_id is null)
    or (kind = 'crew' and crew_id is not null and group_id is null))
);

-- Someone's text, or (user_id null) a plan Issho suggested in a crew's room.
create table app.messages (
  id         bigint generated always as identity primary key,
  room_id    uuid not null references app.rooms(id) on delete cascade,
  user_id    uuid references app.users(id) on delete cascade,
  body       text not null default '' check (char_length(body) <= 1000),
  plan_id    uuid references app.plans(id) on delete cascade,
  created_at timestamptz not null default now(),
  check ((user_id is null) = (plan_id is not null)),
  check (plan_id is not null or char_length(body) >= 1)
);
-- Reading a room after a given message.
create index messages_room_idx on app.messages(room_id, id);
create index messages_plan_id_idx on app.messages(plan_id);
create index messages_user_id_idx on app.messages(user_id);

insert into app.rooms (kind, group_id) select 'outing', id from app.event_groups;
insert into app.rooms (kind, crew_id) select 'crew', id from app.crews;

-- Copied in (created_at, id) order so the new ids follow it. A room's messages
-- all come from one old table, so ids only need to be ordered within a room.
insert into app.messages (room_id, user_id, body, plan_id, created_at)
select room_id, user_id, body, plan_id, created_at from (
  select r.id as room_id, m.user_id, m.body, null::uuid as plan_id, m.created_at, 0 as src, m.id
  from app.chat_messages m join app.rooms r on r.group_id = m.group_id
  union all
  select r.id, m.user_id, m.body, m.plan_id, m.created_at, 1, m.id
  from app.crew_messages m join app.rooms r on r.crew_id = m.crew_id
) old
order by created_at, src, id;

drop table app.chat_messages;
drop table app.crew_messages;

-- +goose Down

create table app.chat_messages (
  id         bigint generated always as identity primary key,
  group_id   uuid not null references app.event_groups(id) on delete cascade,
  user_id    uuid not null references app.users(id) on delete cascade,
  body       text not null check (char_length(body) between 1 and 1000),
  created_at timestamptz not null default now()
);
create index chat_messages_group_idx on app.chat_messages(group_id, id);
create index chat_messages_user_idx on app.chat_messages(user_id);

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
create index crew_messages_plan_id_idx on app.crew_messages(plan_id);
create index crew_messages_user_id_idx on app.crew_messages(user_id);

insert into app.chat_messages (group_id, user_id, body, created_at)
select r.group_id, m.user_id, m.body, m.created_at
from app.messages m join app.rooms r on r.id = m.room_id
where r.kind = 'outing'
order by m.id;

insert into app.crew_messages (crew_id, user_id, body, plan_id, created_at)
select r.crew_id, m.user_id, m.body, m.plan_id, m.created_at
from app.messages m join app.rooms r on r.id = m.room_id
where r.kind = 'crew'
order by m.id;

drop table app.messages;
drop table app.rooms;
