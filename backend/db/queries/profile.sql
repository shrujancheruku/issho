-- name: GetProfile :one
select p.user_id, p.display_name, p.birth_year, p.gender, p.same_gender_only, p.home_area_id,
       a.city_id, p.max_travel_min, p.energy, p.budget, p.size_pref, p.primary_language,
       p.secondary_language, p.conduct_version, p.conduct_accepted_at, p.updated_at
from app.profiles p
join app.areas a on a.id = p.home_area_id
where p.user_id = $1;

-- conduct_accepted_at only moves when a new version is accepted, so it
-- records when the person agreed to the text they're bound by.
-- name: UpsertProfile :exec
insert into app.profiles (
  user_id, display_name, birth_year, gender, same_gender_only, home_area_id, max_travel_min,
  energy, budget, size_pref, primary_language, secondary_language, conduct_version
) values (
  @user_id, @display_name, @birth_year, @gender, @same_gender_only, @home_area_id, @max_travel_min,
  @energy, @budget, @size_pref, @primary_language, @secondary_language, @conduct_version
)
on conflict (user_id) do update set
  display_name = excluded.display_name,
  birth_year = excluded.birth_year,
  gender = excluded.gender,
  same_gender_only = excluded.same_gender_only,
  home_area_id = excluded.home_area_id,
  max_travel_min = excluded.max_travel_min,
  energy = excluded.energy,
  budget = excluded.budget,
  size_pref = excluded.size_pref,
  primary_language = excluded.primary_language,
  secondary_language = excluded.secondary_language,
  conduct_accepted_at = case when excluded.conduct_version <> app.profiles.conduct_version
                             then now() else app.profiles.conduct_accepted_at end,
  conduct_version = excluded.conduct_version,
  updated_at = now();

-- name: ListUserLanguages :many
select lang, fluent, level from app.user_languages where user_id = $1 order by fluent desc, lang;

-- name: DeleteUserLanguages :exec
delete from app.user_languages where user_id = $1;

-- The arrays are parallel (one element per language); unnest in the select
-- list zips them. sqlc doesn't know multi-argument unnest in from.
-- Levels use 0 for "none" because sqlc can't pass arrays with nulls; it's
-- turned back into null here (fluent languages have no level).
-- name: InsertUserLanguages :exec
insert into app.user_languages (user_id, lang, fluent, level)
select @user_id, unnest(@langs::text[]), unnest(@fluent::boolean[]), nullif(unnest(@levels::smallint[]), 0);

-- name: ListAvailability :many
select weekday, block from app.availability_slots where user_id = $1 order by weekday, block;

-- name: DeleteAvailability :exec
delete from app.availability_slots where user_id = $1;

-- name: InsertAvailability :exec
insert into app.availability_slots (user_id, weekday, block)
select @user_id, unnest(@weekdays::smallint[]), unnest(@blocks::text[]);

-- name: ListActivityPrefs :many
select activity_id, weight from app.user_activity_prefs where user_id = $1 order by activity_id;

-- name: DeleteActivityPrefs :exec
delete from app.user_activity_prefs where user_id = $1;

-- name: InsertActivityPrefs :exec
insert into app.user_activity_prefs (user_id, activity_id, weight)
select @user_id, unnest(@activity_ids::text[]), unnest(@weights::smallint[]);

-- name: GetAreaCity :one
select city_id from app.areas where id = $1;

-- Which of the given activities a city offers (enabled globally or by its
-- override), so a profile can't hold preferences for hidden activities.
-- name: ListEnabledActivityIDs :many
select a.id
from app.activities a
left join app.city_activity_overrides o on o.activity_id = a.id and o.city_id = @city_id
where a.id = any(@ids::text[]) and coalesce(o.enabled, true);
