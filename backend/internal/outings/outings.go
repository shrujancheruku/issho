// Package outings owns outings and their seats: the group matching formed on a
// date for a plan, and each person's place in it (docs/spec/outings.md). The
// tables are app.outings and app.seats (db/migrations/00007_outings.sql); no
// other package reads them.
//
// It forms outings from a plan's signups and makes their invitations, takes
// each invited person's yes or no for 48 hours, sets an outing at its third yes
// and lets a person going cancel. Check-in and feedback are later tickets'.
package outings

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/jobs"
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
// 48 hours to say yes or no. A yes while the outing isn't set is SeatAccepted,
// which setting the outing makes SeatGoing; a no is SeatDeclined, and an
// invitation nobody answered in its 48 hours is SeatUnanswered. A person going
// who cancels is SeatCancelled. SeatFull, SeatLateCancel and SeatRemoved are
// the later tickets'.
const (
	SeatInvited    SeatState = "invited"
	SeatAccepted   SeatState = "accepted"
	SeatGoing      SeatState = "going"
	SeatDeclined   SeatState = "declined"
	SeatUnanswered SeatState = "unanswered"
	SeatFull       SeatState = "full"
	SeatCancelled  SeatState = "cancelled"
	SeatLateCancel SeatState = "late_cancel"
	SeatRemoved    SeatState = "removed"
)

// OutingState is where an outing stands (docs/spec/outings.md, "What an outing
// is").
type OutingState string

// Outing states, as stored. Forming makes an outing offered: its invitations
// are out and fewer than 3 have said yes. The third yes sets it; with fewer
// when the last invitation's 48 hours end it is not set. Proposed, over and
// cancelled are the outing in full's (milestones 4 and 5) and are never made
// yet.
const (
	OutingProposed  OutingState = "proposed"
	OutingOffered   OutingState = "offered"
	OutingSet       OutingState = "set"
	OutingOver      OutingState = "over"
	OutingNotSet    OutingState = "not_set"
	OutingCancelled OutingState = "cancelled"
)

// Answer is an invited person's reply: yes or no.
type Answer string

// The two answers.
const (
	Yes Answer = "yes"
	No  Answer = "no"
)

// The ways a call on an outing is refused.
var (
	// ErrNotFound: no such outing, or the caller has no seat in it. The two are
	// told apart to nobody, so no one learns which outings exist.
	ErrNotFound = errors.New("outings: no such outing")
	// ErrInvalid: an answer that is neither yes nor no.
	ErrInvalid = errors.New("outings: invalid answer")
	// ErrInvitationClosed: the invitation's 48 hours are over, or the seat or
	// the outing is in a state that takes no answer (a going person cancels, they
	// don't answer no; an outing not set takes none).
	ErrInvitationClosed = errors.New("outings: the invitation is closed")
	// ErrNotGoing: only a person going can cancel.
	ErrNotGoing = errors.New("outings: the seat is not going")
)

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
	// OutingState is where the whole outing stands.
	OutingState OutingState
	// YesCount is how many have said yes: those going and those whose yes waits
	// for the outing to be set. It is a number, never who.
	YesCount int
	// GoingFirstNames are the first names of the others going, sorted: only for
	// a person who is going themselves, and never their own. For anyone else it
	// is empty. A first name is all anyone sees of another person.
	GoingFirstNames []string
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
	// queue only inserts jobs, in the transaction that causes them; the worker
	// process has the client that works them.
	queue    *river.Client[pgx.Tx]
	queueErr error
}

// New returns a Service that stamps times from clk, keeps outings in pool, reads
// a round's signups from signups and the facts matching needs of each person
// from ppl.
func New(clk clock.Clock, pool *pgxpool.Pool, signups *plans.Signups, ppl *people.Service) *Service {
	queue, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: jobs.Schema})
	return &Service{clock: clk, pool: pool, signups: signups, people: ppl, queue: queue, queueErr: err}
}

