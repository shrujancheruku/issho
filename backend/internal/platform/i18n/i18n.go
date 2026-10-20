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
	"slices"
	"strings"
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
