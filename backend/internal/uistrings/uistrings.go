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
// also the last resort when nothing else is ready. The API starts with the
// copies built into it and then follows the default branch (Follow), so a
// merged pull request from the translation editor is served without a
// deploy.
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
	"log/slog"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"

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

// ContextPrefix starts the context of every UI string in app.translations,
// so other services can tell UI strings from catalogue text there.
const ContextPrefix = translateContext

// HumanDir holds the human versions in the repository (the source), and
// HumanCopyDir the copy `make gen` makes for the API to embed. The
// translation editor commits both, so CI's check of `make gen` passes.
const (
	HumanDir     = "apps/mobile/src/i18n/human"
	HumanCopyDir = "backend/internal/uistrings/human"
)

// Human versions by language, then key.
type humanSet = map[string]map[string]Human

// Catalog is the plain English UI text, key → text, the human versions and
// the notes for translators.
type Catalog struct {
	keys    []string // sorted
	english map[string]string
	// human is replaced whole when the default branch changes (SetHuman);
	// readers take one snapshot.
	human atomic.Pointer[humanSet]
	notes map[string]string // key → note
}

// Keys are every UI string's key, sorted.
func (c *Catalog) Keys() []string { return c.keys }

// English is key's plain English; false when there's no such key.
func (c *Catalog) English(key string) (string, bool) {
	s, ok := c.english[key]
	return s, ok
}

// HumanVersions is lang's human versions as last loaded (read only).
func (c *Catalog) HumanVersions(lang string) map[string]Human { return (*c.human.Load())[lang] }

// HumanLanguages are the languages that have a human-version file.
func (c *Catalog) HumanLanguages() []string { return slices.Sorted(maps.Keys(*c.human.Load())) }

// Current reports whether v can be shown for key: written for today's
// English, not empty, and with every placeholder kept.
func (c *Catalog) Current(key string, v Human) bool {
	english := c.english[key]
	return english != "" && strings.TrimSpace(v.Text) != "" &&
		v.Source == SourceHash(english) && SamePlaceholders(english, v.Text)
}

// SetHuman replaces every language's human versions with files (language →
// a human/<lang>.json). Nothing changes if one doesn't parse.
func (c *Catalog) SetHuman(files map[string][]byte) error {
	h, err := parseHuman(files)
	if err != nil {
		return err
	}
	c.human.Store(&h)
	return nil
}

func parseHuman(files map[string][]byte) (humanSet, error) {
	h := make(humanSet, len(files))
	for lang, raw := range files {
		var v map[string]Human
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("ui strings, human/%s.json: %w", lang, err)
		}
		h[lang] = v
	}
	return h, nil
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

// Context is what a translator is told about key (see context).
func (c *Catalog) Context(key string) string { return c.context(key) }

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
	var notes map[string]string
	if err := json.Unmarshal(notesRaw, &notes); err != nil {
		return nil, fmt.Errorf("ui strings, notes.json: %w", err)
	}
	return New(source, human, notes)
}

// New builds a catalogue from an en.json, the human files by language and
// the notes: Load's embedded ones, or a test's.
func New(english []byte, human map[string][]byte, notes map[string]string) (*Catalog, error) {
	c, err := parse(english, human)
	if err != nil {
		return nil, err
	}
	c.notes = notes
	return c, nil
}

// parse takes en.json and the human files by language.
func parse(raw []byte, human map[string][]byte) (*Catalog, error) {
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("ui strings: %w", err)
	}
	c := &Catalog{keys: slices.Sorted(maps.Keys(m)), english: m}
	if err := c.SetHuman(human); err != nil {
		return nil, err
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
	v, ok := c.HumanVersions(lang)[key]
	if !ok || !c.Current(key, v) {
		return "", false
	}
	return v.Text, true
}

type Handler struct {
	Catalog      *Catalog
	Translations *translate.Service
}

