-- After an outing (M7): feedback, meet-again picks, reports, and what
-- matching and moderation read from them.

-- The others who went ahead with the caller's group: the people they can
-- pick or report. First names only, like the attendee list. People who
-- cancelled (dropped) can still be reported: they may have been in the chat.
-- name: ListGroupMates :many
select m.user_id, pr.display_name, m.response
from app.group_members m
join app.profiles pr on pr.user_id = m.user_id
where m.group_id = @group_id and m.user_id <> @user_id and m.response in ('confirmed', 'dropped')
order by pr.display_name, m.user_id;

-- name: GetFeedback :one
select attended, plan_rating, group_rating, updated_at
from app.event_feedback
where group_id = @group_id and user_id = @user_id;

-- name: ListPicks :many
select picked_id, pick from app.mate_picks
where group_id = @group_id and picker_id = @picker_id
order by picked_id;

-- name: UpsertFeedback :exec
insert into app.event_feedback (group_id, user_id, attended, plan_rating, group_rating, created_at, updated_at)
values (@group_id, @user_id, @attended, @plan_rating, @group_rating, @at, @at)
on conflict (group_id, user_id) do update
set attended = excluded.attended, plan_rating = excluded.plan_rating,
    group_rating = excluded.group_rating, updated_at = excluded.updated_at;

-- name: DeletePicks :exec
delete from app.mate_picks where group_id = @group_id and picker_id = @picker_id;

-- name: InsertPicks :exec
insert into app.mate_picks (group_id, picker_id, picked_id, pick, created_at)
select @group_id, @picker_id, unnest(@picked_ids::uuid[]), @pick, @at;

-- Everyone the person met at outings they went to (by their feedback),
-- with whether they picked them: the input to matching.PickLean.
-- name: ListMetForPicks :many
select pr.gender,
       exists (select 1 from app.mate_picks k
               where k.group_id = f.group_id and k.picker_id = f.user_id and k.picked_id = m.user_id and k.pick = 'again') as picked
from app.event_feedback f
join app.group_members m on m.group_id = f.group_id and m.user_id <> f.user_id and m.response = 'confirmed'
join app.profiles pr on pr.user_id = m.user_id
where f.user_id = $1 and f.attended;

-- An open flag of a kind is updated rather than repeated.
-- name: UpsertModerationFlag :exec
insert into app.moderation_flags (user_id, kind, details, created_at, updated_at)
values (@user_id, @kind, @details, @at, @at)
on conflict (user_id, kind) where resolved_at is null do update
set details = excluded.details, updated_at = excluded.updated_at;

-- Reporting someone again about the same outing updates the report.
-- name: UpsertReport :exec
insert into app.reports (reporter_id, reported_id, event_id, reason, details, created_at, updated_at)
values (@reporter_id, @reported_id, @event_id, @reason, @details, @at, @at)
on conflict (reporter_id, reported_id, event_id) where event_id is not null do update
set reason = excluded.reason, details = excluded.details, updated_at = excluded.updated_at;

-- Every meet-again pick made by someone in the city (matching.MutualLinks):
-- all of a person's picks, wherever they were, set their return rate.
-- name: ListCityPicks :many
select k.picker_id, k.picked_id
from app.mate_picks k
join app.profiles p on p.user_id = k.picker_id
join app.areas a on a.id = p.home_area_id
where a.city_id = $1 and k.pick = 'again'
order by k.picker_id, k.picked_id;

-- Pairs never to group again: someone reported the other, or said they'd
-- rather not be in a group with them again. Whatever a moderator decides,
-- they shouldn't meet again.
-- name: ListCityBlocks :many
select r.reporter_id::uuid as reporter_id, r.reported_id as other_id
from app.reports r
join app.profiles p on p.user_id = r.reported_id
join app.areas a on a.id = p.home_area_id
where a.city_id = @city_id and r.reporter_id is not null
union
select k.picker_id, k.picked_id
from app.mate_picks k
join app.profiles p on p.user_id = k.picker_id
join app.areas a on a.id = p.home_area_id
where a.city_id = @city_id and k.pick = 'avoid'
order by 1, 2;

-- Pairs in the city who have met: both said they went to the same outing.
-- name: ListCityMetPairs :many
select distinct fa.user_id as a_id, fb.user_id as b_id
from app.event_feedback fa
join app.event_feedback fb on fb.group_id = fa.group_id and fb.user_id > fa.user_id and fb.attended
join app.profiles p on p.user_id = fa.user_id
join app.areas a on a.id = p.home_area_id
where a.city_id = @city_id and fa.attended
  -- Nobody is offered a crew with someone suspended.
  and not exists (select 1 from app.suspensions s where s.user_id in (fa.user_id, fb.user_id) and s.lifted_at is null)
order by 1, 2;

-- "How was it?" for the people who went ahead with a group, once per
-- event, from when feedback opens (@opened: start times at or before it)
-- while it's still open (@since: start times after it). Not to anyone who
-- already answered.
-- name: QueueFeedbackRequests :execrows
insert into app.notifications (user_id, kind, event_id)
select m.user_id, 'event_feedback', e.id
from app.events e
join app.plans p on p.id = e.plan_id
join app.event_groups g on g.event_id = e.id and g.status = 'confirmed'
join app.group_members m on m.group_id = g.id and m.response = 'confirmed'
where p.city_id = @city_id and e.status <> 'cancelled'
  and e.starts_at <= @opened and e.starts_at > @since
  and not exists (select 1 from app.notifications n where n.event_id = e.id and n.kind = 'event_feedback')
  and not exists (select 1 from app.event_feedback f where f.group_id = g.id and f.user_id = m.user_id);

-- The meet-again picks needed for one person's friendships: theirs, and
-- those of everyone they picked who picked them back (each person's return
-- rate needs all their picks; matching.MutualLinks). Friends either of
-- them has since blocked (a report or "rather not") are left out.
-- name: ListFriendPicks :many
with friends as (
  select k.picked_id as id
  from app.mate_picks k
  where k.picker_id = @user_id and k.pick = 'again'
    and exists (select 1 from app.mate_picks r where r.picker_id = k.picked_id and r.picked_id = @user_id and r.pick = 'again')
    and not exists (select 1 from app.mate_picks x
                    where x.pick = 'avoid' and ((x.picker_id = @user_id and x.picked_id = k.picked_id) or (x.picker_id = k.picked_id and x.picked_id = @user_id)))
    and not exists (select 1 from app.reports x
                    where (x.reporter_id = @user_id and x.reported_id = k.picked_id) or (x.reporter_id = k.picked_id and x.reported_id = @user_id))
)
select distinct k.picker_id, k.picked_id
from app.mate_picks k
where k.pick = 'again' and (k.picker_id = @user_id or k.picker_id in (select id from friends))
order by 1, 2;

-- Which of these people have signed up for the city's open plans.
-- name: ListOpenSignupsBy :many
select s.plan_id, s.user_id
from app.plan_signups s
join app.plans p on p.id = s.plan_id
where p.city_id = @city_id and p.status = 'open' and s.user_id = any(@user_ids::uuid[])
order by s.plan_id, s.user_id;
