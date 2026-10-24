package curation

import "testing"

func TestFlag(t *testing.T) {
	for name, want := range map[string]string{
		"スナック ゆき":               "snack bar?",
		"Snack Bar Mimi":        "snack bar?",
		"すなっく 萬":                "snack bar?",
		"ぱぶスナツクアミアミ":            "snack bar?",
		"SNACK 蘭":               "snack bar?",
		"Snacks & Sweets Kiosk": "",
		"Girls Bar Ribbon":      "hostess bar?",
		"ガールズバー ルル":             "hostess bar?",
		"キャバクラ Club A":          "hostess bar?",
		"フィリピンパブ マニラ":           "hostess bar?",
		"Lounge Tsuki":          "hostess bar?",
		"Hub Koenji":            "",
		"Ink Tattoo Studio":     "tattoo studio?",
		"Kayaba Coffee":         "",
		"井の頭恩賜公園":               "",
	} {
		if got := Flag(name); got != want {
			t.Errorf("Flag(%q) = %q, want %q", name, got, want)
		}
	}
}
