-- +goose Up

-- Machine translations made on demand. Our own content is stored in English
-- only (the one language that must always exist); any other language is
-- translated the first time someone needs it, stored here, and reused for
-- everyone. Text that already exists in a language (an event prefilled in
-- its own languages, a local area name such as 吉祥寺) is never translated.
--
-- Keyed by a hash of the English source plus its context, so editing the
-- English text naturally produces a fresh translation, and the same word in
-- two contexts (a "Bar" venue vs. a bar activity) can differ.
create table app.translations (
  source_hash text not null,                 -- hex sha256 of source + context
  lang        text not null check (lang ~ '^[a-z]{2,3}$'),
  source      text not null,                 -- English source, for review
  context     text not null default '',      -- what the text is, given to the translator
  text        text not null,
  model       text not null,                 -- which translator produced it
  reviewed    boolean not null default false, -- a person checked it (admin, M8)
  created_at  timestamptz not null default now(),
  primary key (source_hash, lang)
);

-- Supabase's default privileges grant new tables to anon/authenticated; schema
-- usage is already revoked (00001). Defence in depth, as in 00002.
-- +goose StatementBegin
do $$
begin
  if exists (select 1 from pg_roles where rolname = 'anon') then
    revoke all on app.translations from anon, authenticated;
  end if;
end $$;
-- +goose StatementEnd

-- +goose Down
drop table app.translations;
