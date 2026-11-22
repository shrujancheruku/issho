package people_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/people"
)

// ada is joined (an account exists) before the profile tests start, since a
// profile belongs to an account.
func joined(t *testing.T, r *rig, id, email string) {
	t.Helper()
	if _, err := r.svc.Join(context.Background(), people.Claims{Subject: id, Email: email}, people.Preferences{}); err != nil {
		t.Fatal(err)
	}
}

// forget removes every profile, so a table of cases can each be a first save
// (the date of birth is asked once, so a second save with one is refused).
func forget(t *testing.T, r *rig) {
	t.Helper()
	if _, err := r.pool.Exec(context.Background(), `delete from app.profiles`); err != nil {
		t.Fatal(err)
	}
}

// goodProfile is a complete first-signup profile; tests change one field.
func goodProfile() people.ProfileInput {
	return people.ProfileInput{
		Name:                  "Ada",
		DateOfBirth:           "1990-06-15",
		Gender:                people.GenderWoman,
		Languages:             []string{"en", "vi"},
		LivesInCity:           true,
		AcceptedCodeOfConduct: true,
	}
}

// The profile holds the name they go by, gender, spoken languages, whether they
// live in the city and the code of conduct they accepted; and it is what a
// later read returns.
func TestTheProfileHoldsTheSixAnswers(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	joined(t, r, ada, "ada@example.com")

	got, err := r.svc.SaveProfile(ctx, ada, goodProfile())
	if err != nil {
		t.Fatal(err)
	}
	want := people.Profile{
		Name:                  "Ada",
		Gender:                people.GenderWoman,
		Languages:             []string{"en", "vi"},
		LivesInCity:           true,
		OnlyWomenAndNonBinary: false,
		CodeOfConductVersion:  people.CodeOfConductVersion,
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("profile = %+v, want %+v", got, want)
	}
	read, err := r.svc.Profile(ctx, ada)
	if err != nil || fmt.Sprint(read) != fmt.Sprint(want) {
		t.Fatalf("Profile = %+v, %v; want %+v", read, err, want)
	}
}

// Either answer to "do you live in the city" is accepted.
func TestEitherAnswerToLivingInTheCityIsAccepted(t *testing.T) {
	r := newRig(t)
	joined(t, r, ada, "ada@example.com")
	in := goodProfile()
	in.LivesInCity = false
	got, err := r.svc.SaveProfile(context.Background(), ada, in)
	if err != nil || got.LivesInCity {
		t.Fatalf("profile = %+v, %v; want a profile that says they don't live in the city", got, err)
	}
}

// The name is 1 to 40 characters (not bytes), checked against nothing else.
func TestTheNameIsOneToFortyCharacters(t *testing.T) {
	r := newRig(t)
	joined(t, r, ada, "ada@example.com")
	for _, tc := range []struct {
		name    string
		in      string
		invalid bool
	}{
		{"one character", "A", false},
		{"forty characters", strings.Repeat("a", 40), false},
		{"forty multi-byte characters", strings.Repeat("ế", 40), false},
		{"forty-one characters", strings.Repeat("a", 41), true},
		{"empty", "", true},
		{"only spaces", "   ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := goodProfile()
			in.Name = tc.in
			forget(t, r)
			_, err := r.svc.SaveProfile(context.Background(), ada, in)
			if tc.invalid != errors.Is(err, people.ErrInvalid) || (!tc.invalid && err != nil) {
				t.Fatalf("err = %v, invalid want %v", err, tc.invalid)
			}
		})
	}
}

// The four genders are the only answers, and an answer is required.
func TestGenderIsRequiredAndOneOfFour(t *testing.T) {
	r := newRig(t)
	joined(t, r, ada, "ada@example.com")
	for _, g := range []people.Gender{people.GenderWoman, people.GenderMan, people.GenderNonBinary, people.GenderPreferNotToSay} {
		in := goodProfile()
		in.Gender = g
		forget(t, r)
		if _, err := r.svc.SaveProfile(context.Background(), ada, in); err != nil {
			t.Fatalf("gender %q: %v", g, err)
		}
	}
	for _, g := range []people.Gender{"", "other", "WOMAN"} {
		in := goodProfile()
		in.Gender = g
		if _, err := r.svc.SaveProfile(context.Background(), ada, in); !errors.Is(err, people.ErrInvalid) {
			t.Fatalf("gender %q: err = %v, want ErrInvalid", g, err)
		}
	}
}

