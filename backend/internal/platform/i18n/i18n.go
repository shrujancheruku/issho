// Package i18n implements Issho's language rules.
//
// Languages are symmetric: no language is privileged. People can speak any
// language and events can be held in any language. Everything shown to a
// person is in up to two languages, a primary and an optional secondary,
// chosen per context by ForEvent and ForApp. Localised text is stored as JSON
// objects keyed by language code: {"en": "...", "ja": "...", "es": "..."}.
package i18n

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
)

// Translated lists the languages our own content (activity catalogue, venue
// categories, app strings) is fully translated into (MVP). People and events
// may use any language; for others, text falls back as described on Pair.
var Translated = []string{"en", "ja", "es", "fr"}

// Authoring is the language every piece of our own data must exist in, so a
// fallback always exists. It's an authoring rule, not a display preference.
const Authoring = "en"

// IsTranslated reports whether our content is fully translated into lang.
func IsTranslated(lang string) bool { return slices.Contains(Translated, lang) }

// ISO 639-1/639-3 base codes ("ja", "es", "fil"), lowercase, no region:
// regional variants don't change which language someone can read.
var codeRE = regexp.MustCompile(`^[a-z]{2,3}$`)

// Valid reports whether lang is a well-formed base language code.
func Valid(lang string) bool { return codeRE.MatchString(lang) }

// ForEvent picks the two languages to show a user for an event.
//
//   - Primary is the event's language: what they'll hear when they get there.
//   - Secondary is the event's second language if it has one (e.g. a JP–ES
//     language exchange shows Japanese and Spanish).
//   - Otherwise it's the user's primary language, so they can understand an
//     event held in a language they're still learning; failing that (the
//     event is in their primary language), the user's own secondary choice.
//
// secondary is "" when it would repeat primary.
func ForEvent(eventPrimary, eventSecondary, userPrimary, userSecondary string) (primary, secondary string) {
	primary = eventPrimary
	for _, l := range []string{eventSecondary, userPrimary, userSecondary} {
		if l != "" && l != primary {
			return primary, l
		}
	}
	return primary, ""
}

// ForApp picks the two languages for everything outside an event: the user's
// primary language and their optional secondary choice.
func ForApp(userPrimary, userSecondary string) (primary, secondary string) {
	if userSecondary == userPrimary {
		userSecondary = ""
	}
	return userPrimary, userSecondary
}

// Pair resolves a localised JSON object into the text for a primary and a
// secondary language.
//
//   - If the primary translation is missing but the secondary exists, the
//     secondary is shown alone: better one language the person reads than a
//     fallback they may not.
//   - If both are missing, the authoring language (then any value) is used,
//     so a label is never blank.
//   - secondary is "" when there's no translation or it's identical to the
//     primary text (e.g. "Karaoke" in English and Spanish), so callers never
//     show the same words twice.
func Pair(raw []byte, primaryLang, secondaryLang string) (primary, secondary string) {
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return "", ""
	}
	primary, secondary = m[primaryLang], ""
	if secondaryLang != "" && secondaryLang != primaryLang {
		secondary = m[secondaryLang]
	}
	switch {
	case primary == "" && secondary != "":
		primary, secondary = secondary, ""
	case primary == "":
		primary = Pick(raw, Authoring)
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
