package walks

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// generic words say what kind of place it is, not which one: "Kōenji
// Temple" and "Koenji Temple South Gate" are both about "koenji".
var generic = map[string]bool{
	"the": true, "of": true, "and": true, "at": true,
	"temple": true, "shrine": true, "jinja": true, "jingu": true, "tera": true, "church": true,
	"station": true, "sta": true, "exit": true, "gate": true, "north": true, "south": true, "east": true, "west": true, "main": true, "central": true,
	"cafe": true, "coffee": true, "kissa": true, "tea": true, "house": true, "teahouse": true, "bar": true, "pub": true, "restaurant": true,
	"shop": true, "store": true, "ten": true, "honten": true, "market": true, "shopping": true, "street": true, "district": true,
	"arcade": true, "shotengai": true, "yokocho": true, "alley": true, "lane": true,
	"park": true, "garden": true, "gardens": true, "museum": true, "gallery": true, "building": true, "plaza": true, "square": true,
}

// words splits a name into lowercase words without accents or
// punctuation: "Kōen-ji Temple" → [koen ji temple].
func words(s string) []string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r): // a combining accent: ō → o
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Fields(b.String())
}

// core is what names a place: its words without generic ones or the
// area's and city's own names, run together ("Koenji Station South Exit"
// in Koenji → "").
func core(name string, skip map[string]bool) (string, []string) {
	var kept []string
	for _, w := range words(name) {
		if !generic[w] && !skip[w] {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, ""), kept
}

// sameName reports whether the place Google found is plausibly the one the
// model named. Text Search always returns its nearest match, so a made-up
// "Hase-ji Temple" in Koenji comes back as Kōenji Temple: same kind, same
// area, different place. Names are compared loosely (romanisation varies:
// Yokocho and Yokocyo, Kōenji and Koenji, extra words such as a shop's
// branch), and a match the model can't be sure of counts as not found: a
// real place left out costs less than a note about one place shown on
// another.
func sameName(modelName, googleName string, req Request) bool {
	skip := map[string]bool{}
	for _, w := range append(words(req.Area), words(req.City)...) {
		skip[w] = true
	}
	// "Naka-Meguro" is also "nakameguro".
	skip[strings.Join(words(req.Area), "")] = true
	a, aw := core(modelName, skip)
	b, bw := core(googleName, skip)
	switch {
	case a == "" && b == "":
		// Both just "the station" (or park) of the area: the kind check
		// decides.
		return true
	case a == "" || b == "":
		a, b = strings.Join(words(modelName), ""), strings.Join(words(googleName), "")
	}
	if len(a) >= 3 && len(b) >= 3 && (strings.Contains(a, b) || strings.Contains(b, a)) {
		return true
	}
	for _, x := range aw {
		for _, y := range bw {
			if len([]rune(x)) >= 4 && x == y {
				return true
			}
		}
	}
	return similarity(a, b) >= 0.8
}

// similarity is 1 minus the edit distance over the longer length.
func similarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return 1 - float64(prev[len(rb)])/float64(max(len(ra), len(rb)))
}
