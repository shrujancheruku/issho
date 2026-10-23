-- +goose Up

-- Suspending an account (M8, ModerationService.SuspendPerson): a moderator
-- keeps someone out of Issho. While a suspension is open the API refuses
-- their calls (except to see that they're suspended, sign out and delete
-- their account), the chat socket refuses them, and matching leaves them
-- out. Suspending also takes them out of what they'd joined: open
-- signups, upcoming outings and crews. Lifting it doesn't put them back.
--
-- Kept as history: each suspension and its lifting stay on record for the
-- next moderator. *_by are admin user ids without a foreign key, as in
-- 00019.
create table app.suspensions (
  id          bigint generated always as identity primary key,
  user_id     uuid not null references app.users(id) on delete cascade,
  -- Why, for moderators only; the person isn't shown it.
  reason      text not null check (char_length(reason) between 1 and 2000),
  created_by  uuid not null,
  created_at  timestamptz not null default now(),
  lifted_at   timestamptz,
  lifted_by   uuid,
  lift_note   text not null default '' check (char_length(lift_note) <= 2000),
  check ((lifted_at is null) = (lifted_by is null))
);
-- At most one open suspension per person; the API's check reads them all.
create unique index suspensions_open_idx on app.suspensions(user_id) where lifted_at is null;
create index suspensions_user_idx on app.suspensions(user_id, created_at desc);

-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on app.suspensions from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
drop table app.suspensions;
