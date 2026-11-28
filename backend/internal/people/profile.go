package people

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/cityclock"
	"github.com/shrujancheruku/issho/backend/internal/people/store"
	"github.com/shrujancheruku/issho/backend/internal/policy"
)

// Gender is the answer to the profile's gender question: one of four, and an
// answer is required (docs/spec/people.md, "The profile"). It is asked so that
// nobody is the only woman or the only man in a group.
type Gender string

const (
	GenderWoman          Gender = "woman"
	GenderMan            Gender = "man"
	GenderNonBinary      Gender = "non-binary"
	GenderPreferNotToSay Gender = "prefer-not-to-say"
)

// CodeOfConductVersion is the version of the code of conduct a person accepts
// with the profile; the version accepted is stored, so a new version can ask
// for agreement again (docs/spec/people.md, "The code of conduct").
const CodeOfConductVersion = 1

var (
	// ErrProfileRequired: the account has no profile yet. A signup, or anything
	// else that needs one, is refused with it, and the app answers by asking the
	// profile's questions.
	ErrProfileRequired = errors.New("people: a profile is required first")
	// ErrUnderage: the date of birth is under 18 on the day, by the city's clock.
	// It is an ErrInvalid too.
	ErrUnderage = fmt.Errorf("%w: under age", ErrInvalid)
	// ErrWomenOnlyNotAvailable: "only women and non-binary people" was asked for
	// by someone who didn't answer woman or non-binary. It is an ErrInvalid too.
	ErrWomenOnlyNotAvailable = fmt.Errorf("%w: only women and non-binary people is not available", ErrInvalid)
)

// ProfileInput is what the profile's questions send. The first save needs all
// of it; a later one changes everything but the date of birth, which is not
// asked twice and so must be empty.
type ProfileInput struct {
	// Name is the name they go by: 1 to 40 characters, checked against nothing.
	Name string
	// DateOfBirth is "2006-01-02". It is checked and thrown away: only "said they
	// were 18 or over, on this day" is kept.
	DateOfBirth string
	Gender      Gender
	// Languages are the languages they can comfortably hold a conversation in
	// for a whole outing: at least one, any language, no levels. Each is a short
	// language code ("en", "pt-BR"), listed once.
	Languages []string
	// LivesInCity is self-declared; either answer is accepted.
	LivesInCity bool
	// AcceptedCodeOfConduct must be true on the first save; on a change it is
	// ignored and the stored version and time stay.
	AcceptedCodeOfConduct bool
	// OnlyWomenAndNonBinary is the setting offered to people who answered woman
	// or non-binary; for anyone else it is refused.
	OnlyWomenAndNonBinary bool
}

// Profile is a person's own answers. It carries no date of birth, and nothing
// returns another person's beyond their first name.
type Profile struct {
	Name                  string
	Gender                Gender
	Languages             []string
	LivesInCity           bool
	OnlyWomenAndNonBinary bool
	// CodeOfConductVersion is the version they accepted.
	CodeOfConductVersion int
}

// noParts are day parts for a clock used only for its Date: the city's parts
// of the day are a later ticket's setting, and Date doesn't read them. Each is
// an empty span, which cityclock accepts.
var noParts = cityclock.DayParts{
	Morning:   cityclock.Span{From: "00:00", To: "00:00"},
	Afternoon: cityclock.Span{From: "00:00", To: "00:00"},
	Evening:   cityclock.Span{From: "00:00", To: "00:00"},
}

