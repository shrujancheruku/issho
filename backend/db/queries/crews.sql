-- Crews (M7): offered to people who got on well across outings
-- (matching.FindCrews), then a standing group chat with occasional plan
-- suggestions.

-- Pairs where one declined or left a crew the other was in: they're not
-- offered a crew together again (matching.CrewGraph.Averse).
-- name: ListCityCrewAversions :many
select distinct m.user_id as averse_id, o.user_id as other_id
from app.crew_members m
join app.crews c on c.id = m.crew_id
join app.crew_members o on o.crew_id = m.crew_id and o.user_id <> m.user_id
where c.city_id = @city_id and m.response in ('declined', 'left')
order by 1, 2;

-- name: CreateCrew :one
insert into app.crews (city_id, activity_id, area_id, offer_until, created_at)
values (@city_id, @activity_id, @area_id, @offer_until, @at)
returning id;

-- name: InsertCrewMembers :exec
insert into app.crew_members (crew_id, user_id) select @crew_id, unnest(@user_ids::uuid[]);

-- name: InsertCrewNotifications :exec
insert into app.notifications (user_id, kind, crew_id)
select unnest(@user_ids::uuid[]), @kind, @crew_id;

-- Offers to decide: everyone answered, or the offer ran out.
-- name: ListCrewOffersToDecide :many
select c.id from app.crews c
where c.city_id = @city_id and c.status = 'offered'
  and (c.offer_until <= @now
       or not exists (select 1 from app.crew_members m where m.crew_id = c.id and m.response = 'offered'))
order by c.id;

-- name: LockCrew :one
select status, offer_until from app.crews where id = $1 for update;

-- name: ListCrewMemberIDs :many
select user_id from app.crew_members where crew_id = @crew_id and response = @response order by user_id;

-- name: SetCrewStatus :exec
update app.crews set status = @status,
  formed_at = case when @status = 'active' then @at else formed_at end,
  closed_at = case when @status in ('lapsed', 'closed') then @at else closed_at end
where id = @id;

-- Active crews due a plan suggestion: the first a day after forming, then
-- one a week (@before = a week ago).
-- name: ListCrewsToSuggest :many
select id from app.crews
where city_id = @city_id and status = 'active'
  and coalesce(suggested_at, formed_at - interval '6 days') < @before
order by id;

-- The best open plan for a crew: on its activity (as the main activity or
-- the follow-up) or in its area, in a language every member speaks,
-- signups open for at least another day, not suggested to it before.
-- name: GetCrewPlanSuggestion :one
select p.id from app.plans p
join app.crews c on c.id = @crew_id
where p.city_id = c.city_id and p.status = 'open' and p.signup_deadline > @after
  and (p.activity_id = c.activity_id or p.follow_up_id = c.activity_id or p.area_id = c.area_id)
  and not exists (select 1 from app.crew_messages m where m.crew_id = c.id and m.plan_id = p.id)
  and not exists (
    select 1 from app.crew_members cm
    where cm.crew_id = c.id and cm.response = 'accepted'
      and not exists (select 1 from app.user_languages l
                      where l.user_id = cm.user_id and l.fluent and l.lang = p.primary_language))
order by p.score desc, p.id
limit 1;

-- name: InsertCrewPlanMessage :exec
insert into app.crew_messages (crew_id, plan_id, created_at) values (@crew_id, @plan_id, @at);

-- name: SetCrewSuggested :exec
update app.crews set suggested_at = @at where id = @id;

-- The caller's crews: offers still open to them, and crews they're in.
-- name: ListUserCrews :many
select c.id, c.city_id, c.status, c.activity_id, c.area_id, c.offer_until, m.response
from app.crew_members m
join app.crews c on c.id = m.crew_id
where m.user_id = @user_id
  and ((c.status = 'offered' and m.response in ('offered', 'accepted')) or (c.status = 'active' and m.response = 'accepted'))
order by c.created_at desc, c.id;

-- name: GetCrewMembership :one
select c.city_id, c.status, c.activity_id, c.area_id, c.offer_until, m.response
from app.crew_members m
join app.crews c on c.id = m.crew_id
where m.crew_id = @crew_id and m.user_id = @user_id;

