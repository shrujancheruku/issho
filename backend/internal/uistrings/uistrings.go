// Package uistrings serves the app's own UI text in any language.
//
// The app is written in plain English: apps/mobile/src/i18n/en.json is the
// source, and `make gen` copies it here (en.json, embedded). Only strings in
// this copy are ever translated, so the public GetStrings RPC can't be used
// to make the server pay for arbitrary text.
//
// Human versions (human/<lang>.json, copied the same way; see Human) replace
// single strings: the owner's livelier English, and translations a person
// wrote or approved. Machines still translate the plain English, which is
// also the last resort when nothing else is ready.
package uistrings

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
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

//go:embed human/*.json
var humanFiles embed.FS

// notes.json (copied by `make gen`): what an ambiguous string is, for the
// translator ("Save" is a button that stores changes, not saving money).
//
//go:embed notes.json
var notesRaw []byte

// translateContext is what the translator is told the text is. It's part of
// the cache key, so UI text never shares a translation with an activity name.
const translateContext = "text in the user interface of a mobile app for meeting new friends at group outings"

// Catalog is the plain English UI text, key → text, the human versions and
// the notes for translators.
type Catalog struct {
	keys    []string // sorted
	english map[string]string
	human   map[string]map[string]Human // lang → key → version
	notes   map[string]string           // key → note
}

// Items is the catalogue as a translator gets it: the plain English of every
// key, in key order, with its context. For comparing translators (tool
// translate-compare), without the cache in between.
func (c *Catalog) Items() (keys []string, items []translate.Item) {
	for _, k := range c.keys {
		keys = append(keys, k)
		items = append(items, translate.Item{Text: c.english[k], Context: c.context(k)})
	}
	return keys, items
}

// Note is key's note for translators, if it has one.
func (c *Catalog) Note(key string) string { return c.notes[key] }

// context is what the translator is told about key: what all UI text is,
// plus the key's note. Being part of the cache key, a new or reworded note
// gets that string translated again.
func (c *Catalog) context(key string) string {
	if n := c.notes[key]; n != "" {
		return translateContext + ". " + n
	}
	return translateContext
}

// Load parses the embedded catalogue and human versions.
func Load() (*Catalog, error) {
	files, err := fs.Glob(humanFiles, "human/*.json")
	if err != nil {
		return nil, err
	}
	human := make(map[string][]byte, len(files))
	for _, f := range files {
		raw, err := humanFiles.ReadFile(f)
		if err != nil {
			return nil, err
		}
		human[strings.TrimSuffix(path.Base(f), ".json")] = raw
	}
	c, err := parse(source, human)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(notesRaw, &c.notes); err != nil {
		return nil, fmt.Errorf("ui strings, notes.json: %w", err)
	}
	return c, nil
}

// parse takes en.json and the human files by language.
func parse(raw []byte, human map[string][]byte) (*Catalog, error) {
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("ui strings: %w", err)
	}
	c := &Catalog{keys: slices.Sorted(maps.Keys(m)), english: m, human: make(map[string]map[string]Human, len(human))}
	for lang, raw := range human {
		var v map[string]Human
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("ui strings, human/%s.json: %w", lang, err)
		}
		c.human[lang] = v
	}
	return c, nil
}

// Version identifies a catalogue's text, so the app can ask whether its copy
// is still current (GetStringsRequest.known_version). It's a hash of what's
// shown rather than of the files, because a reviewer's correction or a new
// machine translation changes the text without a new build.
func Version(strs map[string]string) string {
	h := sha256.New()
	for _, k := range slices.Sorted(maps.Keys(strs)) {
		h.Write([]byte(k + "\x00" + strs[k] + "\x00"))
	}
	return hex.EncodeToString(h.Sum(nil)[:6])
}

// humanText is key's human version in lang, if it has one written for
// today's English that kept every placeholder. A stale one is skipped (see
// Human.Source): the plain text or a fresh translation is shown instead.
func (c *Catalog) humanText(lang, key string) (string, bool) {
	v, ok := c.human[lang][key]
	english := c.english[key]
	if !ok || english == "" || strings.TrimSpace(v.Text) == "" ||
		v.Source != SourceHash(english) || !SamePlaceholders(english, v.Text) {
		return "", false
	}
	return v.Text, true
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
	version := Version(strs)
	// The app checks a complete copy every so often; most checks find it
	// current, and then there's no need to send ~300 strings again.
	if complete && req.Msg.KnownVersion == version {
		return connect.NewResponse(&isshov1.GetStringsResponse{Complete: true, CatalogVersion: version, Unchanged: true}), nil
	}
	return connect.NewResponse(&isshov1.GetStringsResponse{
		Strings:        strs,
		Complete:       complete,
		CatalogVersion: version,
	}), nil
}

// In returns the catalogue in lang: human versions where there are any,
// machine translations of the plain English for the rest, leaving out strings
// that aren't translated yet or whose translation lost a placeholder.
// complete is false while some are still to come; a broken translation is
// cached, so asking again wouldn't fix it.
func (c *Catalog) In(ctx context.Context, tr *translate.Service, lang string) (map[string]string, bool) {
	out := make(map[string]string, len(c.keys))
	var todo []string // strings without a human version
	for _, k := range c.keys {
		if text, ok := c.humanText(lang, k); ok {
			out[k] = text
		} else {
			todo = append(todo, k)
		}
	}
	if lang == i18n.Authoring {
		for _, k := range todo {
			out[k] = c.english[k]
		}
		return out, true
	}
	if len(todo) == 0 {
		return out, true
	}
	texts := make([]translate.Text, len(todo))
	for i, k := range todo {
		raw, _ := json.Marshal(map[string]string{i18n.Authoring: c.english[k]})
		texts[i] = translate.Text{Raw: raw, Context: c.context(k)}
	}
	got := tr.Resolve(ctx, lang, texts)
	complete := true
	for i, k := range todo {
		switch {
		case got[i] == "":
			complete = false
		case SamePlaceholders(c.english[k], got[i]):
			out[k] = got[i]
		}
	}
	return out, complete
}

// Text returns one string in lang: its human version, else its machine
// translation, else the plain English when the translation isn't ready or
// lost a placeholder (plain, not the livelier English: whoever gets the
// fallback may not read English well). For text the server sends itself,
// such as push notifications.
func (c *Catalog) Text(ctx context.Context, tr *translate.Service, lang, key string) string {
	if text, ok := c.humanText(lang, key); ok {
		return text
	}
	english := c.english[key]
	if lang == i18n.Authoring || tr == nil || english == "" {
		return english
	}
	raw, _ := json.Marshal(map[string]string{i18n.Authoring: english})
	got := tr.Resolve(ctx, lang, []translate.Text{{Raw: raw, Context: c.context(key)}})[0]
	if got == "" || !SamePlaceholders(english, got) {
		return english
	}
	return got
}

var placeholder = regexp.MustCompile(`\{[a-zA-Z]+\}`)

// SamePlaceholders reports whether a translation kept every {placeholder} of
// the English text, unchanged. Machine translation sometimes translates
// or drops them; the app then shows English for that string rather than a
// sentence with a hole in it.
func SamePlaceholders(english, translated string) bool {
	a, b := placeholder.FindAllString(english, -1), placeholder.FindAllString(translated, -1)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
