-- Plan review (M8, ReviewService): admins approve, reject, edit and create
-- plans. Names are the English ones.

-- name: ListReviewCities :many
select c.id, coalesce(c.name->>'en', c.id)::text as name, c.review,
       (select count(*) from app.plans p where p.city_id = c.id and p.status = 'draft')::int as drafts,
       (select count(*) from app.plans p where p.city_id = c.id and p.status = 'open')::int as open
from app.cities c
order by c.id;

-- name: SetCityReview :execrows
update app.cities set review = @review where id = @id;

-- A city's plans in one state: drafts oldest first (they've waited
-- longest), others newest first.
-- name: ListReviewPlans :many
select p.id, p.city_id, p.status, coalesce(p.closed_reason, '')::text as closed_reason,
       p.title, p.description, p.area_id, coalesce(a.name->>'en', p.area_id)::text as area_name,
       p.activity_id, p.follow_up_id, p.primary_language, p.secondary_language,
       exists (select 1 from app.plan_stops st where st.plan_id = p.id) as walk,
       p.audience, p.score, (select count(*) from app.plan_signups s where s.plan_id = p.id)::int as signups,
       p.window_start, p.window_days, p.signup_deadline, p.created_at, p.created_by, p.reviewed_by, p.reviewed_at
from app.plans p
join app.areas a on a.id = p.area_id
where p.city_id = @city_id and p.status = @status
order by case when p.status = 'draft' then p.created_at end asc, p.created_at desc, p.id
limit @max_rows;

-- One plan, with what its stops need.
-- name: GetReviewPlan :one
select p.id, p.city_id, p.status, coalesce(p.closed_reason, '')::text as closed_reason,
       p.title, p.description, p.area_id, coalesce(a.name->>'en', p.area_id)::text as area_name,
       p.activity_id, p.follow_up_id, p.primary_language, p.secondary_language,
       exists (select 1 from app.plan_stops st where st.plan_id = p.id) as walk,
       p.audience, p.score, (select count(*) from app.plan_signups s where s.plan_id = p.id)::int as signups,
       p.window_start, p.window_days, p.signup_deadline, p.created_at, p.created_by, p.reviewed_by, p.reviewed_at,
       p.venue_place_id, p.follow_up_venue_place_id, p.meeting_place_id, p.route_polyline, p.route_seconds,
       st_y(a.centroid::geometry)::float8 as area_lat, st_x(a.centroid::geometry)::float8 as area_lng,
       (select count(*) from app.events e where e.plan_id = p.id and e.status = 'scheduled'
          and (e.starts_at is null or e.starts_at > now()))::int as upcoming_events
from app.plans p
join app.areas a on a.id = p.area_id
where p.id = $1;

-- Opening a draft starts its first round now.
-- name: ApproveDraft :execrows
update app.plans
set status = 'open', window_start = @window_start, window_days = @window_days, signup_deadline = @signup_deadline,
    reviewed_by = @reviewed_by, reviewed_at = @at
where id = @id and status = 'draft';

-- name: RejectDraft :execrows
update app.plans
set status = 'closed', closed_reason = 'rejected', closed_at = @at, reviewed_by = @reviewed_by, reviewed_at = @at
where id = @id and status = 'draft';

-- name: EditPlanText :execrows
update app.plans set title = sqlc.narg(title), description = sqlc.narg(description)
where id = @id and status in ('draft', 'open');

-- name: WithdrawPlan :execrows
update app.plans set status = 'closed', closed_reason = 'withdrawn', closed_at = @at
where id = @id and status = 'open';

-- Venues for an activity in an area, as PickVenue would choose from: one
-- row per place (a place can be in two categories), approved first.
-- name: ListVenueOptions :many
select v.google_place_id, min(v.category_id)::text as category_id,
       (case when bool_or(v.status = 'approved') then 'approved' else 'candidate' end)::text as status
from app.venues v
where v.area_id = @area_id and v.category_id = any(@categories::text[]) and v.status <> 'rejected'
group by v.google_place_id
order by bool_or(v.status = 'approved') desc, v.google_place_id
limit @max_rows;

-- Group review (00022): proposals waiting in a city, or its upcoming
-- scheduled events (for meeting points). Soonest first.
-- name: ListReviewEvents :many
select e.id, e.plan_id, e.status, e.date, e.block, e.starts_at, e.confirm_by, e.forced, e.created_at,
       p.title, p.activity_id, p.follow_up_id, coalesce(a.name->>'en', p.area_id)::text as area_name,
       (select count(*) from app.event_groups g where g.event_id = e.id and g.status <> 'cancelled')::int as groups,
       (select count(*) from app.group_members m where m.event_id = e.id and m.response in ('invited', 'confirmed'))::int as people,
       c.timezone
