-- The queue's table, app.queue_items; only internal/staff calls these.

-- name: InsertQueueItem :one
insert into app.queue_items (kind, subject, due_at, created_at)
values ($1, $2, $3, $4)
returning *;

-- name: ListOpenQueueItems :many
select * from app.queue_items
where state = 'open'
order by due_at, id;

-- name: ResolveQueueItem :one
update app.queue_items
set state = 'resolved', reason = $2, closed_at = $3
where id = $1 and state = 'open'
returning *;

-- The due job takes the row lock, so a person's resolve waits for the default
-- to finish (and then finds the item no longer open) instead of racing it.
-- name: LockQueueItem :one
select * from app.queue_items where id = $1 for update;

-- name: DefaultQueueItem :exec
update app.queue_items
set state = 'defaulted', closed_at = $2
where id = $1 and state = 'open';
