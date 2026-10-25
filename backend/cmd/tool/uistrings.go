package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// uiStrings checks the human versions in <dir>/human/<lang>.json against the
// plain English in <dir>/en.json and stamps new entries with the English they
// were written for (uistrings.Stamp). `make gen` runs it before copying the
// files into the API, so a hand-written entry only needs its "text".
func uiStrings(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: tool ui-strings <apps/mobile/src/i18n>")
	}
	dir := args[0]
	english, err := os.ReadFile(filepath.Join(dir, "en.json"))
	if err != nil {
		return err
	}
	if err := checkNotes(english, filepath.Join(dir, "notes.json")); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "human", "*.json"))
	if err != nil {
		return err
	}
	for _, f := range files {
		lang := strings.TrimSuffix(filepath.Base(f), ".json")
		if !i18n.Valid(lang) {
			return fmt.Errorf("%s: %q isn't an ISO 639-1 language code", f, lang)
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		out, err := uistrings.Stamp(english, raw)
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		if bytes.Equal(out, raw) {
			continue
		}
		if err := os.WriteFile(f, out, 0o644); err != nil {
			return err
		}
		slog.Info("stamped human versions", "file", f)
	}
	return nil
}

// checkNotes makes sure every note in notes.json belongs to a string in
// en.json, so a renamed key doesn't silently lose its note.
func checkNotes(englishRaw []byte, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var english, notes map[string]string
	if err := json.Unmarshal(englishRaw, &english); err != nil {
		return fmt.Errorf("en.json: %w", err)
	}
	if err := json.Unmarshal(raw, &notes); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for key, note := range notes {
		if _, ok := english[key]; !ok {
			return fmt.Errorf("%s: %s isn't in en.json", path, key)
		}
		if strings.TrimSpace(note) == "" {
			return fmt.Errorf("%s: %s has an empty note", path, key)
		}
	}
	return nil
}
