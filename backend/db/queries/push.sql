-- name: UpsertPushToken :exec
insert into app.push_tokens (token, user_id, platform) values (@token, @user_id, @platform)
on conflict (token) do update set user_id = excluded.user_id, platform = excluded.platform, last_seen_at = now();

-- Only the caller's own token: someone else's can't be removed by guessing it.
-- name: DeletePushToken :exec
delete from app.push_tokens where token = @token and user_id = @user_id;

-- Tokens Expo says are no longer registered (the app was uninstalled).
-- name: DeletePushTokens :exec
delete from app.push_tokens where token = any(@tokens::text[]);

-- name: InsertNotifications :exec
insert into app.notifications (user_id, kind, event_id)
select unnest(@user_ids::uuid[]), @kind, @event_id;

-- Pending notifications with what the message needs: the person's display
-- language and the event's plan. Oldest first; a few attempts at most, so
-- one that keeps failing doesn't block the rest forever.
-- name: ListPendingNotifications :many
select n.id, n.user_id, n.kind, n.event_id, pr.primary_language, p.city_id, p.activity_id, p.follow_up_id
from app.notifications n
join app.profiles pr on pr.user_id = n.user_id
join app.events e on e.id = n.event_id
join app.plans p on p.id = e.plan_id
where n.sent_at is null and n.attempts < 5
order by n.created_at, n.id
limit @max;

-- name: ListPushTokens :many
select user_id, token from app.push_tokens where user_id = any(@user_ids::uuid[]) order by user_id, token;

-- name: MarkNotificationsSent :exec
update app.notifications set sent_at = @at where id = any(@ids::bigint[]);

-- name: MarkNotificationsFailed :exec
update app.notifications set attempts = attempts + 1 where id = any(@ids::bigint[]);
