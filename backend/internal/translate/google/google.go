// Package google implements translate.Translator with Google Cloud Translation
// Basic (v2): neural machine translation over REST with an API key, the same
// way the Maps client works (internal/platform/googleapi).
package google

import (
	"context"
	"fmt"
	"html"
	"unicode"
	"unicode/utf8"

	"github.com/shrujancheruku/issho/backend/internal/platform/googleapi"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

const (
	defaultURL = "https://translation.googleapis.com/language/translate/v2"
	// Google allows at most 128 strings per request and recommends keeping a
	// request under ~5,000 characters for latency.
	maxSegments = 128
	maxChars    = 5000
)

type Translator struct {
	api *googleapi.Client
	URL string // overridable for tests
}

// New uses a Google API key restricted to the Cloud Translation API.
func New(apiKey string) *Translator {
	return &Translator{api: googleapi.New(apiKey), URL: defaultURL}
}

var _ translate.Translator = (*Translator)(nil)

func (t *Translator) Name() string { return "google-translate-v2" }

type request struct {
	Q      []string `json:"q"`
	Target string   `json:"target"`
	Source string   `json:"source"`
	// "text", not the default "html", so "&" isn't returned as "&amp;".
	Format string `json:"format"`
}

type response struct {
	Data struct {
		Translations []struct {
			TranslatedText string `json:"translatedText"`
		} `json:"translations"`
	} `json:"data"`
}

// Translate translates items from English. Item.Context isn't used: Basic has
// no way to pass it. It still keys the cache, so a context-aware translator
// can replace this one later without stale entries.
func (t *Translator) Translate(ctx context.Context, lang string, items []translate.Item) ([]string, error) {
	out := make([]string, 0, len(items))
	for start := 0; start < len(items); {
		end, chars := start, 0
		for end < len(items) && end-start < maxSegments {
			n := utf8.RuneCountInString(items[end].Text)
			if end > start && chars+n > maxChars {
				break
			}
			chars += n
			end++
		}
		req := request{Target: lang, Source: i18n.Authoring, Format: "text"}
		for _, it := range items[start:end] {
			req.Q = append(req.Q, it.Text)
		}
		var resp response
		if err := t.api.Post(ctx, t.URL, nil, req, &resp); err != nil {
			return nil, err
		}
		if len(resp.Data.Translations) != end-start {
			return nil, fmt.Errorf("google translate: sent %d strings, got %d translations", end-start, len(resp.Data.Translations))
		}
		for i, tr := range resp.Data.Translations {
			// Defensive: some responses still carry HTML entities in text mode.
			out = append(out, matchCapital(items[start+i].Text, html.UnescapeString(tr.TranslatedText)))
		}
		start = end
	}
	return out, nil
}

// matchCapital capitalises the translation's first letter when the English
// source starts with one: Google sometimes returns labels in lower case
// ("concierto de banda indie"). Scripts without case (Japanese, Korean) are
// unaffected.
func matchCapital(source, translated string) string {
	s, _ := utf8.DecodeRuneInString(source)
	t, size := utf8.DecodeRuneInString(translated)
	if !unicode.IsUpper(s) || !unicode.IsLower(t) {
		return translated
	}
	return string(unicode.ToUpper(t)) + translated[size:]
}