// Form groups the signups to the plan's running round and stores each group as
// an outing with an invited seat for each person, all in one transaction, and
// returns how many outings it made. It forms again from whoever has no seat in
// the round yet and nobody else: running it twice for the same signups makes
// no second outing. A plan that isn't shown, or isn't there, is
// plans.ErrNotFound and forms nothing.
func (s *Service) Form(ctx context.Context, planID string) (int, error) {
	if s.queueErr != nil {
		return 0, fmt.Errorf("outings: form: %w", s.queueErr)
	}
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
			StartsAt: round.StartsAt, Languages: g.Languages, State: string(OutingOffered), CreatedAt: now,
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
		// The job that records the unanswered seats is made with the invitations,
		// in their transaction: it exists if they do, and not otherwise.
		if _, err := s.queue.InsertTx(ctx, tx, ExpireArgs{OutingID: id}, &river.InsertOpts{ScheduledAt: now.Add(policy.InvitationWindow)}); err != nil {
			return 0, fmt.Errorf("outings: form: job: %w", err)
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
	seats := make([]Seat, len(rows))
	for i, r := range rows {
		seats[i] = seatOf(r.OutingID, r.PlanID, r.Date, r.Part, r.StartsAt, r.Languages, r.OutingState, r.State, r.InvitedAt, r.InvitationEndsAt)
	}
	if err := s.reveal(ctx, accountID, seats); err != nil {
		return nil, fmt.Errorf("outings: mine: %w", err)
	}
	return seats, nil
}

// seatOf is a seat as stored, before what the group shows of itself is added.
func seatOf(outing, plan string, date time.Time, part, startsAt string, languages []string, outingState, state string, invitedAt, endsAt time.Time) Seat {
	return Seat{
		OutingID: outing, PlanID: plan, Date: date.Format(time.DateOnly), Part: part, StartsAt: startsAt,
		Languages: languages, State: SeatState(state), OutingState: OutingState(outingState),
		InvitedAt: invitedAt.UTC(), InvitationEndsAt: endsAt.UTC(),
	}
}

// reveal adds what a person sees of the others to their seats: how many have
// said yes, to everyone with a seat, and the first names of the others going,
// to a person who is going themselves and to nobody else.
func (s *Service) reveal(ctx context.Context, accountID string, seats []Seat) error {
	if len(seats) == 0 {
		return nil
	}
	q := store.New(s.pool)
	ids := make([]string, len(seats))
	for i, seat := range seats {
		ids[i] = seat.OutingID
	}
	counts, err := q.CountYeses(ctx, ids)
	if err != nil {
		return err
	}
	yeses := make(map[string]int, len(counts))
	for _, c := range counts {
		yeses[c.OutingID] = int(c.Yeses)
	}
	var going []string
	for _, seat := range seats {
		if seat.State == SeatGoing {
			going = append(going, seat.OutingID)
		}
	}
	others := map[string][]string{} // outing -> account ids going, but the caller
	var accounts []string
	if len(going) > 0 {
		rows, err := q.ListGoing(ctx, going)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.AccountID != accountID {
				others[r.OutingID] = append(others[r.OutingID], r.AccountID)
				accounts = append(accounts, r.AccountID)
			}
		}
	}
	names, err := s.people.FirstNames(ctx, accounts)
	if err != nil {
		return err
	}
	for i := range seats {
		seats[i].YesCount = yeses[seats[i].OutingID]
		if seats[i].State != SeatGoing {
			continue
		}
		for _, id := range others[seats[i].OutingID] {
			if name, ok := names[id]; ok {
				seats[i].GoingFirstNames = append(seats[i].GoingFirstNames, name)
			}
		}
		slices.Sort(seats[i].GoingFirstNames)
	}
	return nil
}

