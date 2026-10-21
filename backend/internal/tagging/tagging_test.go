package tagging

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

type fakeStore struct{ rows []dbgen.ListCityActivitiesRow }

func (f fakeStore) ListCityActivities(context.Context, string) ([]dbgen.ListCityActivitiesRow, error) {
	return f.rows, nil
}

type fakeTagger struct {
	tags []Tag
	got  []Activity
}

func (f *fakeTagger) Tag(_ context.Context, _ string, acts []Activity) ([]Tag, error) {
	f.got = acts
	return f.tags, nil
}

var tokyo = fakeStore{rows: []dbgen.ListCityActivitiesRow{
	{ID: "hike", Name: []byte(`{"en":"Hiking and nature walks"}`)},
	{ID: "karaoke", Name: []byte(`{"en":"Karaoke"}`)},
}}

func TestSuggest(t *testing.T) {
	ctx := context.Background()
	tagger := &fakeTagger{tags: []Tag{
		{"karaoke", 0}, {"hike", 1}, {"karaoke", 2}, // strongest wins, first-seen order kept
		{"sky-diving", 2}, // not offered in the city
		{"hike", 7},       // bad weight
	}}
	svc := &Service{Store: tokyo, Tagger: tagger}
	got, err := svc.Suggest(ctx, "tokyo", "  I love karaoke and like hiking  ")
	if err != nil {
		t.Fatal(err)
	}
	want := []Tag{{"karaoke", 2}, {"hike", 1}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(tagger.got) != 2 || tagger.got[0] != (Activity{"hike", "Hiking and nature walks"}) {
		t.Errorf("catalogue given to the tagger: %v", tagger.got)
	}
}

func TestSuggestRejects(t *testing.T) {
	ctx := context.Background()
	svc := &Service{Store: tokyo, Tagger: &fakeTagger{}}
	for name, tc := range map[string]struct{ city, text string }{
		"blank":    {"tokyo", "   "},
		"too long": {"tokyo", strings.Repeat("あ", MaxTextLen+1)},
		"no city":  {"", "karaoke"},
		"unknown":  {"atlantis", "karaoke"},
	} {
		store := tokyo
		if tc.city == "atlantis" {
			store = fakeStore{}
		}
		svc.Store = store
		var ie *InvalidError
		if _, err := svc.Suggest(ctx, tc.city, tc.text); !errors.As(err, &ie) {
			t.Errorf("%s: err = %v, want InvalidError", name, err)
		}
	}
	// Exactly MaxTextLen characters is fine, counted in characters not bytes.
	svc.Store = tokyo
	if _, err := svc.Suggest(ctx, "tokyo", strings.Repeat("あ", MaxTextLen)); err != nil {
		t.Errorf("max length: %v", err)
	}
	if _, err := (&Service{Store: tokyo}).Suggest(ctx, "tokyo", "karaoke"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("no tagger: %v", err)
	}
}
