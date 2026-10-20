// Package i18n resolves localised text stored as JSON objects keyed by language.
package i18n

import (
	"encoding/json"
	"strings"
)

// Fallback is used when neither the requested locale nor its base language is present.
const Fallback = "en"

// Pick returns the best match for locale from a {"en": "...", "ja": "..."} JSON object.
// It tries the exact tag, then the base language ("ja-JP" -> "ja"), then English,
// then any value, so a missing translation never produces an empty label.
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
	if v, ok := m[Fallback]; ok {
		return v
	}
	for _, v := range m {
		return v
	}
	return ""
}
