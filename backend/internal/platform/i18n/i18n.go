// Package i18n implements Issho's language rules.
//
// Languages are symmetric: no language is privileged. People can speak any
// language and events can be held in any language. Everything shown to a
// person is in up to two languages: their primary and an optional secondary
// (ForApp), events included. Localised text is stored as JSON objects keyed
// by language code: {"en": "...", "ja": "...", "es": "..."}.
package i18n

import (
	"encoding/json"
	"slices"
	"strings"

	"golang.org/x/text/language"
)

// Authoring is the one language every piece of stored text must have. All
// other languages are translated on demand (internal/translate). It's a
// storage rule, not a display preference: nobody is shown English unless
// nothing else is available.
const Authoring = "en"

// iso6391 lists the ISO 639-1 language codes. Only these are accepted, so a
// client can't make the server pay to translate into made-up languages.
var iso6391 = strings.Fields(`aa ab ae af ak am an ar as av ay az ba be bg bi bm bn bo br bs ca ce ch co
cr cs cu cv cy da de dv dz ee el en eo es et eu fa ff fi fj fo fr fy ga gd gl gn gu gv ha he hi ho hr
ht hu hy hz ia id ie ig ii ik io is it iu ja jv ka kg ki kj kk kl km kn ko kr ks ku kv kw ky la lb lg
li ln lo lt lu lv mg mh mi mk ml mn mr ms mt my na nb nd ne ng nl nn no nr nv ny oc oj om or os pa pi
pl ps pt qu rm rn ro ru rw sa sc sd se sg si sk sl sm sn so sq sr ss st su sv sw ta te tg th ti tk tl
tn to tr ts tt tw ty ug uk ur uz ve vi vo wa wo xh yi yo za zh zu`)

// Valid reports whether lang is an ISO 639-1 code ("ja", "es"), lowercase,
// without a region: regional variants don't change which language someone
// can read.
func Valid(lang string) bool { return slices.Contains(iso6391, lang) }

// Codes returns every valid language code, sorted.
func Codes() []string { return slices.Clone(iso6391) }

// TraditionalChinese is the one display language that isn't an ISO 639-1
// code: Chinese written in Traditional characters (Taiwan, Hong Kong,
// Macau). "zh" is Simplified. Spoken languages and matching keep "zh" for
// both: it's how text is written, not who can talk to whom.
const TraditionalChinese = "zh-Hant"

// ValidDisplay reports whether lang can be a display language: an ISO 639-1
// code, or TraditionalChinese.
func ValidDisplay(lang string) bool { return Valid(lang) || lang == TraditionalChinese }

// Display reduces a BCP 47 tag from a device or client to a display
// language: its base language ("ja-JP" -> "ja"), except Chinese written in
// Traditional characters ("zh-TW", "zh-HK", "zh-Hant-TW" -> "zh-Hant"). The
// result may still be invalid; check it with ValidDisplay.
func Display(tag string) string {
	tag = strings.TrimSpace(tag)
	base, _, _ := strings.Cut(strings.ToLower(strings.ReplaceAll(tag, "_", "-")), "-")
	if base != "zh" || tag == "zh" {
		return base
	}
	// The script is explicit (zh-Hant) or follows from the region (zh-TW).
	if script, _ := language.Make(tag).Script(); script.String() == "Hant" {
		return TraditionalChinese
	}
	return base
}

// ForApp picks the two languages a person reads everything in, events
// included: their primary language and their optional secondary choice.
func ForApp(userPrimary, userSecondary string) (primary, secondary string) {
	if userSecondary == userPrimary {
		userSecondary = ""
	}
	return userPrimary, userSecondary
}

// Pair decides what to display from text already resolved into the primary
// and secondary languages ("" = not available) and the stored English text.
//
//   - If the primary is missing but the secondary exists, the secondary is
//     shown alone: better one language the person reads than a fallback they
//     may not.
//   - If both are missing (translation off or still running), the English
//     text is shown, so a label is never blank.
//   - secondary is "" when it's identical to the primary (e.g. "Karaoke" in
//     English and Spanish), so callers never show the same words twice.
func Pair(primary, secondary, english string) (string, string) {
	switch {
	case primary == "" && secondary != "":
		primary, secondary = secondary, ""
	case primary == "":
		primary = english
	}
	if secondary == primary {
		secondary = ""
	}
	return primary, secondary
}

// Pick returns the best single match for locale from a localised JSON object:
// the exact tag, then the base language ("ja-JP" -> "ja"), then the authoring
// language, then any value, so a missing translation never produces an empty
// label. Prefer Pair for anything shown to users.
func Pick(raw []byte, locale string) string {
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return ""
	}
	if v, ok := m[locale]; ok {
		return v
	}
	if base, _, found := strings.Cut(locale, "-"); found {
		if v, ok := m[base]; ok {
			return v
		}
	}
	if v, ok := m[Authoring]; ok {
		return v
	}
	for _, v := range m {
		return v
	}
	return ""
}
