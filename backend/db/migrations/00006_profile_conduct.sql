-- +goose Up

-- Onboarding (M5) ends with accepting the code of conduct. The version is
-- stored so a future revision can ask everyone to accept it again; the text
-- itself is UI copy in the app. not null is safe: profiles are only created
-- by M5 onboarding, which always sets it.
alter table app.profiles
  add column conduct_version smallint not null check (conduct_version > 0),
  add column conduct_accepted_at timestamptz not null default now();

-- A level (1 beginner … 3 advanced) only makes sense for a language someone
-- is learning, and a learner always has one.
alter table app.user_languages
  add constraint user_languages_level check ((fluent and level is null) or (not fluent and level is not null));

-- +goose Down
alter table app.user_languages drop constraint user_languages_level;
alter table app.profiles
  drop column conduct_accepted_at,
  drop column conduct_version;
