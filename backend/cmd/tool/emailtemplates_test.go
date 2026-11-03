package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/platform/config"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

// fakeTranslator marks each string with its language, counts its calls and
// can fail. Its Name is the model under which its cache rows are stored.
type fakeTranslator struct {
	calls atomic.Int32
	err   error
}

const fakeModel = "zz-tool-fake"

func (f *fakeTranslator) Name() string { return fakeModel }

func (f *fakeTranslator) Translate(_ context.Context, lang string, items []translate.Item) ([]string, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = "[" + lang + "] " + it.Text
	}
	return out, nil
}

func TestIntegrationEmailTemplates(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	fake := &fakeTranslator{}
	old := newTranslator
	newTranslator = func(string, string, string) (translate.Translator, error) { return fake, nil }
	t.Cleanup(func() { newTranslator = old })
	useToolCfg(t, config.Tool{})

	// Strings unique to this test, so its cache rows can't be anyone else's.
	if _, err := pool.Exec(ctx, "delete from app.translations where model = $1", fakeModel); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "delete from app.translations where model = $1", fakeModel); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	dir := t.TempDir()
	source := filepath.Join(dir, "source.json")
	cfgPath := filepath.Join(dir, "config.toml")
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, s string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		return string(b)
	}
	write(source, `{"languages":["ja"],"strings":{"subject":"zz-tool subject","heading":"zz-tool heading","expiry":"zz-tool expiry","ignore":"zz-tool ignore"}}`)
	const cfg = `[auth.email.template.magic_link]
subject = '''old'''
content_path = "./supabase/templates/magic_link.html"

[auth.email.template.confirmation]
subject = '''old'''
content_path = "./supabase/templates/confirmation.html"
`
	write(cfgPath, cfg)
	args := []string{"-source", source, "-out", out, "-config", cfgPath}
	magic, confirmation := filepath.Join(out, "magic_link.html"), filepath.Join(out, "confirmation.html")

	// A failed translation writes nothing: no template, and the config as it was.
	fake.err = errors.New("quota")
	if err := emailTemplates(ctx, pool, args); err == nil || !strings.Contains(err.Error(), "nothing written") {
		t.Errorf("failed translation: err = %v; want \"nothing written\"", err)
	}
	if read(magic) != "" || read(confirmation) != "" || read(cfgPath) != cfg {
		t.Error("a failed translation changed files")
	}
	// So does a missing translator key.
	newTranslator = func(string, string, string) (translate.Translator, error) { return nil, nil }
	if err := emailTemplates(ctx, pool, args); err == nil {
		t.Error("no translator: want an error")
	}
	if read(magic) != "" || read(cfgPath) != cfg {
		t.Error("a missing key changed files")
	}
	newTranslator = func(string, string, string) (translate.Translator, error) { return fake, nil }

	// Success writes both templates (the same body) and both subjects.
	fake.err = nil
	if err := emailTemplates(ctx, pool, args); err != nil {
		t.Fatal(err)
	}
	body := read(magic)
	if body == "" || body != read(confirmation) {
		t.Fatal("want the same non-empty body in magic_link.html and confirmation.html")
	}
	if !strings.Contains(body, "[ja] zz-tool heading") {
		t.Errorf("the body lacks the Japanese translation")
	}
	newCfg := read(cfgPath)
	if strings.Contains(newCfg, "'''old'''") || strings.Count(newCfg, "[ja] zz-tool subject") != 2 {
		t.Errorf("config subjects not updated in both templates:\n%s", newCfg)
	}
	if !strings.Contains(newCfg, `content_path = "./supabase/templates/magic_link.html"`) {
		t.Error("the config lost its other lines")
	}

	// A second run reads the cache: no translator calls, the same files.
	calls := fake.calls.Load()
	if err := emailTemplates(ctx, pool, args); err != nil {
		t.Fatal(err)
	}
	if fake.calls.Load() != calls || read(magic) != body || read(cfgPath) != newCfg {
		t.Errorf("second run translated again (%d -> %d calls) or changed the files", calls, fake.calls.Load())
	}
}
