-- Scheduled events as their invitees see them, and what the worker and
-- EventService change about them. One row per (event, person): a person is
-- in at most one group per event.

-- name: ListUserEvents :many
select e.id, e.date, e.block, e.starts_at, e.confirm_by, e.status,
       p.id as plan_id, p.city_id, p.area_id, p.activity_id, p.follow_up_id, p.primary_language,
       p.secondary_language, p.venue_place_id, p.follow_up_venue_place_id, p.route_polyline, p.route_seconds,
       g.id as group_id, g.status as group_status, g.quorum, g.min_attendance, g.venue_place_id as group_venue_place_id,
       g.meeting_point, m.response, m.checked_in_at,
       (select count(*) from app.group_members c where c.group_id = g.id and c.response = 'confirmed')::int as confirmed
from app.group_members m
join app.event_groups g on g.id = m.group_id
join app.events e on e.id = m.event_id
join app.plans p on p.id = e.plan_id
where m.user_id = @user_id and e.date >= @since
order by e.date, e.block, e.id;

-- People who confirmed for a group, for its attendee list (first names
-- only; shown only to people who confirmed themselves).
-- name: ListConfirmedNames :many
select pr.display_name
from app.group_members m
join app.profiles pr on pr.user_id = m.user_id
where m.group_id = $1 and m.response = 'confirmed'
order by m.responded_at, pr.display_name;

-- Locks one person's invitation, their group and its event, so two answers
-- (or an answer and the worker's deadline pass) can't race on the counts.
-- name: LockMembership :one
select m.group_id, m.response, g.status as group_status, g.quorum, g.min_attendance,
       e.status as event_status, e.confirm_by, e.starts_at, e.plan_id
from app.group_members m
join app.event_groups g on g.id = m.group_id
join app.events e on e.id = m.event_id
where m.event_id = @event_id and m.user_id = @user_id
for update of m, g, e;

-- name: SetResponse :exec
update app.group_members set response = @response, responded_at = @at
where group_id = @group_id and user_id = @user_id;

-- name: CountConfirmed :one
select count(*)::int from app.group_members where group_id = $1 and response = 'confirmed';

-- name: SetGroupStatus :exec
update app.event_groups set status = @status where id = @id;

-- name: ListConfirmedUsers :many
select user_id from app.group_members where group_id = $1 and response = 'confirmed' order by user_id;

-- People whose group was cancelled go back to waiting for the plan, if it's
-- still open (with their profile availability: their earlier times went
-- when they were invited).
-- name: ReturnToSignups :exec
insert into app.plan_signups (plan_id, user_id)
select @plan_id, unnest(@user_ids::uuid[])
where exists (select 1 from app.plans where id = @plan_id and status = 'open')
on conflict (plan_id, user_id) do nothing;

-- An event with every group cancelled is cancelled.
-- name: CancelEventIfNoGroups :exec
update app.events e set status = 'cancelled'
where e.id = @event_id and e.status = 'scheduled'
  and not exists (select 1 from app.event_groups g where g.event_id = e.id and g.status <> 'cancelled');

-- Groups still collecting answers whose confirmation deadline has passed.
-- name: ListGroupsToSettle :many
select g.id, g.event_id
from app.event_groups g
join app.events e on e.id = g.event_id
join app.plans p on p.id = e.plan_id
where p.city_id = @city_id and g.status = 'inviting' and e.status = 'scheduled' and e.confirm_by <= @now
order by e.confirm_by, g.id;

-- Locks a group for settling, with what settling needs.
-- name: LockGroup :one
select g.status, g.min_attendance, e.plan_id, e.id as event_id
from app.event_groups g
join app.events e on e.id = g.event_id
where g.id = $1
for update of g, e;

-- name: FinishPastEvents :execrows
update app.events e set status = 'done'
from app.plans p
where p.id = e.plan_id and p.city_id = @city_id and e.status = 'scheduled' and e.date < @today;

-- Checking in: only someone who confirmed, once (checking in twice keeps
-- the first time).
-- name: CheckIn :execrows
update app.group_members set checked_in_at = @at
where event_id = @event_id and user_id = @user_id and response = 'confirmed' and checked_in_at is null;

-- name: GetCheckIn :one
select checked_in_at from app.group_members where event_id = @event_id and user_id = @user_id;
