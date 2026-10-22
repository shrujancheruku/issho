-- +goose Up

-- Expo's tickets for pushes it accepted, kept until their receipt is checked
-- (about 15 minutes later) so a phone whose app is gone stops getting
-- pushes even when Expo only finds out after accepting one. Deleted with
-- the device token, and after a day (Expo keeps receipts for 24 hours).
create table app.push_tickets (
  id         text primary key,
  token      text not null references app.push_tokens(token) on delete cascade,
  created_at timestamptz not null default now()
);
create index push_tickets_created_idx on app.push_tickets(created_at);
create index push_tickets_token_idx on app.push_tickets(token);

-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on app.push_tickets from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
drop table app.push_tickets;
