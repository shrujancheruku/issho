-- name: UpsertPushToken :exec
insert into app.push_tokens (token, user_id, platform) values (@token, @user_id, @platform)
on conflict (token) do update set user_id = excluded.user_id, platform = excluded.platform, last_seen_at = now();

-- Only the caller's own token: someone else's can't be removed by guessing it.
-- name: DeletePushToken :exec
delete from app.push_tokens where token = @token and user_id = @user_id;
