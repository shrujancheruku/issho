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

-- Languages with machine translations, and how many are still unreviewed.
-- name: ListTranslationLanguages :many
select lang, count(*)::int as total, (count(*) filter (where not reviewed))::int as unreviewed
from app.translations
group by lang
order by lang;

-- One language's translations, unreviewed ones first, matching @search in
-- the English source or the translation ('' = all).
-- name: ListTranslations :many
select source_hash, lang, source, context, text, model, reviewed, reviewed_at, created_at
from app.translations
where lang = @lang
  and (@unreviewed_only::boolean = false or not reviewed)
  and (@search::text = '' or source ilike '%' || @search || '%' or text ilike '%' || @search || '%')
order by reviewed, source, context
limit @max_rows offset @skip_rows;

-- name: GetTranslation :one
select source_hash, lang, source, context, text, model, reviewed, reviewed_at, created_at
from app.translations
where source_hash = @source_hash and lang = @lang;

-- A person corrected (or confirmed) it: it's kept from then on.
-- name: ReviewTranslation :execrows
update app.translations
set text = @text, reviewed = true, reviewed_by = @reviewed_by, reviewed_at = @at
where source_hash = @source_hash and lang = @lang;
