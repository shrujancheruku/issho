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
