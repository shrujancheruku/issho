-- name: GetTranslations :many
select source_hash, text from app.translations
where lang = @lang and source_hash = any(@source_hashes::text[]);

-- Concurrent requests may translate the same text; the first one wins and a
-- reviewed translation is never overwritten.
-- name: InsertTranslation :exec
insert into app.translations (source_hash, lang, source, context, text, model)
values ($1, $2, $3, $4, $5, $6)
on conflict (source_hash, lang) do nothing;
