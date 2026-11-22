-- The profiles table, app.profiles; only internal/people calls these.

-- The first profile fixes said_adult_on, the day the person said they were 18
-- or over; later saves change the answers and leave it (and created_at) alone.
-- name: UpsertProfile :one
insert into app.profiles (
  account_id, name, gender, languages, lives_in_city, only_women_and_non_binary,
  said_adult_on, code_of_conduct_version, code_of_conduct_accepted_at, created_at, updated_at
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
on conflict (account_id) do update set
  name = excluded.name,
  gender = excluded.gender,
  languages = excluded.languages,
  lives_in_city = excluded.lives_in_city,
  only_women_and_non_binary = excluded.only_women_and_non_binary,
  code_of_conduct_version = excluded.code_of_conduct_version,
  code_of_conduct_accepted_at = case
    when app.profiles.code_of_conduct_version = excluded.code_of_conduct_version
    then app.profiles.code_of_conduct_accepted_at
    else excluded.code_of_conduct_accepted_at end,
  updated_at = excluded.updated_at
returning *;

-- name: GetProfile :one
select * from app.profiles where account_id = $1;
