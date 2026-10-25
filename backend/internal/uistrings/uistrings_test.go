package uistrings

import (
	"context"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
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
	if len(c.keys) == 0 {
		t.Fatalf("empty catalogue: %+v", c)
	}
	for k := range c.notes {
		if _, ok := c.english[k]; !ok {
			t.Errorf("notes.json: %s isn't in en.json", k)
		}
	}
	if !strings.Contains(c.context("common.save"), "not saving money") || c.context("account.delete") != translateContext {
		t.Errorf("context: %q / %q", c.context("common.save"), c.context("account.delete"))
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
	c, err := parse([]byte(`{"hi":"Hello","who":"Signed in as {email}"}`), nil)
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

// The committed human versions: real languages, real keys, placeholders
// kept. The owner's English must also be current: a reworded English string
// fails here until its livelier version is checked (other languages' stale
// versions don't fail the build, since the owner can't judge them; they're
// skipped when shown, and the translation editor lists them).
func TestEmbeddedHuman(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.human["en"]; !ok {
		t.Error("human/en.json is missing (make gen copies it)")
	}
	for lang, versions := range c.human {
		if !i18n.Valid(lang) {
			t.Errorf("human/%s.json: not an ISO 639-1 code", lang)
		}
		for key, v := range versions {
			english, ok := c.english[key]
			switch {
			case !ok:
				t.Errorf("human/%s.json: %s isn't in en.json", lang, key)
			case !SamePlaceholders(english, v.Text):
				t.Errorf("human/%s.json: %s must keep the placeholders of %q", lang, key, english)
			case v.Source == "":
				t.Errorf("human/%s.json: %s has no source; run make gen", lang, key)
			case lang == i18n.Authoring && v.Source != SourceHash(english):
				t.Errorf("human/en.json: %s was written for an earlier English text; it's now %q. "+
					"Check the livelier version still says the same, then delete its \"source\" and run make gen.", key, english)
			}
		}
	}
}

func TestHumanVersions(t *testing.T) {
	en := []byte(`{"hi":"Hello","who":"Signed in as {email}","bye":"Goodbye"}`)
	human := map[string][]byte{
		"en": []byte(`{"hi":{"text":"Hey there","source":"` + SourceHash("Hello") + `"},` +
			`"bye":{"text":"See ya","source":"` + SourceHash("Good-bye") + `"}}`), // written for older English: stale
		"ja": []byte(`{"who":{"text":"{email} でログイン中","source":"` + SourceHash("Signed in as {email}") + `"},` +
			`"hi":{"text":"やあ","source":"` + SourceHash("Hello") + `"}}`),
		"fr": []byte(`{"who":{"text":"Connecté","source":"` + SourceHash("Signed in as {email}") + `"}}`), // lost {email}
	}
	c, err := parse(en, human)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store := func() *translate.Service {
		return &translate.Service{Store: &memStore{rows: map[string]string{}}, Translator: mangler{}}
	}

	got, complete := c.In(ctx, store(), "en")
	if !complete || got["hi"] != "Hey there" || got["bye"] != "Goodbye" || got["who"] != "Signed in as {email}" {
		t.Errorf("English: %v %v", got, complete)
	}

	// Human versions where current; the machine translates the rest.
	got, complete = c.In(ctx, store(), "ja")
	if !complete || got["who"] != "{email} でログイン中" || got["hi"] != "やあ" || got["bye"] != "GOODBYE" {
		t.Errorf("Japanese: %v %v", got, complete)
	}
	// Nothing left for a translator: complete without one.
	jaOnly, _ := parse([]byte(`{"hi":"Hello"}`), map[string][]byte{"ja": human["ja"]})
	if got, complete := jaOnly.In(ctx, &translate.Service{Store: &memStore{rows: map[string]string{}}}, "ja"); !complete || got["hi"] != "やあ" {
		t.Errorf("all human: %v %v", got, complete)
	}
	// A human version that lost a placeholder isn't used.
	if got, _ := c.In(ctx, &translate.Service{Store: &memStore{rows: map[string]string{}}}, "fr"); got["who"] != "" {
		t.Errorf("broken human version shown: %q", got["who"])
	}

	if s := c.Text(ctx, nil, "en", "hi"); s != "Hey there" {
		t.Errorf("Text en: %q", s)
	}
	if s := c.Text(ctx, nil, "en", "bye"); s != "Goodbye" {
		t.Errorf("Text en, stale: %q", s)
	}
	if s := c.Text(ctx, nil, "ja", "hi"); s != "やあ" {
		t.Errorf("Text ja: %q", s)
	}
	// The fallback is the plain English, not the livelier one.
	if s := c.Text(ctx, nil, "de", "hi"); s != "Hello" {
		t.Errorf("Text fallback: %q", s)
	}
}

// The app checks its copy with the version it has: unchanged text costs no
// strings, and any change (here a reviewer's correction) sends them again.
func TestGetStringsKnownVersion(t *testing.T) {
	c, err := parse([]byte(`{"hi":"Hello","bye":"Goodbye"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	store := &memStore{rows: map[string]string{}}
	h := &Handler{Catalog: c, Translations: &translate.Service{Store: store, Translator: mangler{}}}
	get := func(known string) *isshov1.GetStringsResponse {
		t.Helper()
		res, err := h.GetStrings(context.Background(), connect.NewRequest(&isshov1.GetStringsRequest{Language: "ja", KnownVersion: known}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg
	}

	first := get("")
	if !first.Complete || first.Unchanged || first.Strings["hi"] != "HELLO" || first.CatalogVersion == "" {
		t.Fatalf("first: %+v", first)
	}
	if again := get(first.CatalogVersion); !again.Unchanged || len(again.Strings) != 0 || again.CatalogVersion != first.CatalogVersion {
		t.Errorf("unchanged: %+v", again)
	}
	if old := get("0123456789ab"); old.Unchanged || old.Strings["hi"] != "HELLO" {
		t.Errorf("old version: %+v", old)
	}

	// A correction in the cache (as ReviewTranslation makes) changes the version.
	for k := range store.rows {
		if store.rows[k] == "HELLO" {
			store.rows[k] = "やあ"
		}
	}
	if fixed := get(first.CatalogVersion); fixed.Unchanged || fixed.Strings["hi"] != "やあ" || fixed.CatalogVersion == first.CatalogVersion {
		t.Errorf("after a correction: %+v", fixed)
	}

	// An incomplete catalogue is never "unchanged": the app keeps asking.
	slow := &Handler{Catalog: c, Translations: &translate.Service{Store: &memStore{rows: map[string]string{}}}}
	res, err := slow.GetStrings(context.Background(), connect.NewRequest(&isshov1.GetStringsRequest{Language: "ja", KnownVersion: Version(map[string]string{})}))
	if err != nil || res.Msg.Unchanged || res.Msg.Complete {
		t.Errorf("incomplete: %+v %v", res.Msg, err)
	}
}

func TestStamp(t *testing.T) {
	en := []byte(`{"hi":"Hello","who":"Signed in as {email}"}`)
	old := SourceHash("Hi")
	got, err := Stamp(en, []byte(`{"who":{"text":"You're in as {email} & <ok>"},"hi":{"text":"Hey","source":"`+old+`"}}`))
	if err != nil {
		t.Fatal(err)
	}
	// New entries get today's English; existing sources are kept (refreshing
	// them would hide a change). One line per string, sorted, unescaped.
	want := "{\n" +
		`  "hi": {"text": "Hey", "source": "` + old + `"},` + "\n" +
		`  "who": {"text": "You're in as {email} & <ok>", "source": "` + SourceHash("Signed in as {email}") + `"}` + "\n" +
		"}\n"
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if again, _ := Stamp(en, got); string(again) != want {
		t.Errorf("not stable:\n%s", again)
	}
	if got, _ := Stamp(en, []byte(`{}`)); string(got) != "{}\n" {
		t.Errorf("empty: %q", got)
	}
	for _, bad := range []string{
		`{"nope":{"text":"x"}}`,
		`{"hi":{"text":" "}}`,
		`{"who":{"text":"You're in"}}`,
		`{"hi":"Hey"}`,
	} {
		if _, err := Stamp(en, []byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
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
		if got := SamePlaceholders(tc.en, tc.tr); got != tc.want {
			t.Errorf("SamePlaceholders(%q, %q) = %v", tc.en, tc.tr, got)
		}
	}
}
