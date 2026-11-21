// Package retention holds the registry of every table in the database: what
// personal data it holds and for how long ([Technical operations]
// (../../../docs/spec/operations.md#retention-and-deletion)). The `retention`
// CI check fails when a table has no entry.
//
// The registry is registry.json, embedded so the nightly job (milestone 6) and
// the check read the same file.
package retention

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

//go:embed registry.json
var registryJSON []byte

// NoPersonalData is the value of personal_data for a table that holds none.
const NoPersonalData = "none"

// Entry says what one table holds and how long it is kept.
type Entry struct {
	// Table is schema-qualified, as the database names it: "app.queue_items".
	Table string `json:"table"`
	// PersonalData names what the table holds about a person ("email address
	// and display name"), or is NoPersonalData.
	PersonalData string `json:"personal_data"`
	// Lifetime says how long a row is kept, in the words of the spec's
	// retention table ("while the account exists", "30 days"). Required when the
	// table holds personal data.
	Lifetime string `json:"lifetime,omitempty"`
}

// Registry is every entry, one per table.
type Registry []Entry

// Load reads the registry embedded in the binary.
func Load() (Registry, error) {
	return Parse(bytes.NewReader(registryJSON))
}

// Parse reads a registry and rejects an entry that cannot be checked or acted
// on: no table, a table not named with its schema, no statement about personal
// data, personal data with no lifetime, an unknown field, or a table twice.
func Parse(r io.Reader) (Registry, error) {
	dec := json.NewDecoder(r)
	// A misspelt field ("lifetme") would otherwise read as a missing lifetime.
	dec.DisallowUnknownFields()
	var reg Registry
	if err := dec.Decode(&reg); err != nil {
		return nil, fmt.Errorf("retention registry: %w", err)
	}
	seen := map[string]bool{}
	var errs []error
	for i, e := range reg {
		switch {
		case !strings.Contains(e.Table, ".") || strings.HasPrefix(e.Table, ".") || strings.HasSuffix(e.Table, "."):
			errs = append(errs, fmt.Errorf("entry %d: table %q is not schema.table", i, e.Table))
		case e.PersonalData == "":
			errs = append(errs, fmt.Errorf("%s: personal_data is empty; say %q if the table holds none", e.Table, NoPersonalData))
		case e.PersonalData != NoPersonalData && e.Lifetime == "":
			errs = append(errs, fmt.Errorf("%s: holds personal data but has no lifetime", e.Table))
		case seen[e.Table]:
			errs = append(errs, fmt.Errorf("%s is listed twice", e.Table))
		}
		seen[e.Table] = true
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("retention registry: %w", errors.Join(errs...))
	}
	return reg, nil
}

// Check compares the registry with the tables the database holds (see Tables)
// and returns one line per disagreement, none when they agree. A table with no
// entry is a failure whatever it holds: the registry is how a table is said to
// hold no personal data, so a table nobody has described is treated as holding
// some. An entry for a table that is gone is a failure too, so the registry
// does not drift from the schema.
func (r Registry) Check(tables []string) []string {
	have := map[string]bool{}
	for _, e := range r {
		have[e.Table] = true
	}
	var problems []string
	for _, t := range tables {
		if !have[t] {
			problems = append(problems, fmt.Sprintf("%s: no entry in the retention registry; add one saying what personal data it holds and its lifetime", t))
		}
	}
	for _, e := range r {
		if !slices.Contains(tables, e.Table) {
			problems = append(problems, fmt.Sprintf("%s: in the retention registry but not in the database; remove the entry", e.Table))
		}
	}
	slices.Sort(problems)
	return problems
}
