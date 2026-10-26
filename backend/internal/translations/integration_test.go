package translations

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// TestIntegrationCatalogReview checks the catalogue review against the
// database: UI strings are left out and refused, a correction keeps its
// placeholders, and a machine translation never overwrites it afterwards.
// Skipped unless TEST_DATABASE_URL is set.
func TestIntegrationCatalogReview(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := dbgen.New(pool)

	const source, uiSource = "Translations test: code sent to {email}", "Translations test: Save"
	hash, uiHash := translate.Key(source, "test"), translate.Key(uiSource, uistrings.ContextPrefix)
	cleanup := func() {
		if _, err := pool.Exec(ctx, "delete from app.translations where source_hash = any($1)", []string{hash, uiHash}); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	if err := q.InsertTranslations(ctx, dbgen.InsertTranslationsParams{
		Lang: "ja", Model: "machine",
		SourceHashes: []string{hash, uiHash},
		Sources:      []string{source, uiSource},
		Contexts:     []string{"test", uistrings.ContextPrefix},
		Texts:        []string{"{email} にコードを送信", "節約"},
	}); err != nil {
		t.Fatal(err)
	}

	h := &Handler{Q: q}
	ctx2 := as(japanese)
	list, err := h.ListCatalogTranslations(ctx2, connect.NewRequest(&isshov1.ListCatalogTranslationsRequest{Lang: "ja", UnreviewedOnly: true, Search: "Translations test"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Msg.Translations) != 1 || list.Msg.Translations[0].SourceHash != hash {
		t.Fatalf("translations = %v", list.Msg.Translations)
	}
	review := func(h2, text string) (*connect.Response[isshov1.ReviewCatalogTranslationResponse], error) {
		return h.ReviewCatalogTranslation(ctx2, connect.NewRequest(&isshov1.ReviewCatalogTranslationRequest{SourceHash: h2, Lang: "ja", Text: text}))
	}
	if _, err := review(uiHash, "保存"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a UI string: %v", err)
	}
	if _, err := review(hash, "コードを送信しました"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("lost placeholder: %v", err)
	}
	res, err := review(hash, "{email} にコードを送信しました")
	if err != nil {
		t.Fatal(err)
	}
	if tr := res.Msg.Translation; !tr.Reviewed || tr.Text != "{email} にコードを送信しました" || tr.ReviewedAt == nil {
		t.Fatalf("reviewed = %v", tr)
	}
	if err := q.InsertTranslations(ctx, dbgen.InsertTranslationsParams{
		Lang: "ja", Model: "machine",
		SourceHashes: []string{hash}, Sources: []string{source}, Contexts: []string{"test"}, Texts: []string{"machine again"},
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ := q.GetTranslation(ctx, dbgen.GetTranslationParams{SourceHash: hash, Lang: "ja"}); got.Text != "{email} にコードを送信しました" {
		t.Fatalf("machine translation overwrote the review: %q", got.Text)
	}
	langs, err := h.ListCatalogLanguages(ctx2, connect.NewRequest(&isshov1.ListCatalogLanguagesRequest{}))
	if err != nil || len(langs.Msg.Languages) != 1 || langs.Msg.Languages[0].Lang != "ja" {
		t.Fatalf("languages = %v, %v", langs.Msg.GetLanguages(), err)
	}
}