func (h *Handler) GetStrings(ctx context.Context, req *connect.Request[isshov1.GetStringsRequest]) (*connect.Response[isshov1.GetStringsResponse], error) {
	lang := i18n.Display(req.Msg.Language)
	if !i18n.ValidDisplay(lang) {
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

// Arguments lists the arguments of a UI string, sorted: "{name}" gives
// "name", and an ICU plural block "{count, plural, one {…} other {# …}}"
// gives "count" (the forms inside may differ by language: Polish has four,
// Japanese one). ok is false when the braces don't parse: unbalanced, an
// argument that isn't {lowerCamel}, or a block other than plural.
func Arguments(text string) (args []string, ok bool) {
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '}':
			return nil, false
		case '{':
			m := argument.FindStringSubmatchIndex(text[i:])
			if m == nil {
				return nil, false
			}
			name := text[i+m[2] : i+m[3]]
			if m[4] < 0 { // {name}
				args = append(args, name)
				i += m[1] - 1
				continue
			}
			// {name, plural, ...}: skip to the matching brace.
			depth, j := 0, i
			for ; j < len(text); j++ {
				if text[j] == '{' {
					depth++
				} else if text[j] == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			if j == len(text) {
				return nil, false
			}
			args = append(args, name)
			i = j
		}
	}
	slices.Sort(args)
	return args, true
}

// argument is the start of one: {name} or {name, plural,
var argument = regexp.MustCompile(`^\{([a-z][a-zA-Z]*)(?:\}|(\s*,\s*plural\s*,))`)

// SamePlaceholders reports whether a translation kept every argument of the
// English text ({placeholder}s and plural blocks), unchanged in name and
// parseable. Machine translation sometimes translates or drops them; the app
// then shows English for that string rather than a sentence with a hole in
// it.
func SamePlaceholders(english, translated string) bool {
	a, okA := Arguments(english)
	b, okB := Arguments(translated)
	return okA && okB && slices.Equal(a, b)
}

// Machine is every UI string's machine translation into lang, ignoring
// human versions: what the translation editor shows next to them. Strings
// still being translated are missing; complete is false then.
func (c *Catalog) Machine(ctx context.Context, tr *translate.Service, lang string) (map[string]string, bool) {
	texts := make([]translate.Text, len(c.keys))
	for i, k := range c.keys {
		raw, _ := json.Marshal(map[string]string{i18n.Authoring: c.english[k]})
		texts[i] = translate.Text{Raw: raw, Context: c.context(k)}
	}
	got := tr.Resolve(ctx, lang, texts)
	out := make(map[string]string, len(c.keys))
	complete := true
	for i, k := range c.keys {
		if got[i] == "" {
			complete = false
			continue
		}
		out[k] = got[i]
	}
	return out, complete
}

// HumanSource reads the human-version files on a branch (*github.Client).
type HumanSource interface {
	Dir(ctx context.Context, path, branch string) ([]string, error)
	File(ctx context.Context, path, branch string) ([]byte, string, error)
}

// LoadHuman replaces the human versions with branch's (HumanDir). On any
// error nothing changes.
func (c *Catalog) LoadHuman(ctx context.Context, src HumanSource, branch string) error {
	names, err := src.Dir(ctx, HumanDir, branch)
	if err != nil {
		return err
	}
	files := map[string][]byte{}
	for _, name := range names {
		lang, ok := strings.CutSuffix(name, ".json")
		if !ok || !i18n.ValidDisplay(lang) {
			continue
		}
		raw, _, err := src.File(ctx, HumanDir+"/"+name, branch)
		if err != nil {
			return err
		}
		files[lang] = raw
	}
	return c.SetHuman(files)
}

// Follow keeps the human versions in step with branch, reading it every
// interval until ctx ends, so a merged pull request from the translation
// editor reaches the app (whose check runs hourly) without a deploy. A
// failed read keeps what's loaded. Versions written for English newer than
// this build's are stale here (Current) and skipped, so following a branch
// ahead of the deployed API is safe.
func (c *Catalog) Follow(ctx context.Context, src HumanSource, branch string, every time.Duration, log *slog.Logger) {
	for {
		if err := c.LoadHuman(ctx, src, branch); err != nil && ctx.Err() == nil {
			log.Warn("ui strings: couldn't read the human versions on "+branch, "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}
