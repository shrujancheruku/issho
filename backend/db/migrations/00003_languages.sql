-- +goose Up

-- Display languages. Languages are symmetric (no default language): everything
-- is shown in a primary language plus an optional secondary one, chosen by
-- internal/platform/i18n (ForApp outside events, ForEvent for events).
--   primary_language:   the user's main language; the app is shown in it, and
--                       it's the second language on events held in another one
--   secondary_language: optional extra language shown next to it
-- Any language is allowed (ISO 639-1 codes). Text in languages other than
-- English is translated on demand (internal/translate, app.translations).
-- Separate from app.user_languages, which records every language a person can
-- *speak* (used for matching).
-- not null is safe: profiles are only created from M5 onboarding, which sets it.
alter table app.profiles
  add column primary_language text not null check (primary_language ~ '^[a-z]{2,3}$'),
  add column secondary_language text check (secondary_language ~ '^[a-z]{2,3}$'),
  add constraint profiles_languages_distinct check (secondary_language <> primary_language);

alter table app.user_languages
  add constraint user_languages_code check (lang ~ '^[a-z]{2,3}$');

comment on column app.cities.languages is 'Languages commonly used in the city, most common first (suggestions only)';

-- +goose Down
comment on column app.cities.languages is null;
alter table app.user_languages drop constraint user_languages_code;
alter table app.profiles
  drop constraint profiles_languages_distinct,
  drop column secondary_language,
  drop column primary_language;
