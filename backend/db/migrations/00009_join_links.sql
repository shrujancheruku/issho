-- +goose Up

-- Join links and admission (docs/spec/people.md, "Join links"; docs/spec/launch.md,
-- "The Pilot"). `people` owns both tables; other packages ask its Go API.

-- A member's one-use link. Its code is the secret in the URL the member shares,
-- 128 random bits, so it can't be guessed; it is kept as it is so the member can
-- see their link again to send it. There is no expiry column: a link expires
-- `policy.JoinLinkLife` after created_at, which the API reads on its clock, so
-- one number lives in one place. A link is outstanding while neither of the two
-- end columns is set and it hasn't expired; an expired or cancelled link
-- returns to the member's 3, and a used one doesn't.
create table app.join_links (
  id           uuid primary key default gen_random_uuid(),
  member_id    uuid not null references app.accounts (id) on delete cascade,
  code         text not null check (code <> ''),
  created_at   timestamptz not null,
  cancelled_at timestamptz,
  redeemed_at  timestamptz,
  constraint join_links_code_key unique (code),
  -- A link ends one way.
  constraint join_links_ends_once check (cancelled_at is null or redeemed_at is null)
);

create index join_links_member_id_idx on app.join_links (member_id);

-- Admission: a fact about an account. Having a row is being admitted (a member
-- in the spec's terms; standing arrives with its own ticket). It records who
-- brought the account in, which only a moderator sees (docs/spec/safety.md,
-- "Join links"): no RPC returns it. The Pilot's first members are admitted by
-- the seed command, with no link and nobody who brought them.
create table app.admissions (
  account_id  uuid primary key references app.accounts (id) on delete cascade,
  admitted_at timestamptz not null,
  via         text not null check (via in ('join-link', 'seed')),
  -- Kept (set to null) if the member who brought them is deleted: the chain
  -- is for the moderator, and the admitted person's own account goes on.
  brought_by  uuid references app.accounts (id) on delete set null,
  -- The link that admitted them. A link admits one account, so it is unique.
  link_id     uuid references app.join_links (id) on delete set null,
  constraint admissions_link_id_key unique (link_id)
);

create index admissions_brought_by_idx on app.admissions (brought_by);

-- +goose Down

drop table app.admissions;
drop table app.join_links;
