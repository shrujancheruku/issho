-- Whether and how someone may use an event's group chat: their group, their
-- answer, and when the event starts.
-- name: GetChatAccess :one
select m.group_id, m.response, g.status as group_status, e.status as event_status, e.starts_at, e.confirm_by
from app.group_members m
join app.event_groups g on g.id = m.group_id
join app.events e on e.id = m.event_id
where m.event_id = @event_id and m.user_id = @user_id and e.status <> 'proposed';

-- name: InsertChatMessage :one
insert into app.chat_messages (group_id, user_id, body) values (@group_id, @user_id, @body)
returning id, created_at;

-- Tells every API instance listening on "chat" that a group has a new
-- message; they push it to the sockets open on that chat. Sent in the same
-- transaction as the insert, so it's delivered only once it's committed.
-- name: NotifyChat :exec
select pg_notify('chat', @group_id::text);

-- Someone in a chat is typing: the API instances' listeners pass it to the
-- other sockets open on that chat (internal/chatws). Payload: JSON with the
-- chat (group or crew id), who and their first name. Not stored anywhere.
-- name: NotifyTyping :exec
select pg_notify('chat_typing', @payload::text);

-- The first name shown when someone is typing.
-- name: GetDisplayName :one
select display_name from app.profiles where user_id = $1;

-- Messages after a given one, oldest first, with the sender's first name.
-- name: ListChatMessages :many
select c.id, c.user_id, pr.display_name, c.body, c.created_at
from app.chat_messages c
left join app.profiles pr on pr.user_id = c.user_id
where c.group_id = @group_id and c.id > @after_id
order by c.id
limit @max;

-- The latest messages, for opening a chat.
-- name: ListLatestChatMessages :many
select * from (
  select c.id, c.user_id, pr.display_name, c.body, c.created_at
  from app.chat_messages c
  left join app.profiles pr on pr.user_id = c.user_id
  where c.group_id = @group_id
  order by c.id desc
  limit @max
) latest order by id;

-- Chats of events more than 30 days past (privacy).
-- name: DeleteOldChats :execrows
delete from app.chat_messages c
using app.event_groups g, app.events e, app.plans p
where g.id = c.group_id and e.id = g.event_id and p.id = e.plan_id
  and p.city_id = @city_id and e.date < @before;
