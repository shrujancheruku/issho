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

// Draft proposes a better version of one UI string for a reviewer to edit
// (the translation editor's "Suggest"); nothing is stored. For English it
// drafts the owner's livelier version of the plain text; for any other
// language, a version that reads naturally to a native speaker, starting
// from the current text.
func (t *Translator) Draft(ctx context.Context, lang string, d translate.Draft) (string, error) {
	user, _ := json.Marshal(d)
	raw, err := t.Client.Generate(ctx, gemini.Request{
		System:          draftPrompt(lang),
		User:            string(user),
		Schema:          draftSchema,
		MaxOutputTokens: 4096,
		Thinking:        "LOW",
	})
	if err != nil {
		return "", err
	}
	var a struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return "", fmt.Errorf("gemini draft: %w", err)
	}
	if strings.TrimSpace(a.Text) == "" {
		return "", fmt.Errorf("gemini draft: empty")
	}
	return strings.TrimSpace(a.Text), nil
}

var draftSchema = map[string]any{
	"type":       "object",
	"properties": map[string]any{"text": map[string]any{"type": "string"}},
	"required":   []string{"text"},
}

func draftPrompt(lang string) string {
	if lang == "en" {
		return `You write the English text of the Issho app for native English speakers. The user message is a JSON object with the plain "english" text (written simply so machines translate it well), sometimes a "context" saying what it is and where it appears, and sometimes the "current" livelier version.

Write one livelier, warmer version of it: how a friendly person would say it, not a company. Keep the meaning exact and every {placeholder} unchanged. Keep buttons, labels and anything about safety, rules or someone's account short and clear; liveliness belongs in invitations, confirmations and encouragement. No regional slang, no puns that only work in English writing, no emoji. Answer with the text only.

` + translate.Guide(lang)
	}
	name := display.English.Languages().Name(language.Make(lang))
	if name == "" {
		name = lang
	}
	return fmt.Sprintf(`You are a native %s copywriter improving the Issho app's text in %s (language code %q). The user message is a JSON object with the plain "english" source, sometimes a "context" saying what it is and where it appears, sometimes "voice": the livelier English that English speakers see (a guide to the tone, not to be translated word for word), and "current": the text shown today.

Write one version that a native speaker would find natural and warm in an app like this: idiomatic rather than literal, with the same meaning as the English and every {placeholder} unchanged. Keep buttons and labels short in the usual style of apps in %s. If the current text is already right, you may return it unchanged. Answer with the text only.

%s`, name, name, lang, name, translate.Guide(lang))
}
