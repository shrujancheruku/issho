-- name: ListCityActivities :many
select a.id, coalesce(o.name, a.name) as name, a.category, a.indoor, a.energy, a.cost_level,
       a.conversational, a.min_group_size, a.max_group_size
from app.activities a
left join app.city_activity_overrides o on o.activity_id = a.id and o.city_id = $1
where coalesce(o.enabled, true)
order by a.category, a.id;
