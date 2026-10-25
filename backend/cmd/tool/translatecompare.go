package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/shrujancheruku/issho/backend/internal/translate"
	"github.com/shrujancheruku/issho/backend/internal/translate/gemini"
	"github.com/shrujancheruku/issho/backend/internal/translate/google"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// compareRow is one UI string in every language, from each translator.
type compareRow struct {
	Key     string            `json:"key"`
	English string            `json:"english"`
	Note    string            `json:"note,omitempty"`
	Google  map[string]string `json:"google"`
	Gemini  map[string]string `json:"gemini"`
	// Lost lists "<translator>:<lang>" whose translation dropped or changed a
	// {placeholder}; the app would show English there instead.
	Lost []string `json:"lost,omitempty"`
}

// translateCompare translates the whole UI catalogue with Google
// Translation Basic and with Gemini (notes and the translation guide
// included), straight from the translators without the cache, and writes
// them side by side for review. Nothing is stored. Needs
// GOOGLE_TRANSLATE_API_KEY and GOOGLE_GEMINI_API_KEY.
func translateCompare(args []string) error {
	fs := flag.NewFlagSet("translate-compare", flag.ExitOnError)
	langs := fs.String("langs", "ja,es,pl,zh", "languages to compare")
	out := fs.String("out", "translate-compare.json", "where the comparison goes")
	_ = fs.Parse(args)
	gKey, mKey := os.Getenv("GOOGLE_TRANSLATE_API_KEY"), os.Getenv("GOOGLE_GEMINI_API_KEY")
	if gKey == "" || mKey == "" {
		return errors.New("GOOGLE_TRANSLATE_API_KEY and GOOGLE_GEMINI_API_KEY are both required")
	}
	catalog, err := uistrings.Load()
	if err != nil {
		return err
	}
	keys, items := catalog.Items()
	rows := make([]compareRow, len(keys))
	for i, k := range keys {
		rows[i] = compareRow{Key: k, English: items[i].Text, Note: catalog.Note(k), Google: map[string]string{}, Gemini: map[string]string{}}
	}

	ctx := context.Background()
	translators := []struct {
		name string
		tr   translate.Translator
		into func(*compareRow) map[string]string
	}{
		{"google", google.New(gKey), func(r *compareRow) map[string]string { return r.Google }},
		{"gemini", gemini.New(mKey), func(r *compareRow) map[string]string { return r.Gemini }},
	}
	for _, lang := range strings.Split(*langs, ",") {
		lang = strings.TrimSpace(lang)
		for _, t := range translators {
			got, err := t.tr.Translate(ctx, lang, items)
			if err != nil {
				return fmt.Errorf("%s %s: %w", t.name, lang, err)
			}
			for i := range rows {
				t.into(&rows[i])[lang] = got[i]
				if !uistrings.SamePlaceholders(rows[i].English, got[i]) {
					rows[i].Lost = append(rows[i].Lost, t.name+":"+lang)
				}
			}
			slog.Info("translated", "translator", t.name, "lang", lang, "strings", len(got))
		}
	}
	b, err := json.MarshalIndent(map[string]any{"langs": strings.Split(*langs, ","), "rows": rows}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(*out, b, 0o644)
}
