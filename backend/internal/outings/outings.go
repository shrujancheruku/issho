// Package outings owns outings and their seats: the group matching formed on a
// date for a plan, and each person's place in it (docs/spec/outings.md). The
// tables are app.outings and app.seats (db/migrations/00007_outings.sql); no
// other package reads them.
//
// So far it forms outings from a plan's signups and makes their invitations.
// Answering an invitation, going, check-in and feedback are later tickets'.
package outings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/outings/store"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/policy"
)

// SeatState is where a seat stands: the stage of a person in the outing, or
// how they left it (docs/spec/outings.md, "What an outing is").
type SeatState string

// The states a seat can be in, as stored (docs/spec/outings.md, "What an outing
// is"). Forming makes every seat SeatInvited: a date is set and the person has
// 48 hours to say yes or no. The rest are what answering an invitation, going
// and leaving make of it, which are the next tickets'.
const (
	SeatInvited    SeatState = "invited"
	SeatGoing      SeatState = "going"
	SeatDeclined   SeatState = "declined"
	SeatUnanswered SeatState = "unanswered"
	SeatFull       SeatState = "full"
	SeatCancelled  SeatState = "cancelled"
	SeatLateCancel SeatState = "late_cancel"
	SeatRemoved    SeatState = "removed"
)

// Outing states, as stored. Forming makes an outing offered: its invitations
// are out and fewer than 3 have said yes.
const stateOffered = "offered"

// Seat is one person's place in one outing, with what the person is shown of
// the outing: the date, the time and the languages the group shares. It holds
// no other person: a name is never in it.
type Seat struct {
	OutingID string
	PlanID   string
	// Date is the city's calendar date, "YYYY-MM-DD"; Part the part of the day
	// the group ticked; StartsAt the plan's start, "HH:MM" on the city's clock.
	Date     string
	Part     string
	StartsAt string
	// Languages are every language the whole group speaks, sorted.
	Languages []string
	State     SeatState
	// InvitedAt is when the invitation went out; InvitationEndsAt is when its
	// fixed 48 hours (policy.InvitationWindow) end.
	InvitedAt        time.Time
	InvitationEndsAt time.Time
}

// Service is the exported API of outings.
type Service struct {
	clock   clock.Clock
	pool    *pgxpool.Pool
	signups *plans.Signups
	people  *people.Service
}

// New returns a Service that stamps times from clk, keeps outings in pool, reads
// a round's signups from signups and the facts matching needs of each person
// from ppl.
func New(clk clock.Clock, pool *pgxpool.Pool, signups *plans.Signups, ppl *people.Service) *Service {
	return &Service{clock: clk, pool: pool, signups: signups, people: ppl}
}