from app.events e
join app.plans p on p.id = e.plan_id
join app.areas a on a.id = p.area_id
join app.cities c on c.id = p.city_id
where p.city_id = @city_id and e.status = @status and (e.status = 'proposed' or e.date >= @from_date)
order by e.date, e.block, e.id
limit @max_rows;

-- name: GetReviewEvent :one
select e.id, e.plan_id, e.status, e.round, e.date, e.block, e.starts_at, e.confirm_by, e.forced, e.created_at,
       p.city_id, p.title, p.activity_id, p.follow_up_id, p.primary_language, p.secondary_language,
       coalesce(a.name->>'en', p.area_id)::text as area_name, c.timezone
from app.events e
join app.plans p on p.id = e.plan_id
join app.areas a on a.id = p.area_id
join app.cities c on c.id = p.city_id
where e.id = $1;

-- name: ListReviewEventGroups :many
select g.id, g.quorum, g.min_attendance, g.score, g.status, g.meeting_point
from app.event_groups g
where g.event_id = $1
order by g.created_at, g.id;

-- Who's in each group, with what an admin balances groups by.
-- name: ListReviewEventMembers :many
select m.group_id, m.user_id, m.response,
       coalesce(pr.display_name, '')::text as name, coalesce(pr.gender, '')::text as gender,
       coalesce(pr.birth_year, 0)::int as birth_year,
       exists (select 1 from app.plan_signups s join app.events e on e.plan_id = s.plan_id
               where e.id = m.event_id and s.user_id = m.user_id) as signed_up
from app.group_members m
left join app.profiles pr on pr.user_id = m.user_id
where m.event_id = $1
order by m.group_id, pr.display_name, m.user_id;

-- name: LockProposal :one
select e.plan_id, e.round, e.status
from app.events e
where e.id = $1
for update;

-- The group must be one of the event's (the foreign key on
-- (group_id, event_id) checks it).
-- name: MoveProposalMember :execrows
update app.group_members set group_id = @group_id
where event_id = @event_id and user_id = @user_id;

-- name: DeleteProposalMember :execrows
delete from app.group_members where event_id = @event_id and user_id = @user_id;

-- name: SetProposalStart :exec
update app.events set date = @date, block = @block, starts_at = @starts_at, confirm_by = @confirm_by
where id = @id;

-- name: SetGroupMeetingPoint :execrows
update app.event_groups g set meeting_point = sqlc.narg(meeting_point)
from app.events e
where g.id = @id and e.id = g.event_id and e.status in ('proposed', 'scheduled');

-- name: GetGroupEventStatus :one
select e.id as event_id, e.status, p.primary_language, p.secondary_language
from app.event_groups g
join app.events e on e.id = g.event_id
join app.plans p on p.id = e.plan_id
where g.id = $1;

-- At approval: people who withdrew (or were suspended, which withdraws
-- them) since the proposal aren't invited.
-- name: DropMembersWithoutSignup :exec
delete from app.group_members m
where m.event_id = @event_id
  and not exists (select 1 from app.plan_signups s where s.plan_id = @plan_id and s.user_id = m.user_id);

-- name: DeleteEmptyGroups :exec
delete from app.event_groups g
where g.event_id = $1 and not exists (select 1 from app.group_members m where m.group_id = g.id);

-- name: ListEventMemberIDs :many
select user_id from app.group_members where event_id = $1 order by user_id;

-- name: CountPlanSignups :one
select count(*)::int from app.plan_signups where plan_id = $1;

-- name: ApproveProposalEvent :exec
update app.events set status = 'scheduled', reviewed_by = @reviewed_by, reviewed_at = @at
where id = @id;

-- name: DeleteProposal :exec
delete from app.events where id = $1 and status = 'proposed';

-- name: SetProposalRejected :exec
update app.plans set proposal_rejected_at = @at where id = @id;

-- Editing stops (admin): a walk's stops are replaced whole.
-- name: DeletePlanStops :exec
delete from app.plan_stops where plan_id = $1;

-- name: SetPlanVenues :exec
update app.plans set venue_place_id = sqlc.narg(venue_place_id), follow_up_venue_place_id = sqlc.narg(follow_up_venue_place_id)
where id = @id;

-- name: ClearPlanRoute :exec
update app.plans set route_polyline = null, route_seconds = null, route_computed_at = null where id = $1;

-- Locks a plan an admin edits, with its state.
-- name: LockPlanForEdit :one
select status, exists (select 1 from app.plan_stops s where s.plan_id = p.id) as walk
from app.plans p where p.id = $1 for update;
