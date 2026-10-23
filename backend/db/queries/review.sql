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
       p.venue_place_id, p.follow_up_venue_place_id, p.meeting_place_id, p.route_polyline, p.route_seconds
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
