package uistrings

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

type memStore struct {
	mu   sync.Mutex
	rows map[string]string
}

func (m *memStore) GetTranslations(_ context.Context, arg dbgen.GetTranslationsParams) ([]dbgen.GetTranslationsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []dbgen.GetTranslationsRow
	for _, h := range arg.SourceHashes {
		if t, ok := m.rows[arg.Lang+"/"+h]; ok {
			out = append(out, dbgen.GetTranslationsRow{SourceHash: h, Text: t})
		}
	}
	return out, nil
}

func (m *memStore) InsertTranslation(_ context.Context, arg dbgen.InsertTranslationParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[arg.Lang+"/"+arg.SourceHash] = arg.Text
	return nil
}

// mangler "translates" by upper-casing, which breaks {placeholders}.
type mangler struct{}

func (mangler) Name() string { return "mangler" }
func (mangler) Translate(_ context.Context, _ string, items []translate.Item) ([]string, error) {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = strings.ToUpper(it.Text)
	}
	return out, nil
}

func TestEmbeddedCatalog(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.keys) == 0 || c.version == "" {
		t.Fatalf("empty catalogue: %+v", c)
	}
	for k, v := range c.english {
		if strings.TrimSpace(v) == "" {
			t.Errorf("%s: empty text", k)
		}
		// Only {lowerCamel} placeholders survive the check in In; anything
		// else (e.g. "{{name}}", "%s") would reach users untranslated.
		if strings.Count(v, "{") != len(placeholder.FindAllString(v, -1)) {
			t.Errorf("%s: placeholders must look like {name}: %q", k, v)
		}
	}
}

func TestIn(t *testing.T) {
	c, err := parse([]byte(`{"hi":"Hello","who":"Signed in as {email}"}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	en, complete := c.In(ctx, &translate.Service{Store: &memStore{rows: map[string]string{}}}, "en")
	if !complete || en["who"] != "Signed in as {email}" {
		t.Errorf("English: %v %v", en, complete)
	}

	// No translator: nothing to show yet, ask again later.
	off, complete := c.In(ctx, &translate.Service{Store: &memStore{rows: map[string]string{}}}, "ja")
	if complete || len(off) != 0 {
		t.Errorf("translator off: %v %v", off, complete)
	}

	// A translation that lost its placeholder is dropped (the app shows
	// English for it), but it's final, so the result is complete.
	got, complete := c.In(ctx, &translate.Service{Store: &memStore{rows: map[string]string{}}, Translator: mangler{}}, "fr")
	if !complete || got["hi"] != "HELLO" {
		t.Errorf("got %v %v", got, complete)
	}
	if _, ok := got["who"]; ok {
		t.Errorf("broken placeholder kept: %q", got["who"])
	}
}

func TestSamePlaceholders(t *testing.T) {
	for _, tc := range []struct {
		en, tr string
		want   bool
	}{
		{"Hello", "こんにちは", true},
		{"Code sent to {email}", "{email} にコードを送信しました", true},
		{"{a} and {b}", "{b} y {a}", true},
		{"Code sent to {email}", "Code envoyé à {courriel}", false},
		{"Code sent to {email}", "Code envoyé", false},
	} {
		if got := samePlaceholders(tc.en, tc.tr); got != tc.want {
			t.Errorf("samePlaceholders(%q, %q) = %v", tc.en, tc.tr, got)
		}
	}
}
