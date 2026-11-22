-- +goose Up

-- An account is an email, a city and a display language, nothing more
-- (docs/spec/people.md, "Sign-in and the account"). `people` owns this table;
-- other packages reach it only through people's Go API.
--
-- The profile, standing and join links are later tickets' own tables, keyed by
-- this id, so none of them widens this one.
create table app.accounts (
  -- Supabase Auth's user id, the token's `sub`. It is not a foreign key to
  -- auth.users: that schema doesn't exist on the plain Postgres the tests run
  -- on, and the API never reads Auth's tables.
  id               uuid primary key,
  -- Lowercased by the API; the unique index is what makes it one account per
  -- email even if two first calls race.
  email            text not null check (email <> '' and email = lower(email)),
  -- The city's slug. Not yet a foreign key: the cities table arrives with the
  -- fixture city (#610), which adds the constraint.
  city             text not null check (city <> ''),
  -- A language of the open cities ("en", "vi"); the API checks the list, so a
  -- city that opens with a new language is no migration.
  display_language text not null check (display_language <> ''),
  created_at       timestamptz not null,
  constraint accounts_email_key unique (email)
);

-- +goose Down

drop table app.accounts;
