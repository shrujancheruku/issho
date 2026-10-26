-- A profile with its languages, availability and activity preferences as
-- parallel arrays (each list's share an order): one round trip instead of
-- four (the database is a long way from the API). Languages: level 0 = none.
-- name: GetProfile :one
select p.user_id, p.display_name, p.birth_year, p.gender, p.same_gender_only, p.home_area_id,
       a.city_id, p.max_travel_min, p.energy, p.budget, p.size_pref, p.primary_language,
       p.secondary_language, p.conduct_version, p.conduct_accepted_at, p.updated_at,
       array(select l.lang from app.user_languages l where l.user_id = p.user_id order by l.fluent desc, l.lang)::text[] as lang_codes,
       array(select l.fluent from app.user_languages l where l.user_id = p.user_id order by l.fluent desc, l.lang)::bool[] as lang_fluent,
       array(select coalesce(l.level, 0) from app.user_languages l where l.user_id = p.user_id order by l.fluent desc, l.lang)::int2[] as lang_levels,
       array(select s.weekday from app.availability_slots s where s.user_id = p.user_id order by s.weekday, s.block)::int2[] as slot_weekdays,
       array(select s.block from app.availability_slots s where s.user_id = p.user_id order by s.weekday, s.block)::text[] as slot_blocks,
       array(select ap.activity_id from app.user_activity_prefs ap where ap.user_id = p.user_id order by ap.activity_id)::text[] as pref_activities,
       array(select ap.weight from app.user_activity_prefs ap where ap.user_id = p.user_id order by ap.activity_id)::int2[] as pref_weights
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

-- Replaces a person's languages, availability and activity preferences
-- with the given lists in one statement (one round trip; the app always
-- sends the full lists). Per list, rows no longer given are deleted and the
-- given ones upserted: the two touch different rows, which data-modifying
-- CTEs need (they all see the table as it was before the statement).
-- Each list's arrays are parallel; unnest in the select list zips them
-- (sqlc doesn't know multi-argument unnest in from). Levels use 0 for
-- "none" because sqlc can't pass arrays with nulls; it's turned back into
-- null here (fluent languages have no level).
-- name: ReplaceProfileLists :exec
with drop_langs as (
  delete from app.user_languages
  where user_id = @user_id and lang <> all(@langs::text[])
), put_langs as (
  insert into app.user_languages (user_id, lang, fluent, level)
  select @user_id, unnest(@langs::text[]), unnest(@fluent::boolean[]), nullif(unnest(@levels::smallint[]), 0)
  on conflict (user_id, lang) do update set fluent = excluded.fluent, level = excluded.level
), drop_slots as (
  delete from app.availability_slots
  where user_id = @user_id
    and (weekday, block) not in (select unnest(@weekdays::smallint[]), unnest(@blocks::text[]))
), put_slots as (
  insert into app.availability_slots (user_id, weekday, block)
  select @user_id, unnest(@weekdays::smallint[]), unnest(@blocks::text[])
  on conflict do nothing
), drop_prefs as (
  delete from app.user_activity_prefs
  where user_id = @user_id and activity_id <> all(@activity_ids::text[])
)
insert into app.user_activity_prefs (user_id, activity_id, weight)
select @user_id, unnest(@activity_ids::text[]), unnest(@weights::smallint[])
on conflict (user_id, activity_id) do update set weight = excluded.weight;

-- name: GetAreaCity :one
select city_id from app.areas where id = $1;

-- An area's city and which of the given activities that city offers
-- (enabled globally or by its override), so a profile can't hold
-- preferences for hidden activities. No row: no such area.
-- name: GetAreaEnabledActivities :one
select ar.city_id,
       array(select a.id
             from app.activities a
             left join app.city_activity_overrides o on o.activity_id = a.id and o.city_id = ar.city_id
             where a.id = any(@ids::text[]) and coalesce(o.enabled, true))::text[] as enabled
from app.areas ar
where ar.id = @area_id;
