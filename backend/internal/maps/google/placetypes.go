package google

import (
	_ "embed"
	"strings"
	"sync"
)

//go:embed placetypes.txt
var placeTypesTxt string

var placeTypes = sync.OnceValue(func() map[string]bool {
	m := map[string]bool{}
	for _, l := range strings.Split(placeTypesTxt, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			m[l] = true
		}
	}
	return m
})

// IsPlaceType reports whether t is a Places API (New) Table A type, i.e. one
// accepted in Nearby Search includedTypes. An unknown type fails the whole
// request with INVALID_ARGUMENT, so data files are checked against this list.
// The list is a snapshot; refresh placetypes.txt when Google adds types.
func IsPlaceType(t string) bool { return placeTypes()[t] }
