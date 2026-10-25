-- The moderation queue (M8, ModerationService): reports, flags, crew leave
-- reasons and machine translations, for admins. Names are the English ones:
-- moderators read the admin SPA in English.

-- Reports, oldest open ones first (they've waited longest); resolved ones
-- newest first. @status '' = every status; @reported_id narrows to one
-- person. Each row says how many reports that person has in all, so a
-- pattern shows without opening each one.
-- name: ListReports :many
select r.id, r.reporter_id, r.reported_id, r.event_id, r.crew_id, r.reason, r.details,
       r.status, r.note, r.created_at, r.updated_at, r.reviewed_at, r.reviewed_by,
       coalesce(rp.display_name, '')::text as reporter_name,
       coalesce(dp.display_name, '')::text as reported_name,
       coalesce(pl.title->>'en', pa.name->>'en', '')::text as event_title,
       e.date as event_date,
       coalesce(e.block, '')::text as event_block,
       coalesce(ca.name->>'en', cr.name->>'en', '')::text as crew_name,
       (select count(*) from app.reports x where x.reported_id = r.reported_id)::int as reports_about_reported
from app.reports r
left join app.profiles rp on rp.user_id = r.reporter_id
left join app.profiles dp on dp.user_id = r.reported_id
left join app.events e on e.id = r.event_id
left join app.plans pl on pl.id = e.plan_id
left join app.activities pa on pa.id = pl.activity_id
left join app.crews c on c.id = r.crew_id
left join app.activities ca on ca.id = c.activity_id
left join app.areas cr on cr.id = c.area_id
where (@status::text = '' or r.status = @status)
  and (@reported_id::uuid = '00000000-0000-0000-0000-000000000000' or r.reported_id = @reported_id)
order by case when r.status = 'open' then r.created_at end asc,
         r.created_at desc
limit @max_rows;

-- updated_at stays the reporter's: a report changed after review shows it.
-- name: ResolveReport :execrows
update app.reports
set status = @status, note = @note, reviewed_by = @reviewed_by, reviewed_at = @at
where id = @id;

-- Flags: open ones oldest first, else resolved ones newest first.
-- name: ListModerationFlags :many
select f.id, f.user_id, f.kind, f.details, f.created_at, f.updated_at, f.resolved_at, f.resolved_by, f.note,
       coalesce(p.display_name, '')::text as user_name
from app.moderation_flags f
left join app.profiles p on p.user_id = f.user_id
where (@open::boolean and f.resolved_at is null) or (not @open::boolean and f.resolved_at is not null)
order by case when f.resolved_at is null then f.created_at end asc,
         f.resolved_at desc
limit @max_rows;

-- Resolving closes the flag; the next signal opens a new one.
-- name: ResolveModerationFlag :execrows
update app.moderation_flags
set resolved_at = @at, resolved_by = @resolved_by, note = @note, updated_at = @at
where id = @id and resolved_at is null;

-- Why people left crews, newest first. @reason '' = every reason.
-- name: ListCrewLeaves :many
select l.id, l.crew_id, l.user_id, l.reason, l.details, l.created_at,
       coalesce(p.display_name, '')::text as user_name,
       coalesce(ca.name->>'en', cr.name->>'en', '')::text as crew_name,
       c.status as crew_status
from app.crew_leaves l
join app.crews c on c.id = l.crew_id
left join app.profiles p on p.user_id = l.user_id
left join app.activities ca on ca.id = c.activity_id
left join app.areas cr on cr.id = c.area_id
where (@reason::text = '' or l.reason = @reason)
order by l.created_at desc, l.id desc
limit @max_rows;

-- Suspensions (00020). Everyone suspended now: the API's check
-- (moderation.Suspensions) keeps this set in memory.
-- name: ListSuspendedIDs :many
select user_id from app.suspensions where lifted_at is null;

-- Suspending someone already suspended changes nothing.
-- name: InsertSuspension :execrows
insert into app.suspensions (user_id, reason, created_by, created_at)
values (@user_id, @reason, @created_by, @at)
on conflict (user_id) where lifted_at is null do nothing;

-- name: LiftSuspension :execrows
update app.suspensions
set lifted_at = @at, lifted_by = @lifted_by, lift_note = @note
where user_id = @user_id and lifted_at is null;

-- One person's suspensions, newest first.
-- name: ListPersonSuspensions :many
select id, reason, created_by, created_at, lifted_at, lifted_by, lift_note
from app.suspensions
where user_id = $1
order by created_at desc;

-- Everyone suspended now, most recent first.
-- name: ListOpenSuspensions :many
select s.user_id, s.reason, s.created_at, s.created_by,
       coalesce(p.display_name, '')::text as display_name
from app.suspensions s
left join app.profiles p on p.user_id = s.user_id
where s.lifted_at is null
order by s.created_at desc
limit @max_rows;

-- One person, for the person view. No profile = not onboarded.
-- name: GetPerson :one
select u.id, coalesce(u.email, '')::text as email, u.created_at,
       coalesce(p.display_name, '')::text as display_name,
       coalesce(p.gender, '')::text as gender,
       coalesce(p.birth_year, 0)::int as birth_year,
       coalesce(a.name->>'en', '')::text as home_area,
       coalesce(a.city_id, '')::text as city_id,
       (select count(*) from app.reports r where r.reported_id = u.id)::int as reports_about,
       (select count(*) from app.reports r where r.reported_id = u.id and r.status = 'open')::int as open_reports_about,
       (select count(*) from app.reports r where r.reporter_id = u.id)::int as reports_made
from app.users u
left join app.profiles p on p.user_id = u.id
left join app.areas a on a.id = p.home_area_id
where u.id = $1;

-- People by name or email (contains, any case) or exact id, for finding
-- someone to look at. Suspended people say so.
-- name: SearchPeople :many
select u.id, coalesce(u.email, '')::text as email,
       coalesce(p.display_name, '')::text as display_name,
       exists (select 1 from app.suspensions s where s.user_id = u.id and s.lifted_at is null) as suspended
from app.users u
left join app.profiles p on p.user_id = u.id
where u.id::text = @query::text
   or p.display_name ilike '%' || @query || '%'
   or u.email ilike '%' || @query || '%'
order by p.display_name nulls last, u.id
limit @max_rows;

-- Their outings, newest first: how they answered, and whether they went
-- (checked in, or said so in their feedback).
-- name: ListPersonOutings :many
select e.id as event_id, e.date, e.block, e.status as event_status,
       coalesce(pl.title->>'en', pa.name->>'en', '')::text as title,
       m.response, (m.checked_in_at is not null)::boolean as checked_in,
       f.attended as said_attended
from app.group_members m
join app.events e on e.id = m.event_id
join app.plans pl on pl.id = e.plan_id
join app.activities pa on pa.id = pl.activity_id
left join app.event_feedback f on f.group_id = m.group_id and f.user_id = m.user_id
where m.user_id = $1
order by e.date desc, e.id
limit @max_rows;

-- Every flag about one person, open ones first.
-- name: ListPersonFlags :many
select id, kind, details, created_at, updated_at, resolved_at, resolved_by, note
from app.moderation_flags
where user_id = $1
order by resolved_at desc nulls first, created_at desc;
