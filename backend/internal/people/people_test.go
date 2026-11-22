package people_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

var start = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

const (
	ada   = "11111111-1111-4111-8111-111111111111"
	grace = "22222222-2222-4222-8222-222222222222"
)

type rig struct {
	pool *pgxpool.Pool
	clk  *clock.Fake
	svc  *people.Service
}

func newRig(t *testing.T) *rig {
	t.Helper()
	pool := testdb.NewDatabase(t)
	if err := db.Up(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(start)
	return &rig{pool: pool, clk: clk, svc: people.New(clk, pool, people.Config{DefaultCity: "da-nang-hoi-an"})}
}

// The first call with a valid token makes the account: the token's email, a
// city and a display language.
func TestFirstCallMakesTheAccount(t *testing.T) {
	r := newRig(t)
	got, err := r.svc.Join(context.Background(), people.Claims{Subject: ada, Email: "ada@example.com"},
		people.Preferences{City: "da-nang-hoi-an", DisplayLanguage: "vi"})
	if err != nil {
		t.Fatal(err)
	}
	want := people.Account{ID: ada, Email: "ada@example.com", City: "da-nang-hoi-an", DisplayLanguage: "vi", CreatedAt: start}
	if got != want {
		t.Fatalf("account = %+v, want %+v", got, want)
	}
	again, err := r.svc.Account(context.Background(), ada)
	if err != nil || again != want {
		t.Fatalf("Account = %+v, %v; want %+v", again, err, want)
	}
}

// Nothing sent means the defaults: the only listed city, and English, which
// is the fallback for every device (docs/spec/languages.md).
func TestNoPreferencesMeansTheDefaults(t *testing.T) {
	r := newRig(t)
	got, err := r.svc.Join(context.Background(), people.Claims{Subject: ada, Email: "ada@example.com"}, people.Preferences{})
	if err != nil {
		t.Fatal(err)
	}
	if got.City != "da-nang-hoi-an" || got.DisplayLanguage != "en" {
		t.Fatalf("city, language = %q, %q", got.City, got.DisplayLanguage)
	}
}

// Joining is once: a later call returns the account as it is, and what that
// call sent is not applied (a settings change is its own call).
func TestJoiningAgainChangesNothing(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	c := people.Claims{Subject: ada, Email: "ada@example.com"}
	first, err := r.svc.Join(ctx, c, people.Preferences{DisplayLanguage: "vi"})
	if err != nil {
		t.Fatal(err)
	}
	r.clk.Advance(time.Hour)
	again, err := r.svc.Join(ctx, c, people.Preferences{City: "hanoi", DisplayLanguage: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Fatalf("second call = %+v, want %+v", again, first)
	}
}

func TestEmailIsKeptLowercase(t *testing.T) {
	r := newRig(t)
	got, err := r.svc.Join(context.Background(), people.Claims{Subject: ada, Email: "Ada@Example.COM"}, people.Preferences{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "ada@example.com" {
		t.Fatalf("email = %q", got.Email)
	}
}

func TestAccountsAreSeparate(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	if _, err := r.svc.Join(ctx, people.Claims{Subject: ada, Email: "ada@example.com"}, people.Preferences{}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Join(ctx, people.Claims{Subject: grace, Email: "grace@example.com"}, people.Preferences{}); err != nil {
		t.Fatal(err)
	}
	got, err := r.svc.Account(ctx, grace)
	if err != nil || got.Email != "grace@example.com" {
		t.Fatalf("Account(grace) = %+v, %v", got, err)
	}
}

func TestAccountThatDoesNotExistIsNotFound(t *testing.T) {
	r := newRig(t)
	if _, err := r.svc.Account(context.Background(), ada); !errors.Is(err, people.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestJoinRefusesWhatIsNotAnAccount(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	for name, tc := range map[string]struct {
		claims people.Claims
		prefs  people.Preferences
	}{
		"no subject":           {people.Claims{Email: "a@example.com"}, people.Preferences{}},
		"subject not a uuid":   {people.Claims{Subject: "ada", Email: "a@example.com"}, people.Preferences{}},
		"no email":             {people.Claims{Subject: ada}, people.Preferences{}},
		"email not an address": {people.Claims{Subject: ada, Email: "Ada <a@example.com>"}, people.Preferences{}},
		"unknown language":     {people.Claims{Subject: ada, Email: "a@example.com"}, people.Preferences{DisplayLanguage: "fr"}},
		"city not a slug":      {people.Claims{Subject: ada, Email: "a@example.com"}, people.Preferences{City: "Da Nang"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := r.svc.Join(ctx, tc.claims, tc.prefs); !errors.Is(err, people.ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

// One account per person: an email another account holds is refused rather
// than making a second account for it.
func TestAnEmailBelongsToOneAccount(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	if _, err := r.svc.Join(ctx, people.Claims{Subject: ada, Email: "ada@example.com"}, people.Preferences{}); err != nil {
		t.Fatal(err)
	}
	_, err := r.svc.Join(ctx, people.Claims{Subject: grace, Email: "ADA@example.com"}, people.Preferences{})
	if !errors.Is(err, people.ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
}
