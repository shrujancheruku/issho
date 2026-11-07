-- Whether and how someone may use an event's group chat: their group, their
-- answer, and when the event starts.
-- name: GetChatAccess :one
select m.group_id, m.response, g.status as group_status, e.status as event_status, e.starts_at, e.confirm_by
from app.group_members m
join app.event_groups g on g.id = m.group_id
join app.events e on e.id = m.event_id
where m.event_id = @event_id and m.user_id = @user_id and e.status <> 'proposed';

-- The room of an event's group or of a crew (chat.State.Topic is its id).
-- name: GetGroupRoom :one
select id from app.rooms where group_id = $1;

-- name: GetCrewRoom :one
select id from app.rooms where crew_id = $1;

-- Rooms are locked while a message is stored (internal/chat): ids come from
-- a sequence when the row is inserted, but rows become visible when their
-- transaction commits, and readers follow "id > cursor". Posting under one
-- lock per room makes ids commit in order, so a lower id can never appear
-- after a reader has moved past it. No key update: it still serialises
-- posters (and anything updating the row) but not the foreign-key checks of
-- unrelated inserts.
-- name: LockRoom :one
select id from app.rooms where id = $1 for no key update;

-- The same lock, found by crew, for the plan Issho suggests in a crew's room.
-- name: LockCrewRoomByCrew :one
select id from app.rooms where crew_id = $1 for no key update;

-- A message is someone's text, or a plan Issho suggested (plan_id, no author).
-- name: InsertMessage :one
insert into app.messages (room_id, user_id, body) values (@room_id, @user_id, @body)
returning id, created_at;

-- name: InsertPlanMessage :exec
insert into app.messages (room_id, plan_id, created_at) values (@room_id, @plan_id, @at);

-- Tells every API instance listening on "chat" that a room has a new
-- message; they push it to the sockets open on that room. Sent in the same
-- transaction as the insert, so it's delivered only once it's committed.
-- name: NotifyChat :exec
select pg_notify('chat', @room_id::text);

-- Someone in a room is typing: the API instances' listeners pass it to the
-- other sockets open on that room (internal/chatws). Payload: JSON with the
-- room id, who and their first name. Not stored anywhere.
-- name: NotifyTyping :exec
select pg_notify('chat_typing', @payload::text);

-- The first name shown when someone is typing.
-- name: GetDisplayName :one
select display_name from app.profiles where user_id = $1;

-- Messages after a given one, oldest first, with the sender's first name.
-- name: ListMessages :many
select m.id, m.user_id, pr.display_name, m.body, m.plan_id, m.created_at
from app.messages m
left join app.profiles pr on pr.user_id = m.user_id
where m.room_id = @room_id and m.id > @after_id
order by m.id
limit @max;

-- The latest messages, for opening a room.
-- name: ListLatestMessages :many
select * from (
  select m.id, m.user_id, pr.display_name, m.body, m.plan_id, m.created_at
  from app.messages m
  left join app.profiles pr on pr.user_id = m.user_id
  where m.room_id = @room_id
  order by m.id desc
  limit @max
) latest order by id;

-- Privacy: the messages of outings more than 30 days past (@before_date) and
-- of crews closed more than 30 days ago (@before_at). A crew that lapsed or
-- closed has closed_at set (UpdateCrewStatus), so one rule covers both;
-- someone leaving an active crew doesn't delete their messages. At most @max
-- rows per call so the daily run never holds a long lock: the caller repeats
-- until fewer come back. messages_room_idx finds a room's messages; each side
-- starts from the few old events or closed crews.
-- name: DeleteOldMessages :execrows
delete from app.messages
where id in (
  select old.id from (
    select m.id from app.rooms r
    join app.event_groups g on g.id = r.group_id
    join app.events e on e.id = g.event_id
    join app.messages m on m.room_id = r.id
    where e.date < @before_date
    union all
    select m.id from app.rooms r
    join app.crews c on c.id = r.crew_id
    join app.messages m on m.room_id = r.id
    where c.closed_at < @before_at
  ) old
  order by old.id
  limit @max
);
