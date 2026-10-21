package catalog

import (
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

func TestLanguages(t *testing.T) {
	ja := map[string][2]string{}
	langs := Languages("ja")
	if len(langs) != len(i18n.Codes()) {
		t.Fatalf("%d languages, want every ISO 639-1 code (%d)", len(langs), len(i18n.Codes()))
	}
	for _, l := range langs {
		if l.Autonym == "" || l.Name == "" {
			t.Errorf("%s: missing a name: %+v", l.Code, l)
		}
		ja[l.Code] = [2]string{l.Autonym, l.Name}
	}
	for code, want := range map[string][2]string{
		"es": {"español", "スペイン語"},
		"ja": {"日本語", "日本語"},
		"la": {"Latin", "ラテン語"}, // no CLDR autonym: English instead
	} {
		if ja[code] != want {
			t.Errorf("%s = %v, want %v", code, ja[code], want)
		}
	}
}

func TestStored(t *testing.T) {
	raw := []byte(`{"en":"Kichijoji","ja":"吉祥寺"}`)
	if stored(raw, "ja") != "吉祥寺" || stored(raw, "es") != "" || stored(raw, "") != "" {
		t.Error("stored must return only text stored in exactly that language")
	}
}