// uuid is the shape of an id; anything else can't name an outing, and the
// database would refuse it with an error that isn't "not found".
var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// seat is one of the caller's own seats as Mine shows it.
func (s *Service) seat(ctx context.Context, accountID, outingID string) (Seat, error) {
	r, err := store.New(s.pool).GetSeatOfAccount(ctx, store.GetSeatOfAccountParams{AccountID: accountID, ID: outingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Seat{}, ErrNotFound
	}
	if err != nil {
		return Seat{}, fmt.Errorf("outings: seat: %w", err)
	}
	seats := []Seat{seatOf(r.OutingID, r.PlanID, r.Date, r.Part, r.StartsAt, r.Languages, r.OutingState, r.State, r.InvitedAt, r.InvitationEndsAt)}
	if err := s.reveal(ctx, accountID, seats); err != nil {
		return Seat{}, fmt.Errorf("outings: seat: %w", err)
	}
	return seats[0], nil
}

// Answer records the account's yes or no to its invitation to the outing, and
// returns the seat as the account sees it. An invitation takes answers for its
// fixed 48 hours (policy.InvitationWindow); a no can be changed to a yes inside
// them. A yes is held until the outing is set: the third (policy.YesesToSet)
// sets it, makes everyone who said yes going and takes that time off their
// other signups, all in one transaction; a yes after the outing is set is going
// at once. A person with no seat in the outing, and an outing that isn't there,
// are ErrNotFound alike. ErrInvitationClosed for an answer after the 48 hours,
// for an outing that was not set, and for a no from someone going (they
// cancel); a yes from someone already going is a no-op.
func (s *Service) Answer(ctx context.Context, accountID, outingID string, a Answer) (Seat, error) {
	if a != Yes && a != No {
		return Seat{}, ErrInvalid
	}
	if !uuid.MatchString(outingID) {
		return Seat{}, ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Seat{}, fmt.Errorf("outings: answer: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)
	o, seat, err := s.lockSeat(ctx, q, accountID, outingID)
	if err != nil {
		return Seat{}, err
	}

	now := s.clock.Now()
	state := SeatState(seat.State)
	switch {
	case state == SeatGoing && a == Yes:
		// Already going: saying so again changes nothing.
		return s.seat(ctx, accountID, outingID)
	case state != SeatInvited && state != SeatAccepted && state != SeatDeclined,
		OutingState(o.State) != OutingOffered && OutingState(o.State) != OutingSet,
		!now.Before(seat.InvitationEndsAt):
		return Seat{}, ErrInvitationClosed
	}

	if a == No {
		if err := q.SetSeatState(ctx, store.SetSeatStateParams{OutingID: outingID, AccountID: accountID, State: string(SeatDeclined)}); err != nil {
			return Seat{}, fmt.Errorf("outings: answer: %w", err)
		}
	} else if err := s.sayYes(ctx, tx, q, o, accountID); err != nil {
		return Seat{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Seat{}, fmt.Errorf("outings: answer: %w", err)
	}
	return s.seat(ctx, accountID, outingID)
}

// lockSeat locks the outing and reads the account's seat in it. Both a missing
// outing and a missing seat are ErrNotFound.
func (s *Service) lockSeat(ctx context.Context, q *store.Queries, accountID, outingID string) (store.LockOutingRow, store.GetSeatRow, error) {
	o, err := q.LockOuting(ctx, outingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.LockOutingRow{}, store.GetSeatRow{}, ErrNotFound
	}
	if err != nil {
		return store.LockOutingRow{}, store.GetSeatRow{}, fmt.Errorf("outings: lock: %w", err)
	}
	seat, err := q.GetSeat(ctx, store.GetSeatParams{OutingID: outingID, AccountID: accountID})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.LockOutingRow{}, store.GetSeatRow{}, ErrNotFound
	}
	if err != nil {
		return store.LockOutingRow{}, store.GetSeatRow{}, fmt.Errorf("outings: seat: %w", err)
	}
	return o, seat, nil
}

// sayYes records a yes, with the outing locked. In a set outing the person is
// going at once. Otherwise the yes waits, and if it is the YesesToSet-th the
// outing is set and everyone who said yes is going.
func (s *Service) sayYes(ctx context.Context, tx pgx.Tx, q *store.Queries, o store.LockOutingRow, accountID string) error {
	set := func(account string, state SeatState) error {
		return q.SetSeatState(ctx, store.SetSeatStateParams{OutingID: o.ID, AccountID: account, State: string(state)})
	}
	if OutingState(o.State) == OutingSet {
		if err := set(accountID, SeatGoing); err != nil {
			return fmt.Errorf("outings: answer: %w", err)
		}
		return s.busy(ctx, tx, o, accountID)
	}
	if err := set(accountID, SeatAccepted); err != nil {
		return fmt.Errorf("outings: answer: %w", err)
	}
	counts, err := q.CountYeses(ctx, []string{o.ID})
	if err != nil {
		return fmt.Errorf("outings: answer: %w", err)
	}
	if len(counts) == 0 || counts[0].Yeses < policy.YesesToSet {
		return nil
	}
	if err := q.SetOutingState(ctx, store.SetOutingStateParams{ID: o.ID, State: string(OutingSet)}); err != nil {
		return fmt.Errorf("outings: answer: %w", err)
	}
	going, err := q.PromoteAcceptedSeats(ctx, o.ID)
	if err != nil {
		return fmt.Errorf("outings: answer: %w", err)
	}
	for _, id := range going {
		if err := s.busy(ctx, tx, o, id); err != nil {
			return err
		}
	}
	return nil
}

// busy takes the outing's date and part of the day off the account's other
// signups: a person going to an outing isn't grouped for the same time again.
func (s *Service) busy(ctx context.Context, tx pgx.Tx, o store.LockOutingRow, accountID string) error {
	if err := s.signups.ReleaseTime(ctx, tx, accountID, o.Date.Format(time.DateOnly), o.Part, o.RoundID); err != nil {
		return fmt.Errorf("outings: answer: %w", err)
	}
	return nil
}

// Cancel is a person going giving their seat up: the seat records it as
// cancelled and the person stops being counted among those going. Only someone
// going can cancel (ErrNotGoing); no seat in the outing is ErrNotFound. What
// a cancel costs the person, and the outing it leaves with too few going, are
// the later tickets' (see the README).
func (s *Service) Cancel(ctx context.Context, accountID, outingID string) (Seat, error) {
	if !uuid.MatchString(outingID) {
		return Seat{}, ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Seat{}, fmt.Errorf("outings: cancel: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)
	_, seat, err := s.lockSeat(ctx, q, accountID, outingID)
	if err != nil {
		return Seat{}, err
	}
	if SeatState(seat.State) != SeatGoing {
		return Seat{}, ErrNotGoing
	}
	if err := q.SetSeatState(ctx, store.SetSeatStateParams{OutingID: outingID, AccountID: accountID, State: string(SeatCancelled)}); err != nil {
		return Seat{}, fmt.Errorf("outings: cancel: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Seat{}, fmt.Errorf("outings: cancel: %w", err)
	}
	return s.seat(ctx, accountID, outingID)
}

// Expire closes the invitations of the outing that have run their 48 hours: a
// seat still invited is recorded as unanswered, and an outing still offered when
// its last invitation has ended is not set, with those who said yes left as
// they are (left waiting). It is what the job made with the invitations calls,
// and it may be called again or early: it closes only what is due, so a second
// run, or one before the 48 hours, changes nothing. An outing that isn't there
// is nothing to close.
func (s *Service) Expire(ctx context.Context, outingID string) error {
	if !uuid.MatchString(outingID) {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("outings: expire: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)
	o, err := q.LockOuting(ctx, outingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("outings: expire: %w", err)
	}
	now := s.clock.Now()
	if err := q.ExpireUnansweredSeats(ctx, store.ExpireUnansweredSeatsParams{OutingID: outingID, InvitationEndsAt: now}); err != nil {
		return fmt.Errorf("outings: expire: %w", err)
	}
	if OutingState(o.State) == OutingOffered {
		last, err := q.LastInvitationEnd(ctx, outingID)
		if err != nil {
			return fmt.Errorf("outings: expire: %w", err)
		}
		if !now.Before(last) {
			if err := q.SetOutingState(ctx, store.SetOutingStateParams{ID: outingID, State: string(OutingNotSet)}); err != nil {
				return fmt.Errorf("outings: expire: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("outings: expire: %w", err)
	}
	return nil
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
