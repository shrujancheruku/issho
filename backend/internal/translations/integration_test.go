package translations

import (
	"context"
	"slices"
	"testing"

	"connectrpc.com/connect"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// TestIntegrationCatalogReview checks the catalogue review against the
// database: UI strings are left out and refused, a correction keeps its
// placeholders, and a machine translation never overwrites it afterwards.
// Skipped unless TEST_DATABASE_URL is set.
func TestIntegrationCatalogReview(t *testing.T) {
	ctx := context.Background()
	pool := testfixture.Pool(t)
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

	h := &Handler{Reviews: &translate.Reviewer{Q: q, UIContextPrefix: uistrings.ContextPrefix, SamePlaceholders: uistrings.SamePlaceholders}}
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

// TestIntegrationCatalogPaging walks the catalogue list a page at a time:
// every row once, in order (unreviewed first), with sources that repeat. A
// token is for one search and one language. Skipped unless TEST_DATABASE_URL
// is set.
func TestIntegrationCatalogPaging(t *testing.T) {
	ctx := context.Background()
	pool := testfixture.Pool(t)
	q := dbgen.New(pool)

	// Three sources, each in four contexts, so the source repeats; three
	// rows are reviewed.
	var arg dbgen.InsertTranslationsParams
	arg.Lang, arg.Model = "ja", "machine"
	for _, src := range []string{"Translations paging test A", "Translations paging test B", "Translations paging test C"} {
		for _, c := range []string{"one", "two", "three", "four"} {
			arg.SourceHashes = append(arg.SourceHashes, translate.Key(src, c))
			arg.Sources = append(arg.Sources, src)
			arg.Contexts = append(arg.Contexts, c)
			arg.Texts = append(arg.Texts, "訳 "+src+c)
		}
	}
	cleanup := func() {
		if _, err := pool.Exec(ctx, "delete from app.translations where source_hash = any($1)", arg.SourceHashes); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	if err := q.InsertTranslations(ctx, arg); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "update app.translations set reviewed = true where source_hash = any($1)", arg.SourceHashes[2:5]); err != nil {
		t.Fatal(err)
	}
	var want []string
	rows, err := pool.Query(ctx, `select source_hash from app.translations where lang = 'ja' and source like 'Translations paging test%'
		order by reviewed, source, context, source_hash`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			t.Fatal(err)
		}
		want = append(want, h)
	}
	rows.Close()
	if len(want) != 12 {
		t.Fatalf("setup: %d rows", len(want))
	}

	h := &Handler{Reviews: &translate.Reviewer{Q: q, UIContextPrefix: uistrings.ContextPrefix, SamePlaceholders: uistrings.SamePlaceholders}}
	ctx2 := as(japanese)
	page := func(search string, size int32, token string) (*isshov1.ListCatalogTranslationsResponse, error) {
		res, err := h.ListCatalogTranslations(ctx2, connect.NewRequest(&isshov1.ListCatalogTranslationsRequest{
			Lang: "ja", Search: search, PageSize: size, PageToken: token,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	for _, size := range []int32{5, 0, 1000} {
		var got []string
		token := ""
		for range 100 {
			res, err := page("Translations paging test", size, token)
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range res.Translations {
				got = append(got, tr.SourceHash)
			}
			if token = res.NextPageToken; token == "" {
				break
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("page_size %d: got %d rows, want %d in order", size, len(got), len(want))
		}
	}
	first, err := page("Translations paging test", 2, "")
	if err != nil || first.NextPageToken == "" {
		t.Fatalf("first page: %v", err)
	}
	if _, err := page("paging test A", 2, first.NextPageToken); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("token for another search: %v", err)
	}
	if _, err := page("", 2, "garbage"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("garbage token: %v", err)
	}
	// The translator's language check runs first, token or not.
	if _, err := h.ListCatalogTranslations(ctx2, connect.NewRequest(&isshov1.ListCatalogTranslationsRequest{
		Lang: "es", Search: "Translations paging test", PageToken: first.NextPageToken,
	})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a Japanese translator reading Spanish with a token: %v", err)
	}
}
