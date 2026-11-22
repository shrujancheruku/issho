// Package people owns accounts: who a person is to Issho, from the first
// signed-in call (docs/spec/people.md, "Sign-in and the account"). An account
// is an email, a city and a display language, nothing more. The profile,
// standing and join links are added to this package by later tickets, each in
// a table of its own keyed by the account's id.
//
// Sign-in itself is Supabase Auth's, with an email code. This package never
// sees a code or a password: it checks the token Auth hands the app (Verifier)
// and makes the account the first time that token is used (Service.Join).
package people

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/people/store"
)

// Account is a person's email, city and display language.
type Account struct {
	// ID is Supabase Auth's user id (the token's subject), as lowercase text.
	ID string
	// Email is lowercase. It is the person's own: no RPC returns another
	// person's.
	Email string
	// City is the city's slug.
	City string
	// DisplayLanguage is the language the person reads Issho in: "en" or "vi".
	DisplayLanguage string
	CreatedAt       time.Time
}

// Claims is what a verified token says about the caller.
type Claims struct {
	Subject string
	Email   string
	// SecondStep is whether the person took the second sign-in step: the
	// token's `aal2`. Staff's admin RPCs require it.
	SecondStep bool
}

// Preferences is what the app sends on the first call, which it knows and the
// token doesn't: the city of the link the person arrived by, and the language
// they were reading in ([Languages]: "the language they were reading in when
// they joined"). Either may be empty.
//
// [Languages]: ../../../docs/spec/languages.md
type Preferences struct {
	City            string
	DisplayLanguage string
}

// DefaultLanguage is the display language of a person who sent none: English
// is the fallback for every device (docs/spec/languages.md).
const DefaultLanguage = "en"

// Languages are the display languages of the open cities, a short fixed list
// that grows only when a city that needs another opens.
var Languages = []string{"en", "vi"}

var (
	// ErrNotFound: no account has that id.
	ErrNotFound = errors.New("people: no such account")
	// ErrInvalid wraps every refusal of malformed input.
	ErrInvalid = errors.New("people: invalid")
	// ErrEmailTaken: another account already has the email. It should not
	// happen while Supabase Auth makes one user per email; it is refused rather
	// than making a second account for one person.
	ErrEmailTaken = errors.New("people: email belongs to another account")
)

// Config is what the service needs from the process that builds it.
type Config struct {
	// DefaultCity is the city of a first call that names none: the only listed
	// city (docs/spec/cities.md, "Which city a visitor with no account sees").
	DefaultCity string
}

// Service is the exported API of accounts.
type Service struct {
	clock clock.Clock
	pool  *pgxpool.Pool
	cfg   Config
}

// New returns a Service that stamps times from clk and keeps accounts in pool.
func New(clk clock.Clock, pool *pgxpool.Pool, cfg Config) *Service {
	if cfg.DefaultCity == "" {
		panic("people: Config.DefaultCity is required")
	}
	return &Service{clock: clk, pool: pool, cfg: cfg}
}

var (
	uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// Join returns the caller's account, making it first if this is the first
// call with their token: the token's email, the city and display language in
// prefs (or their defaults). On any later call it returns the account as it
// is and ignores prefs: changing a city or language is a settings call, not a
// second join.
func (s *Service) Join(ctx context.Context, c Claims, prefs Preferences) (Account, error) {
	subject := lowerASCII(c.Subject)
	if !uuidRe.MatchString(subject) {
		return Account{}, fmt.Errorf("%w: subject is not an id", ErrInvalid)
	}
	addr, err := mail.ParseAddress(c.Email)
	if err != nil || addr.Address != c.Email {
		return Account{}, fmt.Errorf("%w: token has no email address", ErrInvalid)
	}
	city := prefs.City
	if city == "" {
		city = s.cfg.DefaultCity
	}
	if !slugRe.MatchString(city) {
		return Account{}, fmt.Errorf("%w: city is not a slug", ErrInvalid)
	}
	language := prefs.DisplayLanguage
	if language == "" {
		language = DefaultLanguage
	}
	if !contains(Languages, language) {
		return Account{}, fmt.Errorf("%w: unknown display language", ErrInvalid)
	}

	row, err := store.New(s.pool).InsertAccount(ctx, store.InsertAccountParams{
		ID: subject, Email: lowerASCII(c.Email), City: city, DisplayLanguage: language, CreatedAt: s.clock.Now(),
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "accounts_email_key" {
		return Account{}, ErrEmailTaken
	}
	if err != nil {
		return Account{}, fmt.Errorf("people: join: %w", err)
	}
	return accountOf(row), nil
}

// Account returns the account with the given id, or ErrNotFound.
func (s *Service) Account(ctx context.Context, id string) (Account, error) {
	row, err := store.New(s.pool).GetAccount(ctx, lowerASCII(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		// A malformed id is a uuid syntax error from Postgres; it names no account.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
			return Account{}, ErrNotFound
		}
		return Account{}, fmt.Errorf("people: account: %w", err)
	}
	return accountOf(row), nil
}

func accountOf(r store.AppAccount) Account {
	return Account{ID: r.ID, Email: r.Email, City: r.City, DisplayLanguage: r.DisplayLanguage, CreatedAt: r.CreatedAt.UTC()}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// lowerASCII lowercases ids and addresses. Email domains are case-insensitive
// and so, in practice, are the local parts Supabase Auth issues tokens for.
func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
