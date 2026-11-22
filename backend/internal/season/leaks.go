package season

import (
	"fmt"
	"strings"
)

// LeakKind is which of a member's details turned up in the log.
type LeakKind string

const (
	LeakEmail     LeakKind = "email"
	LeakName      LeakKind = "name"
	LeakFirstName LeakKind = "first name"
)

// Leak is one of a member's details found in the log. It names the member by
// their place in the cast, never by the detail itself, so a report of a leak is
// not itself one.
type Leak struct {
	Member int
	Kind   LeakKind
}

func (l Leak) String() string { return fmt.Sprintf("member %d's %s", l.Member, l.Kind) }

// Search looks for each member's email, name and first name in the log, in any
// case: a log line may only carry what the account's ID says, so any hit is a
// leak (docs/spec/quality.md, "Privacy as tests"). It returns one Leak per
// member and kind, by member.
func Search(log []byte, cast []Member) []Leak {
	text := strings.ToLower(string(log))
	var out []Leak
	for _, m := range cast {
		for _, c := range []struct {
			kind   LeakKind
			detail string
		}{
			{LeakEmail, m.Email},
			{LeakName, m.Name},
			{LeakFirstName, m.FirstName},
		} {
			if c.detail != "" && strings.Contains(text, strings.ToLower(c.detail)) {
				out = append(out, Leak{Member: m.Index, Kind: c.kind})
			}
		}
	}
	return out
}
