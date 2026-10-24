package curation

import (
	"regexp"
)

// flags are kinds of place discovery lets through under a valid primary
// type (ROADMAP risks): Japanese snack bars and hostess bars are typed
// `bar`, and a tattoo studio came in as `art_studio`. Google's English
// name for a Japanese place is often still in Japanese, so both are
// listed, with the hiragana and old-style (スナツク) spellings seen in
// Tokyo. A match only asks for a closer look: "lounge" also catches hotel
// lounges.
var flags = []struct {
	re     *regexp.Regexp
	reason string
}{
	{regexp.MustCompile(`(?i)スナック|スナツク|すなっく|\bsnack\b`), "snack bar?"},
	{regexp.MustCompile(`(?i)キャバ|ガールズ\s*バー|ホスト|ホステス|フィリピン\s*パブ|ラウンジ|\b(girls'?\s*bar|cabaret|hostess|host\s*club|lounge)\b`), "hostess bar?"},
	{regexp.MustCompile(`(?i)タトゥー|刺青|\btattoo`), "tattoo studio?"},
}

// Flag says why a place's name suggests a closer look, or "".
func Flag(name string) string {
	for _, f := range flags {
		if f.re.MatchString(name) {
			return f.reason
		}
	}
	return ""
}
