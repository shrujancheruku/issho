-- +goose Up

-- The queue (docs/spec/running-issho.md, "The queue"): every by-hand step is an
-- item with a due time. `staff` owns this table (docs/spec/architecture.md,
-- "The seams"); other packages reach it only through staff's Go API.
--
-- The default each kind applies at its due time is Go code registered by the
-- package that opens the item, not data, so the table holds only the kind's name.
create table app.queue_items (
  id          bigint generated always as identity primary key,
  -- Names the step, e.g. "plan-review". Registered with staff by the package that opens it.
  kind        text not null check (kind <> ''),
  -- What the step is about, in the opening package's own terms (an id, a key).
  -- staff never reads it; the default function does.
  subject     text not null,
  due_at      timestamptz not null,
  -- open: waiting for a person. resolved: a person acted. defaulted: the due
  -- job applied the kind's written default.
  state       text not null default 'open' check (state in ('open', 'resolved', 'defaulted')),
  -- Why a person resolved it; the default's own reason is its kind.
  reason      text check (reason <> ''),
  created_at  timestamptz not null,
  closed_at   timestamptz,
  constraint closed_with_state check (
    (state = 'open' and closed_at is null and reason is null)
    or (state = 'resolved' and closed_at is not null and reason is not null)
    or (state = 'defaulted' and closed_at is not null and reason is null)
  )
);

-- The list of open items, soonest due first.
create index queue_items_open_due_idx on app.queue_items (due_at, id) where state = 'open';

-- +goose Down

drop table app.queue_items;
