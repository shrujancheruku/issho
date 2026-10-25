package translators

import "testing"

func TestNew(t *testing.T) {
	for _, tc := range []struct {
		kind, google, gemini string
		want                 string // Name(), "" for none
		err                  bool
	}{
		{"", "g", "m", "google-translate-v2", false},
		{"google", "g", "", "google-translate-v2", false},
		{"gemini", "g", "m", "gemini-3.8-flash", false},
		{"gemini", "g", "", "", false}, // its key missing: translation off, not Google
		{"google", "", "m", "", false},
		{"deepl", "g", "m", "", true},
	} {
		tr, err := New(tc.kind, tc.google, tc.gemini)
		if (err != nil) != tc.err {
			t.Errorf("%+v: err %v", tc, err)
			continue
		}
		got := ""
		if tr != nil {
			got = tr.Name()
		}
		if got != tc.want {
			t.Errorf("%+v: got %q", tc, got)
		}
	}
}
