-- name: UpsertCity :exec
insert into app.cities (id, name, timezone, currency, languages, travel_modes)
values ($1, $2, $3, $4, $5, $6)
on conflict (id) do update set
  name = excluded.name, timezone = excluded.timezone,
  currency = excluded.currency, languages = excluded.languages,
  travel_modes = excluded.travel_modes;

-- name: UpsertArea :exec
insert into app.areas (id, city_id, name, centroid)
values (@id, @city_id, @name, st_setsrid(st_makepoint(@lng::float8, @lat::float8), 4326)::geography)
on conflict (id) do update set
  city_id = excluded.city_id, name = excluded.name, centroid = excluded.centroid;

-- name: UpsertActivity :exec
insert into app.activities (id, name, category, indoor, energy, cost_level, conversational,
                            skill_dependent, min_group_size, max_group_size)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
on conflict (id) do update set
  name = excluded.name, category = excluded.category, indoor = excluded.indoor,
  energy = excluded.energy, cost_level = excluded.cost_level,
  conversational = excluded.conversational, skill_dependent = excluded.skill_dependent,
  min_group_size = excluded.min_group_size, max_group_size = excluded.max_group_size;

-- name: UpsertCityActivityOverride :exec
insert into app.city_activity_overrides (city_id, activity_id, enabled, name, months)
values ($1, $2, $3, $4, $5)
on conflict (city_id, activity_id) do update set
  enabled = excluded.enabled, name = excluded.name, months = excluded.months;

-- name: UpsertVenueCategory :exec
insert into app.venue_categories (id, name, google_types, search_radius_m)
values ($1, $2, $3, $4)
on conflict (id) do update set
  name = excluded.name, google_types = excluded.google_types,
  search_radius_m = excluded.search_radius_m;

-- name: SetActivityVenueCategories :exec
update app.activities set venue_categories = @venue_categories where id = @id;