// At least one spoken language, any language, no levels.
func TestSpokenLanguagesNeedAtLeastOneAndAnyIsAllowed(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	joined(t, r, ada, "ada@example.com")

	in := goodProfile()
	in.Languages = []string{"ja", "pt-BR", "ja"} // not an open city's language; a repeat counts once
	got, err := r.svc.SaveProfile(ctx, ada, in)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got.Languages) != "[ja pt-BR]" {
		t.Fatalf("languages = %v, want [ja pt-BR]", got.Languages)
	}
	for _, bad := range [][]string{nil, {}, {""}, {"English please"}} {
		in := goodProfile()
		in.Languages = bad
		if _, err := r.svc.SaveProfile(ctx, ada, in); !errors.Is(err, people.ErrInvalid) {
			t.Fatalf("languages %q: err = %v, want ErrInvalid", bad, err)
		}
	}
}

// The code of conduct is accepted with the profile, and the version accepted is
// stored. Without it there is no profile.
func TestTheCodeOfConductMustBeAccepted(t *testing.T) {
	r := newRig(t)
	joined(t, r, ada, "ada@example.com")
	in := goodProfile()
	in.AcceptedCodeOfConduct = false
	if _, err := r.svc.SaveProfile(context.Background(), ada, in); !errors.Is(err, people.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if err := r.svc.RequireProfile(context.Background(), ada); !errors.Is(err, people.ErrProfileRequired) {
		t.Fatalf("a refused profile left one: %v", err)
	}
}

// Under 18 on the day, by the city's clock, is refused. The city's clock is
// Asia/Ho_Chi_Minh (UTC+7): at 20:00 UTC on 2026-01-05 it is already
// 2026-01-06 there, so a person born 2008-01-06 turns 18 that day.
func TestUnderEighteenOnTheDayByTheCitysClockIsRefused(t *testing.T) {
	r := newRig(t)
	r.clk.Set(time.Date(2026, 1, 5, 20, 0, 0, 0, time.UTC)) // 2026-01-06 03:00 in the city
	ctx := context.Background()
	joined(t, r, ada, "ada@example.com")
	for _, tc := range []struct {
		dob   string
		under bool
	}{
		{"2008-01-06", false}, // 18 today, by the city's clock (still 17 by UTC's)
		{"2008-01-07", true},  // 18 tomorrow
		{"2010-03-01", true},
		{"1990-06-15", false},
		{"2008-02-29x", true}, // not a date
		{"", true},            // a first profile needs it
		{"2027-01-01", true},  // not yet born
	} {
		in := goodProfile()
		in.DateOfBirth = tc.dob
		forget(t, r)
		_, err := r.svc.SaveProfile(ctx, ada, in)
		if tc.under && err == nil {
			t.Errorf("dob %q: accepted", tc.dob)
		}
		if !tc.under && err != nil {
			t.Errorf("dob %q: %v", tc.dob, err)
		}
	}
	// A date that is a real person's age but too young is ErrUnderage specifically.
	in := goodProfile()
	in.DateOfBirth = "2010-03-01"
	forget(t, r)
	if _, err := r.svc.SaveProfile(ctx, ada, in); !errors.Is(err, people.ErrUnderage) || !errors.Is(err, people.ErrInvalid) {
		t.Fatalf("err = %v, want ErrUnderage (which is also ErrInvalid)", err)
	}
}

// Only the fact and the day it was said are kept: the date of birth is in no
// column, and the day is the city's date.
func TestTheDateOfBirthIsNeverStored(t *testing.T) {
	r := newRig(t)
	r.clk.Set(time.Date(2026, 1, 5, 20, 0, 0, 0, time.UTC)) // 2026-01-06 in the city
	ctx := context.Background()
	joined(t, r, ada, "ada@example.com")
	in := goodProfile()
	in.DateOfBirth = "1987-03-21"
	if _, err := r.svc.SaveProfile(ctx, ada, in); err != nil {
		t.Fatal(err)
	}
	var row, said string
	err := r.pool.QueryRow(ctx, `select p::text, said_adult_on::text from app.profiles p where account_id = $1`, ada).Scan(&row, &said)
	if err != nil {
		t.Fatal(err)
	}
	if said != "2026-01-06" {
		t.Fatalf("said_adult_on = %s, want the city's day 2026-01-06", said)
	}
	for _, leak := range []string{"1987", "03-21", "1987-03-21"} {
		if strings.Contains(row, leak) {
			t.Fatalf("the stored row holds part of the date of birth (%q): %s", leak, row)
		}
	}
}

// Everything but the date of birth can be changed later, and the date of birth
// can't: it is not asked again.
func TestEverythingButTheDateOfBirthCanBeChanged(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	joined(t, r, ada, "ada@example.com")
	if _, err := r.svc.SaveProfile(ctx, ada, goodProfile()); err != nil {
		t.Fatal(err)
	}
	r.clk.Advance(48 * time.Hour)
	change := goodProfile()
	change.DateOfBirth = ""
	change.Name = "Ada L."
	change.LivesInCity = false
	got, err := r.svc.SaveProfile(ctx, ada, change)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ada L." || got.LivesInCity {
		t.Fatalf("profile = %+v, want the new name and answer", got)
	}
	var said string
	if err := r.pool.QueryRow(ctx, `select said_adult_on::text from app.profiles where account_id = $1`, ada).Scan(&said); err != nil || said != "2026-01-05" {
		t.Fatalf("said_adult_on = %q, %v; want the first day, 2026-01-05", said, err)
	}
	again := goodProfile() // sends a date of birth again
	if _, err := r.svc.SaveProfile(ctx, ada, again); !errors.Is(err, people.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid: the date of birth is not asked twice", err)
	}
}