// languageCode is a BCP 47 tag's usual shape: "en", "vi", "pt-BR", "zh-Hant".
var languageCode = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// SaveProfile makes the account's profile on its first call, and changes it on
// later ones. The code of conduct must be accepted on the first call and is
// stamped then; later calls ignore it and keep the stored version and time. It is the one place a date of birth is read: on the first call it
// must be 18 or over on the city's date today, and only that fact and the day
// it was said are stored. ErrNotFound if there is no such account; ErrInvalid
// (ErrUnderage, ErrWomenOnlyNotAvailable) for an answer that is refused, with
// nothing saved.
func (s *Service) SaveProfile(ctx context.Context, accountID string, in ProfileInput) (Profile, error) {
	acct, err := s.Account(ctx, accountID)
	if err != nil {
		return Profile{}, err
	}
	q := store.New(s.pool)
	existing, err := q.GetProfile(ctx, acct.ID)
	hasProfile := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, fmt.Errorf("people: save profile: %w", err)
	}
	// The code of conduct is accepted at the first save only. A change to a
	// profile that exists neither needs it nor can change it: agreeing to a new
	// version is its own, later step.
	if err := checkAnswers(in, !hasProfile); err != nil {
		return Profile{}, err
	}

	// Turning "only women and non-binary people" on is for those who answered
	// woman or non-binary. A person who already had it on and changes their
	// gender to another answer has it turned off by itself (docs/spec/people.md,
	// "What a person can change"), so a form still sending it on isn't refused.
	if in.OnlyWomenAndNonBinary && !eligibleForWomenOnly(in.Gender) {
		if !hasProfile || !existing.OnlyWomenAndNonBinary {
			return Profile{}, ErrWomenOnlyNotAvailable
		}
		in.OnlyWomenAndNonBinary = false
	}

	now := s.clock.Now()
	saidAdultOn := existing.SaidAdultOn
	if hasProfile {
		if in.DateOfBirth != "" {
			return Profile{}, fmt.Errorf("%w: the date of birth is asked once and can't be changed", ErrInvalid)
		}
	} else {
		city, err := s.cities.Get(ctx, acct.City)
		if err != nil {
			return Profile{}, fmt.Errorf("people: save profile: %w", cityError(err))
		}
		clk, err := cityclock.New(city.TimeZone, noParts)
		if err != nil {
			return Profile{}, fmt.Errorf("people: save profile: %w", err)
		}
		today := clk.Date(now)
		if err := checkAdult(in.DateOfBirth, today); err != nil {
			return Profile{}, err
		}
		saidAdultOn, err = time.Parse(time.DateOnly, today)
		if err != nil {
			return Profile{}, fmt.Errorf("people: save profile: %w", err)
		}
	}

	conductVersion, conductAt := int32(CodeOfConductVersion), now
	if hasProfile {
		conductVersion, conductAt = existing.CodeOfConductVersion, existing.CodeOfConductAcceptedAt
	}
	row, err := q.UpsertProfile(ctx, store.UpsertProfileParams{
		AccountID:               acct.ID,
		Name:                    strings.TrimSpace(in.Name),
		Gender:                  string(in.Gender),
		Languages:               dedupe(in.Languages),
		LivesInCity:             in.LivesInCity,
		OnlyWomenAndNonBinary:   in.OnlyWomenAndNonBinary,
		SaidAdultOn:             saidAdultOn,
		CodeOfConductVersion:    conductVersion,
		CodeOfConductAcceptedAt: conductAt,
		CreatedAt:               now,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "profiles_account_id_fkey" {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("people: save profile: %w", err)
	}
	return profileOf(row), nil
}

// Profile returns the account's own profile, ErrProfileRequired if it has none
// and ErrNotFound if there is no such account. It takes the caller's own id:
// no function of this package returns another person's profile.
func (s *Service) Profile(ctx context.Context, accountID string) (Profile, error) {
	acct, err := s.Account(ctx, accountID)
	if err != nil {
		return Profile{}, err
	}
	row, err := store.New(s.pool).GetProfile(ctx, acct.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrProfileRequired
	}
	if err != nil {
		return Profile{}, fmt.Errorf("people: profile: %w", err)
	}
	return profileOf(row), nil
}

// FirstNames returns the first name of each account in ids that has a profile,
// by account id: the first word of the name they go by, and nothing else of
// them. It is the one thing this package shows of another person, for the
// people going to one outing (docs/spec/outings.md, "What the group sees of
// each other"); an id with no profile, or no account, is left out. It is for
// the caller to have checked that the viewer may see these people.
func (s *Service) FirstNames(ctx context.Context, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := store.New(s.pool).ListProfileNames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("people: first names: %w", err)
	}
	for _, r := range rows {
		if words := strings.Fields(r.Name); len(words) > 0 {
			out[r.AccountID] = words[0]
		}
	}
	return out, nil
}

// RequireProfile is the check a signup (or anything else that needs the
// profile) makes first: nil when the account has a profile, ErrProfileRequired
// when it hasn't.
func (s *Service) RequireProfile(ctx context.Context, accountID string) error {
	_, err := s.Profile(ctx, accountID)
	return err
}

// checkAnswers refuses answers that are malformed, whatever the person's age.
func checkAnswers(in ProfileInput, first bool) error {
	name := strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(name); n < policy.NameMinLength || n > policy.NameMaxLength {
		return fmt.Errorf("%w: the name is %d to %d characters", ErrInvalid, policy.NameMinLength, policy.NameMaxLength)
	}
	switch in.Gender {
	case GenderWoman, GenderMan, GenderNonBinary, GenderPreferNotToSay:
	default:
		return fmt.Errorf("%w: gender is one of woman, man, non-binary or prefer-not-to-say", ErrInvalid)
	}
	if len(in.Languages) == 0 {
		return fmt.Errorf("%w: at least one spoken language", ErrInvalid)
	}
	for _, l := range in.Languages {
		if !languageCode.MatchString(l) {
			return fmt.Errorf("%w: %q is not a language code", ErrInvalid, l)
		}
	}
	if first && !in.AcceptedCodeOfConduct {
		return fmt.Errorf("%w: the code of conduct must be accepted", ErrInvalid)
	}
	return nil
}

func eligibleForWomenOnly(g Gender) bool { return g == GenderWoman || g == GenderNonBinary }

// checkAdult says whether a person born on dob is policy.MinAge or over on the
// city's date today. Both are "2006-01-02"; comparing dates, not instants, is
// the whole point of using the city's clock.
func checkAdult(dob, today string) error {
	born, err := time.Parse(time.DateOnly, dob)
	if err != nil {
		return fmt.Errorf("%w: the date of birth is not a date", ErrInvalid)
	}
	day, err := time.Parse(time.DateOnly, today)
	if err != nil {
		return fmt.Errorf("people: today %q: %w", today, err)
	}
	// AddDate on 29 February lands on 1 March in a common year, so someone born
	// on the 29th turns 18 on 1 March.
	if born.AddDate(policy.MinAge, 0, 0).After(day) {
		return ErrUnderage
	}
	return nil
}

func cityError(err error) error {
	if errors.Is(err, cities.ErrNotFound) {
		return fmt.Errorf("%w: city is not listed", ErrInvalid)
	}
	return err
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, l := range in {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}

func profileOf(r store.AppProfile) Profile {
	return Profile{
		Name: r.Name, Gender: Gender(r.Gender), Languages: r.Languages, LivesInCity: r.LivesInCity,
		OnlyWomenAndNonBinary: r.OnlyWomenAndNonBinary, CodeOfConductVersion: int(r.CodeOfConductVersion),
	}
}
