// Package uistrings translates the app's own UI text on demand.
//
// The app is written in English: apps/mobile/src/i18n/en.json is the source,
// and `make gen` copies it here (en.json, embedded). Only strings in this
// copy are ever translated, so the public GetStrings RPC can't be used to
// make the server pay for arbitrary text.
package uistrings

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

//go:embed en.json
var source []byte

// translateContext is what the translator is told the text is. It's part of
// the cache key, so UI text never shares a translation with an activity name.
const translateContext = "text in the user interface of a mobile app for meeting new friends at group outings"

// Catalog is the English UI text, key → text.
type Catalog struct {
	keys    []string // sorted
	english map[string]string
	version string
}

// Load parses the embedded catalogue.
func Load() (*Catalog, error) { return parse(source) }

func parse(raw []byte) (*Catalog, error) {
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("ui strings: %w", err)
	}
	h := sha256.Sum256(raw)
	return &Catalog{keys: slices.Sorted(maps.Keys(m)), english: m, version: hex.EncodeToString(h[:6])}, nil
}

type Handler struct {
	Catalog      *Catalog
	Translations *translate.Service
}

func (h *Handler) GetStrings(ctx context.Context, req *connect.Request[isshov1.GetStringsRequest]) (*connect.Response[isshov1.GetStringsResponse], error) {
	lang, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(req.Msg.Language)), "-")
	if !i18n.Valid(lang) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("language must be a language code such as \"ja\""))
	}
	strs, complete := h.Catalog.In(ctx, h.Translations, lang)
	return connect.NewResponse(&isshov1.GetStringsResponse{
		Strings:        strs,
		Complete:       complete,
		CatalogVersion: h.Catalog.version,
	}), nil
}

// In returns the catalogue in lang, leaving out strings that aren't
// translated yet or whose translation lost a placeholder. complete is false
// while some are still to come; a broken translation is cached, so asking
// again wouldn't fix it.
func (c *Catalog) In(ctx context.Context, tr *translate.Service, lang string) (map[string]string, bool) {
	if lang == i18n.Authoring {
		return maps.Clone(c.english), true
	}
	texts := make([]translate.Text, len(c.keys))
	for i, k := range c.keys {
		raw, _ := json.Marshal(map[string]string{i18n.Authoring: c.english[k]})
		texts[i] = translate.Text{Raw: raw, Context: translateContext}
	}
	got := tr.Resolve(ctx, lang, texts)
	out := make(map[string]string, len(c.keys))
	complete := true
	for i, k := range c.keys {
		switch {
		case got[i] == "":
			complete = false
		case samePlaceholders(c.english[k], got[i]):
			out[k] = got[i]
		}
	}
	return out, complete
}

// Text returns one string in lang, or its English text when the
// translation isn't ready or lost a placeholder. For text the server sends
// itself, such as push notifications.
func (c *Catalog) Text(ctx context.Context, tr *translate.Service, lang, key string) string {
	english := c.english[key]
	if lang == i18n.Authoring || tr == nil || english == "" {
		return english
	}
	raw, _ := json.Marshal(map[string]string{i18n.Authoring: english})
	got := tr.Resolve(ctx, lang, []translate.Text{{Raw: raw, Context: translateContext}})[0]
	if got == "" || !samePlaceholders(english, got) {
		return english
	}
	return got
}

var placeholder = regexp.MustCompile(`\{[a-zA-Z]+\}`)

// samePlaceholders reports whether a translation kept every {placeholder} of
// the English text, unchanged. Machine translation sometimes translates
// or drops them; the app then shows English for that string rather than a
// sentence with a hole in it.
func samePlaceholders(english, translated string) bool {
	a, b := placeholder.FindAllString(english, -1), placeholder.FindAllString(translated, -1)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
