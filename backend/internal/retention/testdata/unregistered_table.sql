-- A table that holds personal data and has no entry in the registry. The test
-- runs this after the migrations to prove the retention check fails on it.
create table app.fixture_people (
  id    bigint generated always as identity primary key,
  email text not null
);
