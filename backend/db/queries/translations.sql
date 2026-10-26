-- name: GetTranslations :many
select source_hash, text from app.translations
where lang = @lang and source_hash = any(@source_hashes::text[]);

-- Concurrent requests may translate the same text; the first one wins and a
-- reviewed translation is never overwritten.
-- A batch from one translator call is one statement: one round trip
-- instead of one per text (the database is a long way from the API).
-- name: InsertTranslations :exec
insert into app.translations (source_hash, lang, source, context, text, model)
-- Several unnests in one select list advance together (same lengths).
select unnest(@source_hashes::text[]), @lang::text, unnest(@sources::text[]),
       unnest(@contexts::text[]), unnest(@texts::text[]), @model::text
on conflict (source_hash, lang) do nothing;