// A person who answered woman or non-binary can turn on the setting; anyone
// else is refused, and nothing is saved.
func TestOnlyWomenAndNonBinaryIsForWomenAndNonBinaryPeople(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	joined(t, r, ada, "ada@example.com")
	for _, tc := range []struct {
		gender people.Gender
		ok     bool
	}{
		{people.GenderWoman, true},
		{people.GenderNonBinary, true},
		{people.GenderMan, false},
		{people.GenderPreferNotToSay, false},
	} {
		forget(t, r)
		in := goodProfile()
		in.Gender = tc.gender
		in.OnlyWomenAndNonBinary = true
		got, err := r.svc.SaveProfile(ctx, ada, in)
		if tc.ok && (err != nil || !got.OnlyWomenAndNonBinary) {
			t.Errorf("%s: %+v, %v; want the setting on", tc.gender, got, err)
		}
		if !tc.ok {
			if !errors.Is(err, people.ErrWomenOnlyNotAvailable) || !errors.Is(err, people.ErrInvalid) {
				t.Errorf("%s: err = %v, want ErrWomenOnlyNotAvailable (an ErrInvalid)", tc.gender, err)
			}
			if err := r.svc.RequireProfile(ctx, ada); !errors.Is(err, people.ErrProfileRequired) {
				t.Errorf("%s: a refused profile left one: %v", tc.gender, err)
			}
		}
	}
}

