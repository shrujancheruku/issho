package translate

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

type reviewStore struct {
	ReviewStore
	rows     map[string]dbgen.GetTranslationRow // by source hash
	reviewed []dbgen.ReviewTranslationParams
}

func (s *reviewStore) GetTranslation(_ context.Context, arg dbgen.GetTranslationParams) (dbgen.GetTranslationRow, error) {
	r, ok := s.rows[arg.SourceHash]
	if !ok {
		return r, pgx.ErrNoRows
	}
	return r, nil
}

func (s *reviewStore) ReviewTranslation(_ context.Context, arg dbgen.ReviewTranslationParams) (int64, error) {
	s.reviewed = append(s.reviewed, arg)
	r := s.rows[arg.SourceHash]
	r.Text, r.Reviewed = arg.Text, true
	s.rows[arg.SourceHash] = r
	return 1, nil
}

func TestReviewerReview(t *testing.T) {
	const ui = "UI text"
	st := &reviewStore{rows: map[string]dbgen.GetTranslationRow{
		"ui":  {SourceHash: "ui", Lang: "ja", Source: "Save", Context: ui + ". A button."},
		"act": {SourceHash: "act", Lang: "ja", Source: "Pottery {n}", Context: "name of a group activity"},
	}}
	at := time.Unix(1700000000, 0)
	r := &Reviewer{
		Q: st, UIContextPrefix: ui, Now: func() time.Time { return at },
		SamePlaceholders: func(en, tr string) bool { return strings.Count(en, "{n}") == strings.Count(tr, "{n}") },
	}
	who := uuid.New()
	for name, tc := range map[string]struct {
		hash, text string
		kind       errs.Kind
	}{
		"empty text":          {"act", "  ", errs.KindInvalid},
		"unknown translation": {"nope", "x", errs.KindNotFound},
		"a UI string":         {"ui", "保存", errs.KindInvalid},
		"a lost placeholder":  {"act", "陶芸", errs.KindInvalid},
	} {
		_, err := r.Review(context.Background(), who, "ja", tc.hash, tc.text)
		var e *errs.Error
		if !errors.As(err, &e) || e.Kind != tc.kind {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(st.reviewed) != 0 {
		t.Fatalf("refused reviews wrote: %+v", st.reviewed)
	}
	got, err := r.Review(context.Background(), who, "ja", "act", " 陶芸 {n} ")
	if err != nil || !got.Reviewed || got.Text != "陶芸 {n}" {
		t.Fatalf("review: %+v %v", got, err)
	}
	if w := st.reviewed[0]; w.ReviewedBy.Bytes != who || !w.At.Time.Equal(at) {
		t.Errorf("reviewer and time: %+v", w)
	}
}
