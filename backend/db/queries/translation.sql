-- The translation editor's catalogue text (TranslationService): machine
-- translations other than the app's UI strings, which are edited as human
-- versions in the repository instead. @ui_context is uistrings.ContextPrefix.

-- Languages with catalogue translations, and how many are still unreviewed.
-- name: ListCatalogLanguages :many
select lang, count(*)::int as total, (count(*) filter (where not reviewed))::int as unreviewed
from app.translations
where not starts_with(context, @ui_context::text)
group by lang
order by lang;

-- One language's catalogue translations, unreviewed ones first, matching
-- @search in the English source or the translation ('' = all).
-- name: ListCatalogTranslations :many
select source_hash, lang, source, context, text, model, reviewed, reviewed_at, created_at
from app.translations
where lang = @lang
  and not starts_with(context, @ui_context::text)
  and (@unreviewed_only::boolean = false or not reviewed)
  and (@search::text = '' or source ilike '%' || @search || '%' or text ilike '%' || @search || '%')
order by reviewed, source, context
limit @max_rows offset @skip_rows;

-- Languages with any machine translations, UI strings included (for the
-- admins' list of languages).
-- name: ListTranslationLanguages :many
select lang, count(*)::int as total, (count(*) filter (where not reviewed))::int as unreviewed
from app.translations
group by lang
order by lang;

-- name: GetTranslation :one
select source_hash, lang, source, context, text, model, reviewed, reviewed_at, created_at
from app.translations
where source_hash = @source_hash and lang = @lang;

-- A person corrected (or confirmed) it: it's kept from then on.
-- name: ReviewTranslation :execrows
update app.translations
set text = @text, reviewed = true, reviewed_by = @reviewed_by, reviewed_at = @at
where source_hash = @source_hash and lang = @lang;
