package uistrings

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Human is a person's version of one UI string in one language: the owner's
// livelier English (human/en.json), or a translation someone wrote or
// approved. It replaces that string only; every other string keeps the plain
// English or its machine translation, so a file lists just the strings that
// differ.
type Human struct {
	Text string `json:"text"`
	// Source is SourceHash of the plain English it was written for. When the
	// English changes, the version is stale and isn't shown until someone
	// updates it: the change may be one of meaning (a rule, a deadline), and
	// the old wording would then say the wrong thing.
	Source string `json:"source"`
}

// SourceHash identifies the plain English a human version was written for:
// the first 8 hex digits of its SHA-256, enough to notice that it changed.
func SourceHash(english string) string {
	h := sha256.Sum256([]byte(english))
	return hex.EncodeToString(h[:4])
}

// Stamp checks a human/<lang>.json file against the plain English (en.json)
// and fills in Source for new entries, which `make gen` runs for whoever wrote
// them by hand. Existing Sources are kept: refreshing them would hide that the
// English changed under a version. The result is in EncodeHuman's format.
func Stamp(englishRaw, humanRaw []byte) ([]byte, error) {
	var english map[string]string
	if err := json.Unmarshal(englishRaw, &english); err != nil {
		return nil, fmt.Errorf("en.json: %w", err)
	}
	var human map[string]Human
	if err := json.Unmarshal(humanRaw, &human); err != nil {
		return nil, err
	}
	for key, v := range human {
		src, ok := english[key]
		switch {
		case !ok:
			return nil, fmt.Errorf("%s: no such key in en.json", key)
		case strings.TrimSpace(v.Text) == "":
			return nil, fmt.Errorf("%s: empty text", key)
		case !SamePlaceholders(src, v.Text):
			return nil, fmt.Errorf("%s: keep every {placeholder} of the English text, unchanged", key)
		case v.Source == "":
			v.Source = SourceHash(src)
			human[key] = v
		}
	}
	return EncodeHuman(human), nil
}

// EncodeHuman writes a human-version file with one string per line, sorted by
// key, so a change shows in a diff (and a pull request) as just its own line.
func EncodeHuman(human map[string]Human) []byte {
	if len(human) == 0 {
		return []byte("{}\n")
	}
	var out bytes.Buffer
	out.WriteString("{\n")
	for i, key := range slices.Sorted(maps.Keys(human)) {
		v := human[key]
		k, text, src := marshal(key), marshal(v.Text), marshal(v.Source)
		fmt.Fprintf(&out, `  %s: {"text": %s, "source": %s}`, k, text, src)
		if i < len(human)-1 {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString("}\n")
	return out.Bytes()
}

// marshal quotes a string as JSON without escaping <, > and & (UI text has
// them). Encoding a string can't fail.
func marshal(s string) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}
