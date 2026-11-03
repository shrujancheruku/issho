// Package paging is the one paging convention for the admin lists
// (AIP-158, https://google.aip.dev/158): the request carries page_size and
// page_token, the response next_page_token (empty on the last page).
//
// A token is a keyset cursor: the sort key and id of the last row sent,
// never an offset, so rows added or resolved between pages neither repeat
// nor skip a row. It is opaque to clients (base64 of JSON) and carries a
// scope, the list's name and filters. A token is refused (InvalidArgument)
// on any other list, or the same list with other filters, so it can't be
// used to read another list's rows or to step around a filter. It is not a
// permission: every query still applies its own auth and filter clauses and
// the cursor only narrows them.
package paging

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

const (
	// DefaultSize is the page size for page_size 0.
	DefaultSize = 50
	// MaxSize is the largest page; bigger requests are clamped to it.
	MaxSize = 200
)

// Size turns a request's page_size into the number of rows to send: 0 is
// DefaultSize and anything over MaxSize is MaxSize. Negative is a mistake.
func Size(requested int32) (int32, error) {
	switch {
	case requested < 0:
		return 0, errs.Invalid("page_size: not negative")
	case requested == 0:
		return DefaultSize, nil
	case requested > MaxSize:
		return MaxSize, nil
	}
	return requested, nil
}

// Scope names a list and its filters for tokens, so a token only works for
// the request that made it. Pass the list's name, then every filter value.
func Scope(list string, filters ...string) string {
	return list + "\x00" + strings.Join(filters, "\x00")
}

type token struct {
	S string   `json:"s"`
	K []string `json:"k"`
}

// Encode makes the token for the page after a row with these sort keys
// (strings from Time, UUID and the like, in sort order, the unique id last).
func Encode(scope string, keys ...string) string {
	b, _ := json.Marshal(token{S: scope, K: keys}) // strings only: can't fail
	return base64.RawURLEncoding.EncodeToString(b)
}

// Time, Int, Bool and UUID format sort keys for Encode; Keys reads them back.
func Time(t time.Time) string  { return strconv.FormatInt(t.UnixMicro(), 10) } // Postgres keeps microseconds
func Int(n int64) string       { return strconv.FormatInt(n, 10) }
func Bool(b bool) string       { return strconv.FormatBool(b) }
func UUID(id uuid.UUID) string { return id.String() }

// Keys reads a token's sort keys in order. The first parse failure is kept
// and returned by Err, so a handler reads every key, then checks once.
type Keys struct {
	// Present is false for the first page (no token); the readers then
	// return zero values, which the queries ignore.
	Present bool
	v       []string
	i       int
	err     error
}

var errBad = errs.Invalid("page_token: not valid for this list; start again without one")

// Open checks a request's page_token against its scope. An empty token is
// the first page. A token that isn't ours, or is for another list or other
// filters, or doesn't have n keys, is InvalidArgument.
func Open(tok, scope string, n int) (*Keys, error) {
	if tok == "" {
		return &Keys{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return nil, errBad
	}
	var t token
	if json.Unmarshal(raw, &t) != nil || t.S != scope || len(t.K) != n {
		return nil, errBad
	}
	return &Keys{Present: true, v: t.K}, nil
}

func (k *Keys) next() string {
	if !k.Present || k.i >= len(k.v) {
		return ""
	}
	s := k.v[k.i]
	k.i++
	return s
}

// String is the next key as it was given to Encode.
func (k *Keys) String() string { return k.next() }

// Time is the next key, made by paging.Time.
func (k *Keys) Time() time.Time {
	s := k.next()
	if !k.Present {
		return time.Time{}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		k.err = errBad
	}
	return time.UnixMicro(n).UTC()
}

// Int is the next key, made by paging.Int.
func (k *Keys) Int() int64 {
	s := k.next()
	if !k.Present {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		k.err = errBad
	}
	return n
}

// Bool is the next key, made by paging.Bool.
func (k *Keys) Bool() bool {
	s := k.next()
	if !k.Present {
		return false
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		k.err = errBad
	}
	return b
}

// UUID is the next key, made by paging.UUID. uuid.Nil on a first page.
func (k *Keys) UUID() uuid.UUID {
	s := k.next()
	if !k.Present {
		return uuid.Nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		k.err = errBad
	}
	return id
}

// Date formats a calendar date for Encode.
func Date(t time.Time) string { return t.Format(time.DateOnly) }

// Date is the next key, made by paging.Date.
func (k *Keys) Date() time.Time {
	s := k.next()
	if !k.Present {
		return time.Time{}
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		k.err = errBad
	}
	return t
}

// Err is the first key that didn't parse, as InvalidArgument.
func (k *Keys) Err() error { return k.err }

// Trim cuts a page off rows fetched with a limit of size+1: the extra row
// only says there is a next page. more is true when it was there.
func Trim[T any](rows []T, size int32) (page []T, more bool) {
	if int32(len(rows)) > size {
		return rows[:size], true
	}
	return rows, false
}
