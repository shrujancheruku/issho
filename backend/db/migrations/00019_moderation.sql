-- +goose Up

-- The moderation queue (M8): who reviewed what, and why. Admins are
-- Supabase users with app_metadata.admin (internal/platform/auth); the
-- *_by columns hold their user id without a foreign key, because an admin
-- needn't have an app account, and the record should outlive theirs.

alter table app.reports
  add column reviewed_by uuid,
  -- The moderator's note on the decision: what they did and why.
  add column note text not null default '' check (char_length(note) <= 2000);

alter table app.moderation_flags
  add column resolved_by uuid,
  add column note text not null default '' check (char_length(note) <= 2000);
-- The queue lists open flags oldest first.
create index moderation_flags_queue_idx on app.moderation_flags(created_at) where resolved_at is null;

-- A corrected machine translation: reviewed rows are never overwritten
-- (InsertTranslation does nothing on conflict).
alter table app.translations
  add column reviewed_by uuid,
  add column reviewed_at timestamptz;

-- The newest crew leaves first.
create index crew_leaves_created_idx on app.crew_leaves(created_at desc);

-- +goose Down
drop index app.crew_leaves_created_idx;
alter table app.translations drop column reviewed_at, drop column reviewed_by;
drop index app.moderation_flags_queue_idx;
alter table app.moderation_flags drop column note, drop column resolved_by;
alter table app.reports drop column note, drop column reviewed_by;
