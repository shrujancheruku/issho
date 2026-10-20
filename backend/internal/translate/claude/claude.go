// Package claude implements translate.Translator with Claude (Haiku): fast and
// cheap for short UI strings, and much better than a plain machine-translation
// API at one-word labels because it gets context for each item.
package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/shrujancheruku/issho/backend/internal/translate"
)

// Model is the Claude model used for translation (see the ROADMAP decision log:
// Haiku for tagging and copy).
const Model = "claude-haiku-5-5"

const systemPrompt = `You translate short pieces of text for Issho, an app that proposes small-group outings to people who want to make friends (platonic, not dating).

You receive a JSON array of items, each with an id, an English text and a note on what the text is. Translate every item into the target language and return each translation with the same id.

- Write what a native speaker would naturally see in an app: short, everyday words, no explanations or quotation marks.
- Keep the length and tone of the original. A label stays a label.
- Keep proper nouns (place names, brand names) as they are commonly written in the target language.
- If a term is normally used untranslated in the target language (e.g. "karaoke" in Spanish), use that form.`

// The response schema: structured outputs guarantee valid JSON of this shape.
var outputSchema = map[string]any{
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
				"required":             []string{"id", "text"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"translations"},
	"additionalProperties": false,
}

type Translator struct {
	client anthropic.Client
}

// New uses the given API key (ANTHROPIC_API_KEY, a server-only secret).
func New(apiKey string, opts ...option.RequestOption) *Translator {
	return &Translator{client: anthropic.NewClient(append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)...)}
}

var _ translate.Translator = (*Translator)(nil)

func (t *Translator) Name() string { return Model }

type inputItem struct {
	ID      int    `json:"id"`
	Text    string `json:"text"`
	Context string `json:"context,omitempty"`
}

func (t *Translator) Translate(ctx context.Context, lang string, items []translate.Item) ([]string, error) {
	in := make([]inputItem, len(items))
	for i, it := range items {
		in[i] = inputItem{ID: i, Text: it.Text, Context: it.Context}
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	resp, err := t.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     Model,
		MaxTokens: 16000,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		OutputConfig: anthropic.OutputConfigParam{
			// Short, well-specified work: low effort keeps latency down.
			Effort: anthropic.OutputConfigEffortLow,
			Format: anthropic.JSONOutputFormatParam{Schema: outputSchema},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(
				fmt.Sprintf("Target language: %s (ISO 639-1 code).\n\n%s", lang, payload))),
		},
	})
	if err != nil {
		return nil, err
	}
	switch resp.StopReason {
	case anthropic.StopReasonEndTurn:
	case anthropic.StopReasonRefusal:
		return nil, fmt.Errorf("translation refused (%s)", resp.StopDetails.Category)
	default:
		return nil, fmt.Errorf("translation stopped early: %s", resp.StopReason)
	}

	var text strings.Builder
	for _, block := range resp.Content {
		if b, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(b.Text)
		}
	}
	var out struct {
		Translations []struct {
			ID   int    `json:"id"`
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal([]byte(text.String()), &out); err != nil {
		return nil, fmt.Errorf("parse translation: %w", err)
	}
	// Match by id, not position; anything missing stays "" (not cached, retried
	// on a later request).
	res := make([]string, len(items))
	for _, tr := range out.Translations {
		if tr.ID >= 0 && tr.ID < len(res) {
			res[tr.ID] = strings.TrimSpace(tr.Text)
		}
	}
	for _, r := range res {
		if r != "" {
			return res, nil
		}
	}
	return nil, errors.New("translation returned no items")
}
