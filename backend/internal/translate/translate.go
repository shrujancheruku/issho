// Package translate provides text in any language on demand.
//
// Our own content is stored in English only (the authoring language). When
// someone needs another language, Resolve returns text that already exists in
// that language (e.g. an event prefilled in its own languages), else a cached
// machine translation, else asks the Translator, stores the result in
// app.translations and reuses it for everyone after that.
//
// Translation is best-effort: if the translator is off, slow or failing,
// Resolve returns "" for that text and callers fall back (i18n.Pair). A slow
// translation keeps running in the background so the next request finds it
// cached.
package translate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

// Item is one English string to translate, with a short note on what it is
// ("activity name", "venue category"). Translators that can use context get it;
// it's also part of the cache key, so the same word in two roles can differ.
type Item struct {
	Text    string
	Context string
}

// Translator turns English items into the target language, preserving order.
type Translator interface {
	Translate(ctx context.Context, lang string, items []Item) ([]string, error)
	// Name identifies the translator in app.translations.model.
	Name() string
}

// Store is the cache; *dbgen.Queries implements it.
type Store interface {
	GetTranslations(ctx context.Context, arg dbgen.GetTranslationsParams) ([]dbgen.GetTranslationsRow, error)
	InsertTranslations(ctx context.Context, arg dbgen.InsertTranslationsParams) error
}

// Text is one piece of localised text as stored ({"en": "...", ...}) plus
// the context to give the translator.
type Text struct {
	Raw     []byte
	Context string
}

const (
	// DefaultWait is how long a request waits for fresh translations before
	// falling back. First requests for a language are slow; later ones hit the
	// cache. Short, because a request that falls back is marked incomplete
	// and the app asks again (GetStrings), while a long wait holds up a screen.
	DefaultWait = 3 * time.Second
	// backgroundTimeout bounds a translation that outlives its request.
	backgroundTimeout = 2 * time.Minute
	// batchSize keeps each translator call (and its JSON output) small.
	batchSize = 100
)

type Service struct {
	Store      Store
	Translator Translator // nil disables machine translation
	Wait       time.Duration

	mu       sync.Mutex
	inflight map[string]*call // key: lang + "/" + hash
}

// call is one text being translated; done closes when text is final ("" on failure).
type call struct {
	done chan struct{}
	text string
}

// Key identifies an English source and its context in the cache.
func Key(source, context string) string {
	h := sha256.Sum256([]byte(source + "\x00" + context))
	return hex.EncodeToString(h[:])
}

// Resolve returns each text in lang, "" where it isn't available yet. It
// waits at most Service.Wait for fresh translations.
func (s *Service) Resolve(ctx context.Context, lang string, texts []Text) []string {
	wait := s.Wait
	if wait == 0 {
		wait = DefaultWait
	}
	return s.resolve(ctx, lang, texts, wait)
}