// The setting can be switched at any time while the person is eligible, and a
// gender change to one that isn't eligible turns it off by itself (a form that
// still sends it on isn't refused), though a person who never had it on can't
// turn it on with that gender.
func TestTheSettingFollowsEligibilityWhenTheGenderChanges(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	joined(t, r, ada, "ada@example.com")
	in := goodProfile()
	in.OnlyWomenAndNonBinary = true
	if _, err := r.svc.SaveProfile(ctx, ada, in); err != nil {
		t.Fatal(err)
	}
	in.DateOfBirth = ""
	in.OnlyWomenAndNonBinary = false
	if got, err := r.svc.SaveProfile(ctx, ada, in); err != nil || got.OnlyWomenAndNonBinary {
		t.Fatalf("profile = %+v, %v; want the setting off", got, err)
	}
	in.OnlyWomenAndNonBinary = true
	if got, err := r.svc.SaveProfile(ctx, ada, in); err != nil || !got.OnlyWomenAndNonBinary {
		t.Fatalf("profile = %+v, %v; want the setting back on", got, err)
	}
	in.Gender = people.GenderMan // the gender changes while the setting is on, and still sent on
	if got, err := r.svc.SaveProfile(ctx, ada, in); err != nil || got.Gender != people.GenderMan || got.OnlyWomenAndNonBinary {
		t.Fatalf("profile = %+v, %v; want a man with the setting turned off", got, err)
	}
	// Now off, a man can't turn it back on.
	if _, err := r.svc.SaveProfile(ctx, ada, in); !errors.Is(err, people.ErrWomenOnlyNotAvailable) {
		t.Fatalf("err = %v, want ErrWomenOnlyNotAvailable", err)
	}
}

// A signup needs a profile: an account with none is told so, in an error the
// app can act on (ErrProfileRequired); it is not ErrNotFound, which is no account.
func TestAnAccountWithNoProfileIsToldItNeedsOne(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	joined(t, r, ada, "ada@example.com")
	if err := r.svc.RequireProfile(ctx, ada); !errors.Is(err, people.ErrProfileRequired) {
		t.Fatalf("err = %v, want ErrProfileRequired", err)
	}
	if _, err := r.svc.Profile(ctx, ada); !errors.Is(err, people.ErrProfileRequired) {
		t.Fatalf("err = %v, want ErrProfileRequired", err)
	}
	// No account at all is a different answer.
	if err := r.svc.RequireProfile(ctx, grace); !errors.Is(err, people.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, err := r.svc.SaveProfile(ctx, ada, goodProfile()); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.RequireProfile(ctx, ada); err != nil {
		t.Fatalf("a person with a profile is refused: %v", err)
	}
	// Grace's still isn't there.
	joined(t, r, grace, "grace@example.com")
	if err := r.svc.RequireProfile(ctx, grace); !errors.Is(err, people.ErrProfileRequired) {
		t.Fatalf("err = %v, want ErrProfileRequired", err)
	}
}

// A profile belongs to an account: saving one for no account makes nothing.
func TestAProfileForNoAccountIsRefused(t *testing.T) {
	r := newRig(t)
	if _, err := r.svc.SaveProfile(context.Background(), ada, goodProfile()); !errors.Is(err, people.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// A first name is the one thing of another person that is shown (docs/spec/
// outings.md, "What the group sees of each other"): the first word of the name
// they go by, and no more, whatever else it holds. A person with no profile has
// none, and an id nobody has is left out rather than an error.
func TestFirstNamesAreTheFirstWordOfTheNameAndNothingMore(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	joined(t, r, ada, "ada@example.com")
	joined(t, r, grace, "grace@example.com")
	joined(t, r, "33333333-3333-4333-8333-333333333333", "third@example.com")
	in := goodProfile()
	in.Name = "  Ada Augusta  King "
	if _, err := r.svc.SaveProfile(ctx, ada, in); err != nil {
		t.Fatal(err)
	}
	in.Name = "Grace"
	if _, err := r.svc.SaveProfile(ctx, grace, in); err != nil {
		t.Fatal(err)
	}

	got, err := r.svc.FirstNames(ctx, []string{ada, grace, "33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"})

	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{ada: "Ada", grace: "Grace"}; len(got) != 2 || got[ada] != want[ada] || got[grace] != want[grace] {
		t.Fatalf("FirstNames = %v, want %v", got, want)
	}
}

// No ids is no query and no names.
func TestFirstNamesOfNobodyIsNobody(t *testing.T) {
	r := newRig(t)
	got, err := r.svc.FirstNames(context.Background(), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("FirstNames(nil) = %v, %v; want none", got, err)
	}
}
