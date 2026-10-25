// Package gemini implements translate.Translator with Gemini on Google
// Cloud's Agent Platform (internal/platform/gemini). Unlike Google
// Translation Basic, it reads each item's context (what the text is, e.g. "a
// button that stores changes") and follows the translation guide
// (translate.Guide): the product's name, its terms and the register per
// language.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"github.com/shrujancheruku/issho/backend/internal/platform/gemini"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

// Model is the default: the one that composes walks, at low thinking.
const Model = "gemini-3.8-flash"

const (
	// chunk keeps each answer well inside the output limit (UI strings are a
	// sentence or two; a chunk's answer is a few thousand tokens at most).
	chunk = 40
	// parallel calls per Translate: a whole catalogue (~300 strings) takes
	// one round of calls instead of eight in a row.
	parallel = 4
)

type Translator struct {
	Client *gemini.Client
}

// New uses a Google Cloud API key for Agent Platform.
func New(apiKey string) *Translator {
	return &Translator{Client: gemini.New(apiKey, Model, 60*time.Second)}
}

var _ translate.Translator = (*Translator)(nil)

func (t *Translator) Name() string { return t.Client.Model }

type item struct {
	ID      int    `json:"id"`
	Text    string `json:"text"`
	Context string `json:"context,omitempty"`
}

type answer struct {
	Translations []struct {
		ID   int    `json:"id"`
		Text string `json:"text"`
	} `json:"translations"`
}

var schema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"translations": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":   map[string]any{"type": "integer"},
					"text": map[string]any{"type": "string"},
				},
				"required": []string{"id", "text"},
			},
		},
	},
	"required": []string{"translations"},
}

// Translate translates items from English into lang, in chunks run side by
// side. Every item must come back: a missing or empty one fails the call,
// which the translate.Service treats like any other failure (nothing cached,
// asked again later).
func (t *Translator) Translate(ctx context.Context, lang string, items []translate.Item) ([]string, error) {
	out := make([]string, len(items))
	system := systemPrompt(lang)
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(parallel)
	for start := 0; start < len(items); start += chunk {
		end := min(start+chunk, len(items))
		g.Go(func() error {
			batch := make([]item, 0, end-start)
			for i := start; i < end; i++ {
				batch = append(batch, item{ID: i, Text: items[i].Text, Context: items[i].Context})
			}
			user, _ := json.Marshal(batch)
			raw, err := t.Client.Generate(ctx, gemini.Request{
				System:          system,
				User:            string(user),
				Schema:          schema,
				MaxOutputTokens: 16384,
				Thinking:        "LOW",
			})
			if err != nil {
				return err
			}
			var a answer
			if err := json.Unmarshal([]byte(raw), &a); err != nil {
				return fmt.Errorf("gemini translate: %w", err)
			}
			for _, tr := range a.Translations {
				if tr.ID >= start && tr.ID < end {
					out[tr.ID] = strings.TrimSpace(tr.Text)
				}
			}
			for i := start; i < end; i++ {
				if out[i] == "" {
					return fmt.Errorf("gemini translate: no translation for item %d", i)
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

func systemPrompt(lang string) string {
	name := display.English.Languages().Name(language.Make(lang))
	if name == "" {
		name = lang
	}
	return fmt.Sprintf(`You translate English text from the Issho app into %s (language code %q).

The user message is a JSON array of items, each with an "id", the English "text" and sometimes a "context" saying what the text is and where it appears. Translate every item's text, using its context to choose the right meaning, and answer with each item's id and its translation. Translate the text only, never the context.

%s`, name, lang, translate.Guide(lang))
}
