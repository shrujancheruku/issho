package i18n

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestPick(t *testing.T) {
	raw := []byte(`{"en":"Photo walk","ja":"フォトウォーク"}`)
	tests := []struct{ locale, want string }{
		{"ja", "フォトウォーク"},
		{"ja-JP", "フォトウォーク"},
		{"en-GB", "Photo walk"},
		{"es", "Photo walk"},
		{"", "Photo walk"},
	}
	for _, tt := range tests {
		if got := Pick(raw, tt.locale); got != tt.want {
			t.Errorf("Pick(%q) = %q, want %q", tt.locale, got, tt.want)
		}
	}
	if got := Pick([]byte(`{"ja":"花見"}`), "en"); got != "花見" {
		t.Errorf("fallback to any value: got %q", got)
	}
	if got := Pick([]byte(`not json`), "en"); got != "" {
		t.Errorf("invalid json: got %q", got)
	}
}

func TestForApp(t *testing.T) {
	if p, s := ForApp("ja", "fr"); p != "ja" || s != "fr" {
		t.Errorf("got %q, %q", p, s)
	}
	if _, s := ForApp("ja", "ja"); s != "" {
		t.Errorf("secondary equal to primary must be dropped, got %q", s)
	}
}

func TestPair(t *testing.T) {
	tests := []struct{ primary, secondary, wantP, wantS string }{
		{"カラオケ", "Karaoke", "カラオケ", "Karaoke"},
		{"Karaoke", "Karaoke", "Karaoke", ""}, // identical: shown once
		{"カラオケ", "", "カラオケ", ""},              // no secondary
		{"", "カラオケ", "カラオケ", ""},              // missing primary: the secondary alone, not English
		{"", "", "Karaoke (en)", ""},          // nothing yet: English
	}
	for _, tt := range tests {
		p, s := Pair(tt.primary, tt.secondary, "Karaoke (en)")
		if p != tt.wantP || s != tt.wantS {
			t.Errorf("Pair(%q, %q) = %q, %q; want %q, %q", tt.primary, tt.secondary, p, s, tt.wantP, tt.wantS)
		}
	}
}

func TestValid(t *testing.T) {
	for _, ok := range []string{"ja", "es", "ko", "zh"} {
		if !Valid(ok) {
			t.Errorf("Valid(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "JA", "ja-JP", "japanese", "j", "zz", "fil"} {
		if Valid(bad) {
			t.Errorf("Valid(%q) = true", bad)
		}
	}
}

// TestAppLanguageCodes keeps the app's copy of the language list
// (apps/mobile/src/i18n/device.ts), used to pick a supported device
// language, identical to the one the server validates against.
func TestAppLanguageCodes(t *testing.T) {
	src, err := os.ReadFile("../../../../apps/mobile/src/i18n/device.ts")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile("(?s)languageCodes = new Set\\(\\s*`([^`]+)`").FindSubmatch(src)
	if m == nil {
		t.Fatal("language list not found in device.ts")
	}
	app := strings.Fields(string(m[1]))
	slices.Sort(app)
	if !slices.Equal(app, Codes()) {
		t.Errorf("app and server language lists differ:\napp:    %v\nserver: %v", app, Codes())
	}
}

func TestDisplay(t *testing.T) {
	for tag, want := range map[string]string{
		"ja-JP": "ja", "en": "en", " ES_mx ": "es", "zh": "zh", "zh-CN": "zh", "zh-Hans-CN": "zh", "zh-SG": "zh",
		"zh-TW": "zh-Hant", "zh-HK": "zh-Hant", "zh-MO": "zh-Hant", "zh-Hant": "zh-Hant", "zh-Hant-TW": "zh-Hant",
		"zh_TW": "zh-Hant", "": "",
	} {
		if got := Display(tag); got != want {
			t.Errorf("Display(%q) = %q, want %q", tag, got, want)
		}
	}
	if !ValidDisplay("zh-Hant") || !ValidDisplay("ja") || ValidDisplay("zh-Hans") || Valid("zh-Hant") {
		t.Error("ValidDisplay accepts ISO 639-1 codes and zh-Hant only; Valid stays ISO 639-1")
	}
}
