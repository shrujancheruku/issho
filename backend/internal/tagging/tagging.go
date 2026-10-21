// Package tagging turns what someone writes about their interests, in any
// language, into activities from the catalogue, so onboarding can suggest
// tiles instead of making people find them all. Suggestions are only ever
// suggestions: the app shows them and the person confirms each one.
package tagging

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

// MaxTextLen bounds the text a person can submit (characters): enough for a
// few sentences about their interests, small enough to keep each call cheap.
const MaxTextLen = 500

var (
	// ErrUnavailable means no Tagger is configured (GOOGLE_GEMINI_API_KEY unset).
	ErrUnavailable = errors.New("interest suggestions are not configured on this server")
	// ErrNoSuggestion means the model declined to answer (a refusal); the app
	// asks the person to pick tiles themselves.
	ErrNoSuggestion = errors.New("no suggestion for this text")
)

// Activity is one catalogue entry offered to the tagger: its id and English
// name (the authoring language; the tagger reads text in any language).
type Activity struct {
	ID   string
	Name string
}

// Tag is one suggested activity with an interest weight, as in
// app.user_activity_prefs: 0 open to try, 1 like, 2 love.
type Tag struct {
	ActivityID string
	Weight     int
}

// Tagger maps free text to activities. Implementations must only return ids
// from activities.
type Tagger interface {
	Tag(ctx context.Context, text string, activities []Activity) ([]Tag, error)
}

// Store is the subset of generated queries this service needs.
type Store interface {
	ListCityActivities(ctx context.Context, cityID string) ([]dbgen.ListCityActivitiesRow, error)
}

// InvalidError is a problem with the caller's input, safe to show them.
type InvalidError struct{ Reason string }

func (e *InvalidError) Error() string { return e.Reason }

type Service struct {
	Store  Store
	Tagger Tagger // nil = unavailable
}

// Suggest returns activities the text suggests, among those offered in the
// city, at most one tag per activity (the strongest).
func (s *Service) Suggest(ctx context.Context, cityID, text string) ([]Tag, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, &InvalidError{"text is required"}
	}
	if utf8.RuneCountInString(text) > MaxTextLen {
		return nil, &InvalidError{"text is too long"}
	}
	if cityID == "" {
		return nil, &InvalidError{"city_id is required"}
	}
	if s.Tagger == nil {
		return nil, ErrUnavailable
	}
	rows, err := s.Store.ListCityActivities(ctx, cityID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, &InvalidError{"unknown city"}
	}
	acts := make([]Activity, len(rows))
	known := make(map[string]bool, len(rows))
	for i, r := range rows {
		// The English name, with the city's override (e.g. "Lake or pool day"
		// in Austin): the tagger matches against what the person will see.
		acts[i] = Activity{ID: r.ID, Name: i18n.Pick(r.Name, i18n.Authoring)}
		known[r.ID] = true
	}
	tags, err := s.Tagger.Tag(ctx, text, acts)
	if err != nil {
		return nil, err
	}
	// Defence in depth: the Gemini tagger's schema already restricts ids to
	// this list, but a Tagger is an interface.
	best := map[string]int{}
	var order []string
	for _, t := range tags {
		if !known[t.ActivityID] || t.Weight < 0 || t.Weight > 2 {
			continue
		}
		w, seen := best[t.ActivityID]
		if !seen {
			order = append(order, t.ActivityID)
		}
		if !seen || t.Weight > w {
			best[t.ActivityID] = t.Weight
		}
	}
	out := make([]Tag, len(order))
	for i, id := range order {
		out[i] = Tag{ActivityID: id, Weight: best[id]}
	}
	return out, nil
}
