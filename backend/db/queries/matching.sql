-- What internal/matching needs about one city, loaded by internal/plans for
-- the worker. Everything is per city: a person belongs to the city of their
-- home area.

-- name: GetCityMatching :one
select timezone, matching_weights from app.cities where id = $1;

-- name: ListCityAreaIDs :many
select id from app.areas where city_id = $1 order by id;

-- The fastest of the city's travel modes for each pair (matching.TravelTimes):
-- someone who'd walk 20 minutes won't drive 25.
-- name: ListCityTravelTimes :many
select t.from_area_id, t.to_area_id, min(t.minutes)::int as minutes
from app.area_travel_times t
join app.areas a on a.id = t.from_area_id
join app.cities c on c.id = a.city_id
where a.city_id = $1 and t.mode = any(c.travel_modes)
group by t.from_area_id, t.to_area_id;

-- Activities enabled in the city and in season in the given month (0 = any).
-- name: ListMatchingActivities :many
select a.id, a.category, a.cost_level, a.conversational, a.min_group_size, a.max_group_size,
       a.venue_categories, a.blocks, a.walk_theme
from app.activities a
left join app.city_activity_overrides o on o.activity_id = a.id and o.city_id = @city_id
where coalesce(o.enabled, true)
  and (o.months is null or @month::smallint = 0 or @month::smallint = any(o.months))
order by a.id;

-- Venues an admin hasn't rejected, counted per area and category.
-- name: ListCityVenueCounts :many
select area_id, category_id, count(*)::int as venues
from app.venues
where city_id = $1 and status <> 'rejected'
group by area_id, category_id;

-- name: ListMatchingUsers :many
-- Reliability is how often someone turned up when they'd confirmed: outings
-- attended over confirmed places at finished events, with a prior of two
-- outings attended so a newcomer starts at 1.0 and one miss doesn't sink
-- them (one no-show in their first outing gives 0.67). Cancelling in
-- advance (dropped) isn't a miss. Attended = checked in, or said so in
-- their feedback afterwards (people forget to check in).
select p.user_id, p.home_area_id, p.max_travel_min, p.gender, p.same_gender_only, p.size_pref,
       p.budget,
       ((2 + (select count(*) from app.group_members m join app.events e on e.id = m.event_id
              where m.user_id = p.user_id and e.status = 'done' and m.response = 'confirmed'
                and (m.checked_in_at is not null or exists (
                  select 1 from app.event_feedback f where f.group_id = m.group_id and f.user_id = m.user_id and f.attended))))::real
        / (2 + (select count(*) from app.group_members m join app.events e on e.id = m.event_id
                where m.user_id = p.user_id and e.status = 'done' and m.response = 'confirmed'))::real)::real as reliability,
       (select count(*) from app.group_members m
        where m.user_id = p.user_id
          and (m.checked_in_at is not null or exists (
            select 1 from app.event_feedback f where f.group_id = m.group_id and f.user_id = m.user_id and f.attended)))::int as events_attended
from app.profiles p
join app.areas a on a.id = p.home_area_id
join app.users u on u.id = p.user_id
where a.city_id = $1 and u.deleted_at is null
order by p.user_id;

-- name: ListMatchingLanguages :many
select l.user_id, l.lang, l.fluent, coalesce(l.level, 0)::smallint as level
from app.user_languages l
join app.profiles p on p.user_id = l.user_id
join app.areas a on a.id = p.home_area_id
where a.city_id = $1
order by l.user_id, l.lang;

-- name: ListMatchingAvailability :many
select s.user_id, s.weekday, s.block
from app.availability_slots s
join app.profiles p on p.user_id = s.user_id
join app.areas a on a.id = p.home_area_id
where a.city_id = $1
order by s.user_id, s.weekday, s.block;

-- name: ListMatchingPrefs :many
select f.user_id, f.activity_id, f.weight
from app.user_activity_prefs f
join app.profiles p on p.user_id = f.user_id
join app.areas a on a.id = p.home_area_id
where a.city_id = $1
order by f.user_id, f.activity_id;

-- Plans that closed without a group recently: not suggested again until the
-- cool-down passes, or the same plan would reopen the next morning.
-- name: ListRecentlyFailedPlans :many
select area_id, activity_id, follow_up_id, primary_language, secondary_language
from app.plans
where city_id = @city_id and closed_reason = 'no_group' and closed_at >= @since;

-- One person and their city, for their feed.
-- name: GetMatchingUser :one
select p.user_id, a.city_id, p.home_area_id, p.max_travel_min, p.gender, p.same_gender_only, p.size_pref,
       p.budget, p.reliability, p.primary_language, p.secondary_language
from app.profiles p
join app.areas a on a.id = p.home_area_id
where p.user_id = $1;
