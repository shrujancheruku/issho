-- The cities tables, app.cities and app.city_names; only internal/cities calls these.

-- name: GetCity :one
select * from app.cities where slug = $1;

-- name: GetCityNames :many
select language, name from app.city_names where city = $1 order by language;

-- Idempotent on the slug, so loading the fixture again refreshes the city.
-- name: UpsertCity :exec
insert into app.cities (slug, state, time_zone, currency)
values ($1, $2, $3, $4)
on conflict (slug) do update
  set state = excluded.state, time_zone = excluded.time_zone, currency = excluded.currency;

-- name: UpsertCityName :exec
insert into app.city_names (city, language, name)
values ($1, $2, $3)
on conflict (city, language) do update set name = excluded.name;
