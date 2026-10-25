-- +goose Up
-- Display languages can be a script variant where one spoken language is
-- written two ways: Traditional Chinese (zh-Hant) beside zh, which stays
-- Simplified. Only where text is shown: the translation cache and the
-- profile's display languages. Spoken languages (user_languages) and plan
-- languages stay ISO 639-1: matching treats Chinese as one language.
alter table app.translations drop constraint translations_lang_check,
  add constraint translations_lang_check check (lang ~ '^[a-z]{2,3}(-Hant)?$');
alter table app.profiles drop constraint profiles_primary_language_check,
  add constraint profiles_primary_language_check check (primary_language ~ '^[a-z]{2,3}(-Hant)?$');
alter table app.profiles drop constraint profiles_secondary_language_check,
  add constraint profiles_secondary_language_check check (secondary_language ~ '^[a-z]{2,3}(-Hant)?$');

-- +goose Down
update app.profiles set primary_language = 'zh' where primary_language = 'zh-Hant';
update app.profiles set secondary_language = null where secondary_language = 'zh-Hant';
delete from app.translations where lang = 'zh-Hant';
alter table app.translations drop constraint translations_lang_check,
  add constraint translations_lang_check check (lang ~ '^[a-z]{2,3}$');
alter table app.profiles drop constraint profiles_primary_language_check,
  add constraint profiles_primary_language_check check (primary_language ~ '^[a-z]{2,3}$');
alter table app.profiles drop constraint profiles_secondary_language_check,
  add constraint profiles_secondary_language_check check (secondary_language ~ '^[a-z]{2,3}$');