// Form groups the signups to the plan's running round and stores each group as
// an outing with an invited seat for each person, all in one transaction, and
// returns how many outings it made. It forms again from whoever has no seat in
// the round yet and nobody else: running it twice for the same signups makes
// no second outing. A plan that isn't shown, or isn't there, is
// plans.ErrNotFound and forms nothing.
func (s *Service) Form(ctx context.Context, planID string) (int, error) {
	round, err := s.signups.Forming(ctx, planID)
	if err != nil {
		return 0, fmt.Errorf("outings: form: %w", err)
	}
	if round.RoundID == "" || len(round.Entries) == 0 {
		return 0, nil
	}
	facts, err := s.facts(ctx, round.Entries)
	if err != nil {
		return 0, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("outings: form: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)
	if err := q.LockRound(ctx, round.RoundID); err != nil {
		return 0, fmt.Errorf("outings: form: lock: %w", err)
	}
	seated, err := q.ListSeatedInRound(ctx, round.RoundID)
	if err != nil {
		return 0, fmt.Errorf("outings: form: %w", err)
	}

	in := matching.Input{}
	for _, e := range round.Entries {
		f, ok := facts[e.Account]
		if !ok || slices.Contains(seated, e.Account) {
			continue
		}
		in.Signups = append(in.Signups, f.signup(e))
	}
	groups := matching.Match(in)

	now := s.clock.Now()
	for _, g := range groups {
		date, err := time.Parse(time.DateOnly, g.Date)
		if err != nil {
			return 0, fmt.Errorf("outings: form: matching's date %q: %w", g.Date, err)
		}
		id, err := q.InsertOuting(ctx, store.InsertOutingParams{
			PlanID: round.PlanID, RoundID: round.RoundID, Date: date, Part: string(g.Part),
			StartsAt: round.StartsAt, Languages: g.Languages, State: stateOffered, CreatedAt: now,
		})
		if err != nil {
			return 0, fmt.Errorf("outings: form: outing: %w", err)
		}
		for _, p := range g.People {
			if err := q.InsertSeat(ctx, store.InsertSeatParams{
				OutingID: id, RoundID: round.RoundID, AccountID: string(p), State: string(SeatInvited),
				InvitedAt: now, InvitationEndsAt: now.Add(policy.InvitationWindow),
			}); err != nil {
				return 0, fmt.Errorf("outings: form: seat: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("outings: form: %w", err)
	}
	return len(groups), nil
}

// Mine returns the account's own seats, earliest date first. It takes the
// account, so it can't return a seat of anyone else's, and an outing the
// account has no seat in is never among them.
func (s *Service) Mine(ctx context.Context, accountID string) ([]Seat, error) {
	rows, err := store.New(s.pool).ListSeatsOfAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("outings: mine: %w", err)
	}
	out := make([]Seat, len(rows))
	for i, r := range rows {
		out[i] = Seat{
			OutingID: r.OutingID, PlanID: r.PlanID, Date: r.Date.Format(time.DateOnly), Part: r.Part,
			StartsAt: r.StartsAt, Languages: r.Languages, State: SeatState(r.State),
			InvitedAt: r.InvitedAt.UTC(), InvitationEndsAt: r.InvitationEndsAt.UTC(),
		}
	}
	return out, nil
}

// facts is what matching reads of one person besides their ticks.
type facts struct {
	languages []string
	gender    matching.Gender
	womenOnly bool
}

func (f facts) signup(e plans.Entry) matching.Signup {
	ticks := make([]matching.Slot, len(e.Ticks))
	for i, t := range e.Ticks {
		ticks[i] = matching.Slot{Date: t.Date, Part: matching.Part(t.Part)}
	}
	return matching.Signup{
		Person: matching.PersonID(e.Account), Ticks: ticks, Languages: f.languages,
		Gender: f.gender, WomenAndNonBinaryOnly: f.womenOnly,
	}
}

// facts reads the languages, gender and "only women and non-binary people"
// setting of each person in entries: the only parts of a profile matching
// reads (docs/spec/matching.md, "What matching is"). One call per person, as
// a round has tens of them; a person with no profile can't have signed up and
// is left out.
func (s *Service) facts(ctx context.Context, entries []plans.Entry) (map[string]facts, error) {
	out := make(map[string]facts, len(entries))
	for _, e := range entries {
		p, err := s.people.Profile(ctx, e.Account)
		if errors.Is(err, people.ErrProfileRequired) || errors.Is(err, people.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("outings: form: profile: %w", err)
		}
		out[e.Account] = facts{languages: p.Languages, gender: genderOf(p.Gender), womenOnly: p.OnlyWomenAndNonBinary}
	}
	return out, nil
}

// genderOf is the profile's answer in matching's terms: "prefer not to say" is
// Undeclared.
func genderOf(g people.Gender) matching.Gender {
	switch g {
	case people.GenderWoman:
		return matching.Woman
	case people.GenderMan:
		return matching.Man
	case people.GenderNonBinary:
		return matching.NonBinary
	default:
		return matching.Undeclared
	}
}