func (s *Service) resolve(ctx context.Context, lang string, texts []Text, wait time.Duration) []string {
	out := make([]string, len(texts))
	if lang == "" {
		return out
	}

	// 1. Text stored in the requested language wins; English is always stored.
	type pending struct {
		idx  int
		item Item
		key  string
	}
	var need []pending
	for i, t := range texts {
		var m map[string]string
		if err := json.Unmarshal(t.Raw, &m); err != nil {
			continue
		}
		if v := m[lang]; v != "" {
			out[i] = v
			continue
		}
		src := m[i18n.Authoring]
		if src == "" {
			continue // invalid data; the caller's fallback shows what exists
		}
		need = append(need, pending{i, Item{src, t.Context}, Key(src, t.Context)})
	}
	if len(need) == 0 {
		return out
	}

	// 2. The cache.
	keys := make([]string, 0, len(need))
	for _, p := range need {
		keys = append(keys, p.key)
	}
	cached := map[string]string{}
	rows, err := s.Store.GetTranslations(ctx, dbgen.GetTranslationsParams{Lang: lang, SourceHashes: keys})
	if err != nil {
		slog.Error("translation cache read failed", "lang", lang, "err", err)
	}
	for _, r := range rows {
		cached[r.SourceHash] = r.Text
	}
	var missing []pending
	for _, p := range need {
		if v, ok := cached[p.key]; ok {
			out[p.idx] = v
		} else {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 || s.Translator == nil {
		return out
	}

	// 3. Translate what's missing. Texts already being translated by another
	// request are waited on rather than translated twice.
	waitFor := map[string]*call{}
	var mine []Item
	var mineKeys []string
	s.mu.Lock()
	if s.inflight == nil {
		s.inflight = map[string]*call{}
	}
	for _, p := range missing {
		k := lang + "/" + p.key
		if _, ok := waitFor[k]; ok {
			continue // the same text twice in one request
		}
		c, ok := s.inflight[k]
		if !ok {
			c = &call{done: make(chan struct{})}
			s.inflight[k] = c
			mine = append(mine, p.item)
			mineKeys = append(mineKeys, p.key)
		}
		waitFor[k] = c
	}
	s.mu.Unlock()
	if len(mine) > 0 {
		// Detached from the request: if the client gives up or we stop waiting,
		// the translation still completes and lands in the cache.
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), backgroundTimeout)
		go func() {
			defer cancel()
			s.translate(bg, lang, mine, mineKeys)
		}()
	}

	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for _, p := range missing {
		c := waitFor[lang+"/"+p.key]
		select {
		case <-c.done:
			out[p.idx] = c.text
		case <-deadline.C:
			slog.Info("translation still running; falling back for now", "lang", lang)
			return out
		case <-ctx.Done():
			return out
		}
	}
	return out
}

// translate runs the translator in batches, stores results and releases waiters.
func (s *Service) translate(ctx context.Context, lang string, items []Item, keys []string) {
	results := make([]string, len(items))
	var wg sync.WaitGroup
	for start := 0; start < len(items); start += batchSize {
		end := min(start+batchSize, len(items))
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			got, err := s.Translator.Translate(ctx, lang, items[start:end])
			if err != nil {
				slog.Error("translation failed", "lang", lang, "items", end-start, "err", err)
				return
			}
			batch := dbgen.InsertTranslationsParams{Lang: lang, Model: s.Translator.Name()}
			for i, text := range got {
				results[start+i] = text
				if text == "" {
					continue
				}
				batch.SourceHashes = append(batch.SourceHashes, keys[start+i])
				batch.Sources = append(batch.Sources, items[start+i].Text)
				batch.Contexts = append(batch.Contexts, items[start+i].Context)
				batch.Texts = append(batch.Texts, text)
			}
			if len(batch.Texts) > 0 {
				if err := s.Store.InsertTranslations(ctx, batch); err != nil {
					slog.Error("translation cache write failed", "lang", lang, "err", err)
				}
			}
		}(start, end)
	}
	wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()
	for i, k := range keys {
		if c := s.inflight[lang+"/"+k]; c != nil {
			c.text = results[i]
			close(c.done)
			delete(s.inflight, lang+"/"+k)
		}
	}
}

// Prefill returns the localised JSON for an English text with every given
// language filled in. Used for content that must exist in specific languages
// up front, such as an event held in Japanese and Spanish. English is always
// included. Unlike Resolve it waits for the translator (it runs in background
// jobs, not requests) and fails if a language can't be produced, so the job
// can retry.
func (s *Service) Prefill(ctx context.Context, english, context string, langs []string) ([]byte, error) {
	m := map[string]string{i18n.Authoring: english}
	raw, _ := json.Marshal(m)
	for _, l := range langs {
		if l == "" || l == i18n.Authoring {
			continue
		}
		v := s.resolve(ctx, l, []Text{{Raw: raw, Context: context}}, backgroundTimeout)[0]
		if v == "" {
			return nil, fmt.Errorf("prefill %q: no translation into %s", english, l)
		}
		m[l] = v
	}
	return json.Marshal(m)
}

// Draft is one UI string a reviewer wants a better version of.
type Draft struct {
	English string `json:"english"`
	Context string `json:"context,omitempty"`
	// Voice is the owner's livelier English, a guide to the tone.
	Voice string `json:"voice,omitempty"`
	// Current is what's shown today: the human version or the machine's.
	Current string `json:"current,omitempty"`
}
