package i18n

import "testing"

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

func TestForEvent(t *testing.T) {
	tests := []struct {
		name                                  string
		evPrimary, evSecondary, user, userSec string
		wantPrimary, wantSecondary            string
	}{
		{"JP–ES exchange: both event languages", "ja", "es", "fr", "", "ja", "es"},
		{"JP-only event: the user's primary language", "ja", "", "es", "", "ja", "es"},
		{"event in the user's language: their secondary choice", "ja", "", "ja", "en", "ja", "en"},
		{"event in the user's language, no choice: one language", "ja", "", "ja", "", "ja", ""},
		{"any language works", "ko", "", "pt", "", "ko", "pt"},
		{"never repeats the primary", "es", "es", "es", "es", "es", ""},
	}
	for _, tt := range tests {
		p, s := ForEvent(tt.evPrimary, tt.evSecondary, tt.user, tt.userSec)
		if p != tt.wantPrimary || s != tt.wantSecondary {
			t.Errorf("%s: got %q, %q; want %q, %q", tt.name, p, s, tt.wantPrimary, tt.wantSecondary)
		}
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
	raw := []byte(`{"en":"Karaoke","ja":"カラオケ","es":"Karaoke"}`)
	tests := []struct{ primary, secondary, wantP, wantS string }{
		{"ja", "es", "カラオケ", "Karaoke"},
		{"es", "ja", "Karaoke", "カラオケ"},
		{"en", "es", "Karaoke", ""}, // identical: shown once
		{"ja", "fr", "カラオケ", ""},    // missing secondary
		{"fr", "ja", "カラオケ", ""},    // missing primary: the secondary alone, not a fallback
		{"ko", "pt", "Karaoke", ""}, // neither translated: authoring language
		{"ja", "", "カラオケ", ""},
	}
	for _, tt := range tests {
		p, s := Pair(raw, tt.primary, tt.secondary)
		if p != tt.wantP || s != tt.wantS {
			t.Errorf("Pair(%q, %q) = %q, %q; want %q, %q", tt.primary, tt.secondary, p, s, tt.wantP, tt.wantS)
		}
	}
}

func TestValid(t *testing.T) {
	for _, ok := range []string{"ja", "es", "fil"} {
		if !Valid(ok) {
			t.Errorf("Valid(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "JA", "ja-JP", "japanese", "j"} {
		if Valid(bad) {
			t.Errorf("Valid(%q) = true", bad)
		}
	}
}
