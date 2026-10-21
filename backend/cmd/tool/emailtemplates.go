package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/emailtemplates"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	gtranslate "github.com/shrujancheruku/issho/backend/internal/translate/google"
)

// emailTemplates regenerates the sign-in email templates in every language
// listed in the source, through the translation cache (so a re-run only
// translates what changed), and writes them only if every language succeeded.
func emailTemplates(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("email-templates", flag.ExitOnError)
	source := fs.String("source", "../supabase/templates/source.json", "English source and language list")
	outDir := fs.String("out", "../supabase/templates", "where magic_link.html and confirmation.html go")
	config := fs.String("config", "../supabase/config.toml", "Supabase config whose template subjects are updated")
	_ = fs.Parse(args)

	src, err := emailtemplates.ReadSource(*source)
	if err != nil {
		return err
	}
	key := os.Getenv("GOOGLE_TRANSLATE_API_KEY")
	if key == "" {
		return errors.New("GOOGLE_TRANSLATE_API_KEY is required: the templates are machine-translated (see docs/google-maps.md)")
	}
	// A tool can wait for slow first translations; requests can't.
	svc := &translate.Service{Store: dbgen.New(pool), Translator: gtranslate.New(key), Wait: 2 * time.Minute}

	texts := make([]translate.Text, len(emailtemplates.Keys))
	for i, k := range emailtemplates.Keys {
		raw, _ := json.Marshal(map[string]string{i18n.Authoring: src.Strings[k]})
		texts[i] = translate.Text{Raw: raw, Context: "text in a short email that gives someone a sign-in code for a mobile app"}
	}
	tr := map[string]emailtemplates.Text{}
	for _, lang := range src.Languages {
		got := svc.Resolve(ctx, lang, texts)
		t := emailtemplates.Text{}
		for i, k := range emailtemplates.Keys {
			if got[i] == "" {
				return fmt.Errorf("%s: no translation for %q yet; nothing written (re-run)", lang, k)
			}
			t[k] = got[i]
		}
		tr[lang] = t
	}

	body, subject, err := emailtemplates.Render(src, tr)
	if err != nil {
		return err
	}
	cfg, err := os.ReadFile(*config)
	if err != nil {
		return err
	}
	newCfg, err := emailtemplates.SetSubjects(string(cfg), subject)
	if err != nil {
		return err
	}
	// Supabase sends "confirmation" to new users and "magic_link" to existing
	// ones; both carry the same code.
	for _, name := range []string{"magic_link.html", "confirmation.html"} {
		if err := os.WriteFile(filepath.Join(*outDir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(*config, []byte(newCfg), 0o644); err != nil {
		return err
	}
	slog.Info("email templates written", "languages", len(src.Languages)+1, "bytes", len(body))
	return nil
}
