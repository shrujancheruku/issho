-- Domain metrics for the admin SPA (M8), per city and week. Weeks start on
-- Monday in the city's own time zone; an outing counts in the week of its
-- (local) date. Attended means what matching's reliability means: confirmed
-- and checked in, or said so in their feedback.

-- name: WeeklyMetrics :many
with city as (
  select timezone from app.cities where id = @city_id
), weeks as (
  -- Mondays from the first week to the last (worked out in the city's
  -- time zone by the caller).
  select generate_series(@first::date, @last::date, interval '1 week')::date as week
), members as (
  -- Everyone invited to a finished outing that went ahead, and whether
  -- they went.
  select m.user_id, m.group_id, m.response, e.date,
         (m.response = 'confirmed' and (m.checked_in_at is not null or exists (
           select 1 from app.event_feedback f where f.group_id = m.group_id and f.user_id = m.user_id and f.attended))) as attended
  from app.group_members m
  join app.event_groups g on g.id = m.group_id
  join app.events e on e.id = m.event_id
  join app.plans p on p.id = e.plan_id
  where p.city_id = @city_id and e.status = 'done' and g.status <> 'cancelled'
), went as (
  select user_id, date from members where attended
)
select w.week,
       (select count(distinct a.user_id) from went a where a.date >= w.week and a.date < w.week + 7)::int as attendees,
       -- The north star: this week's attendees with at least two outings
       -- so far (this one included).
       (select count(distinct a.user_id) from went a
        where a.date >= w.week and a.date < w.week + 7
          and (select count(*) from went b where b.user_id = a.user_id and b.date < w.week + 7) >= 2)::int as repeat_attendees,
       (select count(distinct a.user_id) from went a
        where a.date >= w.week and a.date < w.week + 7
          and not exists (select 1 from went b where b.user_id = a.user_id and b.date < w.week))::int as first_timers,
       (select count(distinct m.group_id) from members m where m.date >= w.week and m.date < w.week + 7)::int as groups,
       (select count(*) from members m where m.date >= w.week and m.date < w.week + 7)::int as invited,
       (select count(*) from members m where m.date >= w.week and m.date < w.week + 7 and m.response = 'confirmed')::int as confirmed,
       (select count(*) from members m where m.date >= w.week and m.date < w.week + 7 and m.attended)::int as attended,
       coalesce((select avg(f.plan_rating) from app.event_feedback f join members m on m.group_id = f.group_id and m.user_id = f.user_id
        where m.date >= w.week and m.date < w.week + 7), 0)::float8 as plan_rating,
       coalesce((select avg(f.group_rating) from app.event_feedback f join members m on m.group_id = f.group_id and m.user_id = f.user_id
        where m.date >= w.week and m.date < w.week + 7), 0)::float8 as group_rating,
       (select count(*) from app.mate_picks k join members m on m.group_id = k.group_id and m.user_id = k.picker_id
        where m.date >= w.week and m.date < w.week + 7 and k.pick = 'again')::int as picks_again,
       (select count(*) from app.mate_picks k join members m on m.group_id = k.group_id and m.user_id = k.picker_id
        where m.date >= w.week and m.date < w.week + 7)::int as picks,
       -- Pairs who both said they'd meet again: counted once.
       (select count(*) from app.mate_picks k join members m on m.group_id = k.group_id and m.user_id = k.picker_id
        where m.date >= w.week and m.date < w.week + 7 and k.pick = 'again' and k.picker_id < k.picked_id
          and exists (select 1 from app.mate_picks r where r.group_id = k.group_id and r.picker_id = k.picked_id
                      and r.picked_id = k.picker_id and r.pick = 'again'))::int as mutual_pairs,
       (select count(*) from app.users u join app.profiles pr on pr.user_id = u.id join app.areas a on a.id = pr.home_area_id
        where a.city_id = @city_id and (u.created_at at time zone city.timezone)::date >= w.week
          and (u.created_at at time zone city.timezone)::date < w.week + 7)::int as new_people,
       (select count(*) from app.crews cr where cr.city_id = @city_id and cr.formed_at is not null
          and (cr.formed_at at time zone city.timezone)::date >= w.week
          and (cr.formed_at at time zone city.timezone)::date < w.week + 7)::int as crews_formed,
       (select count(*) from app.reports r join app.events e on e.id = r.event_id join app.plans p on p.id = e.plan_id
        where p.city_id = @city_id and (r.created_at at time zone city.timezone)::date >= w.week
          and (r.created_at at time zone city.timezone)::date < w.week + 7)::int as reports
from weeks w, city
order by w.week;

-- The city now.
-- name: CityNow :one
select
  (select count(*) from app.profiles pr join app.areas a on a.id = pr.home_area_id join app.users u on u.id = pr.user_id
   where a.city_id = @city_id and u.deleted_at is null
     and not exists (select 1 from app.suspensions s where s.user_id = pr.user_id and s.lifted_at is null))::int as people,
  (select count(*) from app.plans p where p.city_id = @city_id and p.status = 'open')::int as open_plans,
  (select count(*) from app.plan_signups s join app.plans p on p.id = s.plan_id where p.city_id = @city_id and p.status = 'open')::int as waiting,
  (select count(*) from app.events e join app.plans p on p.id = e.plan_id
   where p.city_id = @city_id and e.status = 'scheduled')::int as upcoming_outings,
  (select count(*) from app.crews c where c.city_id = @city_id and c.status = 'active')::int as active_crews;
