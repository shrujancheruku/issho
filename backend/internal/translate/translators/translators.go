// Package translators picks the machine translator from configuration
// (TRANSLATOR), so the API, the worker and the tools all use the same one.
package translators

import (
	"fmt"

	"github.com/shrujancheruku/issho/backend/internal/translate"
	"github.com/shrujancheruku/issho/backend/internal/translate/gemini"
	"github.com/shrujancheruku/issho/backend/internal/translate/google"
)

// New returns the translator named by kind: "google" (Cloud Translation
// Basic, the default) or "gemini" (reads each string's context and the
// translation guide). It returns nil, with no error, when that translator's
// key is missing: machine translation is then off.
func New(kind, googleKey, geminiKey string) (translate.Translator, error) {
	switch kind {
	case "", "google":
		if googleKey == "" {
			return nil, nil
		}
		return google.New(googleKey), nil
	case "gemini":
		if geminiKey == "" {
			return nil, nil
		}
		return gemini.New(geminiKey), nil
	default:
		return nil, fmt.Errorf("TRANSLATOR must be google or gemini, not %q", kind)
	}
}