-- name: LockCrewMembership :one
select c.status, c.offer_until, m.response
from app.crew_members m
join app.crews c on c.id = m.crew_id
where m.crew_id = @crew_id and m.user_id = @user_id
for update of m, c;

-- First names of a crew's members: those who accepted, and (while it's an
-- offer) those still deciding.
-- name: ListCrewMembers :many
select m.user_id, pr.display_name, m.response
from app.crew_members m
join app.profiles pr on pr.user_id = m.user_id
where m.crew_id = $1 and m.response in ('offered', 'accepted')
order by pr.display_name, m.user_id;

-- name: SetCrewResponse :exec
update app.crew_members set response = @response, responded_at = @at
where crew_id = @crew_id and user_id = @user_id;

-- name: CountCrewMembers :one
select count(*)::int from app.crew_members where crew_id = @crew_id and response = @response;

-- name: InsertCrewLeave :exec
insert into app.crew_leaves (crew_id, user_id, reason, details, created_at)
values (@crew_id, @user_id, @reason, @details, @at);

-- name: InsertCrewMessage :one
insert into app.crew_messages (crew_id, user_id, body) values (@crew_id, @user_id, @body)
returning id, created_at;

-- Messages after a given one, oldest first, with the sender's first name.
-- name: ListCrewMessages :many
select c.id, c.user_id, pr.display_name, c.body, c.plan_id, c.created_at
from app.crew_messages c
left join app.profiles pr on pr.user_id = c.user_id
where c.crew_id = @crew_id and c.id > @after_id
order by c.id
limit @max;

-- name: ListLatestCrewMessages :many
select * from (
  select c.id, c.user_id, pr.display_name, c.body, c.plan_id, c.created_at
  from app.crew_messages c
  left join app.profiles pr on pr.user_id = c.user_id
  where c.crew_id = @crew_id
  order by c.id desc
  limit @max
) latest order by id;

-- Someone who is or was in a crew (accepted, or left since).
-- name: IsCrewMember :one
select exists (select 1 from app.crew_members where crew_id = @crew_id and user_id = @user_id and response in ('accepted', 'left'))::bool;

-- name: UpsertCrewReport :exec
insert into app.reports (reporter_id, reported_id, crew_id, reason, details, created_at, updated_at)
values (@reporter_id, @reported_id, @crew_id, @reason, @details, @at, @at)
on conflict (reporter_id, reported_id, crew_id) where crew_id is not null do update
set reason = excluded.reason, details = excluded.details, updated_at = excluded.updated_at;

-- name: GetAreaName :one
select name from app.areas where id = $1;

-- Whether a crew was ever offered to all of these people (whatever
-- happened to it): the same people aren't offered a crew twice.
-- name: CrewCoversMembers :one
select exists (
  select 1 from app.crews c
  where c.city_id = @city_id
    and (select count(*) from app.crew_members m where m.crew_id = c.id and m.user_id = any(@user_ids::uuid[]))
        = cardinality(@user_ids::uuid[])
)::bool;

-- Whether an offered or active crew on the same basis already has at least
-- two of these people: a few new faces don't make a new crew. (Inviting
-- them into the existing crew is for later.)
-- name: CrewOverlaps :one
select exists (
  select 1 from app.crews c
  where c.city_id = @city_id and c.status in ('offered', 'active')
    and c.activity_id is not distinct from @activity_id and c.area_id is not distinct from @area_id
    and (select count(*) from app.crew_members m
         where m.crew_id = c.id and m.response in ('offered', 'accepted') and m.user_id = any(@user_ids::uuid[])) >= 2
)::bool;

-- Crews a suspended person is in or offered (Service.RemoveFromUpcoming).
-- name: ListUserOpenCrewMemberships :many
select c.id
from app.crew_members m
join app.crews c on c.id = m.crew_id
where m.user_id = @user_id
  and ((m.response = 'accepted' and c.status in ('offered', 'active'))
       or (m.response = 'offered' and c.status = 'offered'))
order by c.id;
