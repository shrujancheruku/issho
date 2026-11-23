-- +goose Up

-- A profile is the answers asked once, at a person's first signup, not before
-- (docs/spec/people.md, "The profile"). `people` owns this table; other
-- packages reach it only through people's Go API.
--
-- Its personal data goes with the account, so the key is a foreign key with
-- on delete cascade: deleting an account (milestone 6) leaves no profile.
create table app.profiles (
  account_id uuid primary key references app.accounts (id) on delete cascade,
  -- The name they go by. Its length (1 to 40 characters) is policy's, checked
  -- by the API; the database refuses only an empty one.
  name       text not null check (btrim(name) <> ''),
  gender     text not null check (gender in ('woman', 'man', 'non-binary', 'prefer-not-to-say')),
  -- Language codes ("en", "vi", "ja"), at least one; any language is allowed,
  -- so the list is not a foreign key.
  languages  text[] not null check (cardinality(languages) >= 1),
  -- Whether they say they live in the account's city: self-declared, and
  -- either answer is accepted.
  lives_in_city boolean not null,
  -- "Only women and non-binary people" (docs/spec/people.md): only for those
  -- who answered woman or non-binary.
  only_women_and_non_binary boolean not null default false
    check (not only_women_and_non_binary or gender in ('woman', 'non-binary')),
  -- The date of birth is never stored: Issho keeps only that the person said
  -- they were 18 or over, and the day they said it, on the city's clock.
  said_adult_on date not null,
  -- The code of conduct accepted, by version, and when.
  code_of_conduct_version     integer     not null check (code_of_conduct_version > 0),
  code_of_conduct_accepted_at timestamptz not null,
  created_at timestamptz not null,
  updated_at timestamptz not null
);

-- +goose Down

drop table app.profiles;
