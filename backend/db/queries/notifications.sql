-- The notification outbox (internal/notifications): rows written in the
-- caller's transaction, sent by the worker, with Expo's tickets kept until
-- their receipts are checked.

-- Written through notifications.Enqueue; exactly one of the three ids is set.
-- name: InsertNotifications :exec
insert into app.notifications (user_id, kind, event_id, plan_id, crew_id)
select unnest(@user_ids::uuid[]), @kind, @event_id, @plan_id, @crew_id;

-- Pending notifications: who, which kind, what it is about and the person's
-- display language. What the message says is the owning module's job (it
-- looks up its own plan or crew from the ids). Oldest first; a few
-- attempts at most, so one that keeps failing doesn't block the rest.
-- name: ListPendingNotifications :many
select n.id, n.user_id, n.kind, n.event_id, n.plan_id, n.crew_id, pr.primary_language
from app.notifications n
join app.profiles pr on pr.user_id = n.user_id
where n.sent_at is null and n.attempts < 5
order by n.created_at, n.id
limit @max;

-- Suspended people get no notifications; their tokens are kept for when
-- the suspension is lifted.
-- name: ListPushTokens :many
select t.user_id, t.token from app.push_tokens t
where t.user_id = any(@user_ids::uuid[])
  and not exists (select 1 from app.suspensions s where s.user_id = t.user_id and s.lifted_at is null)
order by t.user_id, t.token;

-- name: MarkNotificationsSent :exec
update app.notifications set sent_at = @at where id = any(@ids::bigint[]);

-- name: MarkNotificationsFailed :exec
update app.notifications set attempts = attempts + 1 where id = any(@ids::bigint[]);

-- Notifications older than @before that were sent or ran out of attempts
-- (privacy: the queue keeps no history of who was told what beyond 30
-- days). Unsent rows with attempts left are kept: the sender still wants
-- them. At most @max per call so the daily run never holds a long lock;
-- the caller repeats until fewer come back. No index covers "old and
-- done" (notifications_pending_idx holds only the pending rows), so each
-- batch scans the table, which holds 30 days of a push queue.
-- name: DeleteOldNotifications :execrows
delete from app.notifications
where id in (
  select n.id from app.notifications n
  where n.created_at < @before and (n.sent_at is not null or n.attempts >= 5)
  order by n.id
  limit @max
);

-- Tickets for pushes Expo accepted, to check their receipts later. A token
-- deleted meanwhile (DeviceNotRegistered on another ticket) is skipped.
-- name: InsertPushTickets :exec
insert into app.push_tickets (id, token, created_at)
select t.id, t.token, @at
from (select unnest(@ids::text[]) as id, unnest(@tokens::text[]) as token) t
where exists (select 1 from app.push_tokens k where k.token = t.token)
on conflict (id) do nothing;

-- Tickets whose receipts should be ready: sent before @ready, oldest first.
-- name: ListDuePushTickets :many
select id, token from app.push_tickets where created_at < @ready order by created_at limit @max;

-- name: DeletePushTickets :exec
delete from app.push_tickets where id = any(@ids::text[]);

-- Receipts are kept for 24 hours; one still missing after that never comes.
-- name: DeleteExpiredPushTickets :execrows
delete from app.push_tickets where created_at < @before;

-- Tokens Expo says are no longer registered (the app was uninstalled).
-- name: DeletePushTokens :exec
delete from app.push_tokens where token = any(@tokens::text[]);
