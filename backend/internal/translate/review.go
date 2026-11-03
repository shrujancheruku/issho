package translate

// Review of catalogue translations: activity and city names have no file in
// the repository, so their machine translations are corrected in
// app.translations, which this package owns. translations.Handler checks the
// caller and the language, and maps protos.

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

// ReviewStore is the catalogue translations' part of the database
// (*dbgen.Queries).
type ReviewStore interface {
	ListTranslationLanguages(ctx context.Context) ([]dbgen.ListTranslationLanguagesRow, error)
	ListCatalogLanguages(ctx context.Context, uiContext string) ([]dbgen.ListCatalogLanguagesRow, error)
	ListCatalogTranslations(ctx context.Context, arg dbgen.ListCatalogTranslationsParams) ([]dbgen.ListCatalogTranslationsRow, error)
	GetTranslation(ctx context.Context, arg dbgen.GetTranslationParams) (dbgen.GetTranslationRow, error)
	ReviewTranslation(ctx context.Context, arg dbgen.ReviewTranslationParams) (int64, error)
}

// Reviewer lists and corrects the cached translations of catalogue text.
// UI strings are not its business (see uistrings.Editor).
type Reviewer struct {
	Q ReviewStore
	// UIContextPrefix starts the context of every UI string in the cache
	// (uistrings.ContextPrefix); SamePlaceholders is uistrings.SamePlaceholders.
	// Both are injected because uistrings imports this package.
	UIContextPrefix  string
	SamePlaceholders func(english, translated string) bool
	Now              func() time.Time // nil = time.Now
}

func (r *Reviewer) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// CachedLanguages are the languages anything is translated into.
func (r *Reviewer) CachedLanguages(ctx context.Context) ([]dbgen.ListTranslationLanguagesRow, error) {
	return r.Q.ListTranslationLanguages(ctx)
}

// CatalogLanguages are the languages with catalogue text translated, with
// how much of it nobody has reviewed.
func (r *Reviewer) CatalogLanguages(ctx context.Context) ([]dbgen.ListCatalogLanguagesRow, error) {
	return r.Q.ListCatalogLanguages(ctx, r.UIContextPrefix)
}

// CatalogTranslations is one page of a language's catalogue translations
// (arg.MaxRows is the caller's page size plus one). The UI context is set
// here, so a caller can't list UI strings.
func (r *Reviewer) CatalogTranslations(ctx context.Context, arg dbgen.ListCatalogTranslationsParams) ([]dbgen.ListCatalogTranslationsRow, error) {
	arg.UiContext = r.UIContextPrefix
	return r.Q.ListCatalogTranslations(ctx, arg)
}

// Review stores text as the reviewed translation of the source with hash,
// into lang, by user reviewer.
func (r *Reviewer) Review(ctx context.Context, reviewer uuid.UUID, lang, hash, text string) (dbgen.GetTranslationRow, error) {
	key := dbgen.GetTranslationParams{SourceHash: hash, Lang: lang}
	text = strings.TrimSpace(text)
	if text == "" {
		return dbgen.GetTranslationRow{}, errs.Invalid("text: required")
	}
	t, err := r.Q.GetTranslation(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, errs.NotFound("no such translation")
	}
	if err != nil {
		return t, err
	}
	// UI strings are edited as human versions (SaveAppString): a correction
	// here would be hidden by them and lost when machines redo the cache.
	if strings.HasPrefix(t.Context, r.UIContextPrefix) {
		return t, errs.Invalid("this is a UI string: edit it with SaveAppString")
	}
	if !r.SamePlaceholders(t.Source, text) {
		return t, errs.Invalid("text: keep every {placeholder} of the English text, unchanged")
	}
	if _, err := r.Q.ReviewTranslation(ctx, dbgen.ReviewTranslationParams{
		SourceHash: key.SourceHash, Lang: key.Lang, Text: text,
		ReviewedBy: pgtype.UUID{Bytes: reviewer, Valid: true}, At: pgtype.Timestamptz{Time: r.now(), Valid: true},
	}); err != nil {
		return t, err
	}
	return r.Q.GetTranslation(ctx, key)
}
