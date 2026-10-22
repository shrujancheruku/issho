-- name: InsertPlan :one
insert into app.plans (
  city_id, area_id, activity_id, follow_up_id, primary_language, secondary_language,
  audience, score, window_start, window_days, signup_deadline, venue_place_id, follow_up_venue_place_id
) values (
  @city_id, @area_id, @activity_id, @follow_up_id, @primary_language, @secondary_language,
  @audience, @score, @window_start, @window_days, @signup_deadline, @venue_place_id, @follow_up_venue_place_id
)
returning id;

-- A venue for an activity in an area: one of its venue categories, never a
-- rejected place, approved ones first (admin curation, M8), otherwise any
-- candidate at random so plans in one area don't all use the same café.
-- name: PickVenue :one
select google_place_id from app.venues
where area_id = @area_id and category_id = any(@categories::text[]) and status <> 'rejected'
order by (status = 'approved') desc, random()
limit 1;

-- name: SetPlanDescription :exec
update app.plans set description = @description where id = @id;

-- Signup counts come from the same rows the worker schedules from, so the
-- feed's "7 interested" always matches.
-- name: ListOpenPlans :many
select p.id, p.area_id, p.activity_id, p.follow_up_id, p.primary_language, p.secondary_language,
       p.round, p.window_start, p.window_days, p.signup_deadline, p.venue_place_id, p.follow_up_venue_place_id,
       p.route_polyline, p.route_seconds, p.meeting_place_id,
       (select count(*) from app.plan_signups s where s.plan_id = p.id)::int as signups
from app.plans p
where p.city_id = @city_id and p.status = 'open'
order by p.created_at, p.id;

-- name: GetPlan :one
select p.id, p.city_id, p.area_id, p.activity_id, p.follow_up_id, p.primary_language,
       p.secondary_language, p.description, p.round, p.window_start, p.window_days,
       p.signup_deadline, p.status, p.closed_reason, p.closed_at, p.created_at
from app.plans p
where p.id = $1;

-- Locks the plan so the worker and a signup arriving at the same moment
-- don't both act on a round that's being scheduled.
-- name: LockOpenPlan :one
select round from app.plans where id = $1 and status = 'open' for update;

-- name: ClosePlan :exec
update app.plans set status = 'closed', closed_reason = @reason::text, closed_at = @closed_at
where id = @id and status = 'open';

-- Closing a plan no group could form for, and telling whoever was still
-- waiting, in one statement (nothing is sent if the plan was already
-- closed). Their signups stay, as the plan's record.
-- name: ClosePlanNoGroup :exec
with closed as (
  update app.plans p set status = 'closed', closed_reason = 'no_group', closed_at = @closed_at
  where p.id = @id and p.status = 'open'
  returning p.id
)
insert into app.notifications (user_id, kind, plan_id)
select s.user_id, 'plan_no_group', s.plan_id from app.plan_signups s join closed c on c.id = s.plan_id;

-- name: NextPlanRound :exec
update app.plans
set round = round + 1, window_start = @window_start, window_days = @window_days,
    signup_deadline = @signup_deadline
where id = @id and status = 'open';

-- name: UpsertSignup :exec
insert into app.plan_signups (plan_id, user_id) values ($1, $2)
on conflict (plan_id, user_id) do nothing;

-- name: DeleteSignup :exec
delete from app.plan_signups where plan_id = $1 and user_id = $2;

-- name: DeleteSignupTimes :exec
-- Before saving a signup's times again: they're replaced wholesale.
with slots as (
  delete from app.plan_signup_slots s where s.plan_id = @plan_id and s.user_id = @user_id
)
delete from app.plan_signup_unavailable u where u.plan_id = @plan_id and u.user_id = @user_id;

-- The arrays are parallel; unnest in the select list zips them (as in profile.sql).
-- name: InsertSignupSlots :exec
insert into app.plan_signup_slots (plan_id, user_id, weekday, block)
select @plan_id, @user_id, unnest(@weekdays::smallint[]), unnest(@blocks::text[]);

-- name: InsertSignupUnavailable :exec
insert into app.plan_signup_unavailable (plan_id, user_id, date, block)
select @plan_id, @user_id, unnest(@dates::date[]), unnest(@blocks::text[]);

-- name: ListSignups :many
select user_id from app.plan_signups where plan_id = $1 order by user_id;

-- name: ListSignupSlots :many
select user_id, weekday, block from app.plan_signup_slots
where plan_id = $1 order by user_id, weekday, block;

-- name: ListSignupUnavailable :many
select user_id, date, block from app.plan_signup_unavailable
where plan_id = $1 order by user_id, date, block;

-- name: DeleteSignups :exec
-- People placed in a group stop waiting: their invitation records them now.
delete from app.plan_signups where plan_id = @plan_id and user_id = any(@user_ids::uuid[]);

-- name: InsertEvent :one
insert into app.events (plan_id, round, date, block, starts_at, confirm_by, forced)
values (@plan_id, @round, @date, @block, @starts_at, @confirm_by, @forced)
returning id;

-- name: InsertEventGroup :one
insert into app.event_groups (event_id, quorum, min_attendance, score)
values (@event_id, @quorum, @min_attendance, @score)
returning id;

-- name: InsertGroupMembers :exec
insert into app.group_members (group_id, event_id, user_id)
select @group_id, @event_id, unnest(@user_ids::uuid[]);

-- Occasions a person is already invited to or going to, so a signup for
-- another plan can't double-book them (matching.Signup.Unavailable).
-- name: ListBusyOccasions :many
select m.user_id, e.date, e.block
from app.group_members m
join app.events e on e.id = m.event_id
where m.user_id = any(@user_ids::uuid[])
  and m.response in ('invited', 'confirmed')
  and e.status = 'scheduled'
  and e.date >= @from_date
order by m.user_id, e.date, e.block;

-- name: ListUserOpenSignups :many
select s.plan_id from app.plan_signups s
join app.plans p on p.id = s.plan_id
where s.user_id = $1 and p.status = 'open';

-- name: ListUserSignupSlots :many
select weekday, block from app.plan_signup_slots
where plan_id = $1 and user_id = $2 order by weekday, block;

-- name: ListUserSignupUnavailable :many
select date, block from app.plan_signup_unavailable
where plan_id = $1 and user_id = $2 order by date, block;

-- A plan in the city that closed after @since, for its screen: people who
-- were waiting (or follow a link) see why it closed instead of "not found".
-- name: GetRecentlyClosedPlan :one
select p.id, p.area_id, p.activity_id, p.follow_up_id, p.primary_language, p.secondary_language,
       p.round, p.window_start, p.window_days, p.signup_deadline, p.venue_place_id, p.follow_up_venue_place_id,
       p.route_polyline, p.route_seconds, p.meeting_place_id, p.closed_reason::text as closed_reason, p.closed_at,
       (select count(*) from app.plan_signups s where s.plan_id = p.id)::int as signups,
       exists (select 1 from app.plan_signups s where s.plan_id = p.id and s.user_id = @user_id) as signed_up
from app.plans p
where p.id = @id and p.city_id = @city_id and p.status = 'closed' and p.closed_at > @since;
