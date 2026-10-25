package translate

import (
	_ "embed"
	"strings"
)

// guide.md: how Issho's text is translated (voice, the product's name,
// terms, and the register per language). Translators that follow
// instructions get it; Google Translation Basic has no way to.
//
//go:embed guide.md
var guide string

// Guide returns the rules for every language ("# all") plus lang's own
// section, if guide.md has one.
func Guide(lang string) string {
	sections := map[string]string{}
	for _, part := range strings.Split("\n"+guide, "\n# ")[1:] {
		name, body, _ := strings.Cut(part, "\n")
		sections[strings.TrimSpace(name)] = strings.TrimSpace(body)
	}
	out := sections["all"]
	if s := sections[lang]; s != "" {
		out += "\n\nFor this language:\n" + s
	}
	return out
}
