package outings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/internal/outings/store"
)

// Companion is another person of an outing as feedback names them: a first name,
// which is all anyone sees of another person, and the handle the seat goes by in
// this outing. The handle is random, means nothing outside the outing and is not
// an account, an email or anything that could be linked to one.
type Companion struct {
	Handle    string
	FirstName string
}

// FeedbackInput is a person's answers to feedback's first three questions.
// NotThere are the handles of the people they say were not there; everyone else
// is ticked, which is the default. MeetAgain are the handles of the people they
// would meet again. Both are ignored when Went is false: a "no" to "did you go?"
// ends the feedback there.
type FeedbackInput struct {
	Went      bool
	NotThere  []string
	MeetAgain []string
}

// Feedback is the form as one person sees it: who else they can tick, and their
// answers so far. It holds nobody's account and nothing about who picked them.
type Feedback struct {
	OutingID string
	// Answered is whether the person has answered "did you go?" yet; Went is that
	// answer, and meaningless before it.
	Answered bool
	Went     bool
	// Others are everyone who was going at the start but this person, by first
	// name and then handle.
	Others []Companion
	// NotThere and MeetAgain are this person's own answers, by handle, sorted.
	NotThere, MeetAgain []string
	// ClosesAt is when feedback closes, policy.FeedbackWindow after the plan's
	// end.
	ClosesAt time.Time
}

// Feedback reads the form of a person who was going at the start, while feedback
// is open: from the plan's stated end for policy.FeedbackWindow. ErrNotFound for
// no seat, ErrNotThere for a seat that wasn't going at the start (decided from
// its history, not its state now), ErrFeedbackNotOpen before the end and
// ErrFeedbackClosed from the moment the 7 days are over.
func (s *Service) Feedback(ctx context.Context, accountID, outingID string) (Feedback, error) {
	return s.withFeedback(ctx, accountID, outingID, func(q *store.Queries, f feedbackScope) (Feedback, error) {
		return s.form(ctx, q, f)
	})
}

// SaveFeedback records the person's answers, replacing any they gave before, and
// returns the form as it now stands. It is open as Feedback is, and refuses with
// ErrInvalid a handle that is no other person who was going at the start, and a
// pick of someone the same answer says was not there. A "no" keeps nothing but
// itself: any picks and unticks from before are dropped.
func (s *Service) SaveFeedback(ctx context.Context, accountID, outingID string, in FeedbackInput) (Feedback, error) {
	return s.withFeedback(ctx, accountID, outingID, func(q *store.Queries, f feedbackScope) (Feedback, error) {
		var unticked, picked []string // account ids
		if in.Went {
			var err error
			if unticked, err = f.accounts(in.NotThere); err != nil {
				return Feedback{}, err
			}
			if picked, err = f.accounts(in.MeetAgain); err != nil {
				return Feedback{}, err
			}
			for _, id := range picked {
				if slices.Contains(unticked, id) {
					return Feedback{}, fmt.Errorf("%w: a pick of someone said not to be there", ErrInvalid)
				}
			}
		}
		now := s.clock.Now()
		if err := q.UpsertFeedback(ctx, store.UpsertFeedbackParams{OutingID: f.outing, AccountID: accountID, Went: in.Went, AnsweredAt: now}); err != nil {
			return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
		}
		if err := q.ClearUnticks(ctx, store.ClearUnticksParams{OutingID: f.outing, RaterID: accountID}); err != nil {
			return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
		}
		if err := q.ClearPicks(ctx, store.ClearPicksParams{OutingID: f.outing, PickerID: accountID}); err != nil {
			return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
		}
		for _, id := range unticked {
			if err := q.InsertUntick(ctx, store.InsertUntickParams{OutingID: f.outing, RaterID: accountID, SubjectID: id}); err != nil {
				return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
			}
		}
		for _, id := range picked {
			if err := q.InsertPick(ctx, store.InsertPickParams{OutingID: f.outing, PickerID: accountID, PickedID: id, CreatedAt: now}); err != nil {
				return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
			}
		}
		return s.form(ctx, q, f)
	})
}

// Attended is whether the account attended the outing: they checked in or said so
// in feedback, and their own "I didn't go" wins over a check-in
// (docs/spec/outings.md, "Attended, met and no-show"). What others said of them
// is the later tickets'. ErrNotFound for no seat.
func (s *Service) Attended(ctx context.Context, accountID, outingID string) (bool, error) {
	if !uuid.MatchString(outingID) {
		return false, ErrNotFound
	}
	attended, err := store.New(s.pool).IsAttended(ctx, store.IsAttendedParams{OutingID: outingID, AccountID: accountID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("outings: attended: %w", err)
	}
	return attended, nil
}

// feedbackScope is what answering knows once the person has been let in: the
// outing, who was going at its start and when feedback closes.
type feedbackScope struct {
	outing, account string
	closes          time.Time
	// others are the people going at the start besides the account, with the
	// handle of each.
	others []store.ListGoingAtStartRow
}

// accounts are the accounts the handles name, each once, in account order; a
// handle that names no other person who was going at the start is ErrInvalid.
func (f feedbackScope) accounts(handles []string) ([]string, error) {
	var out []string
	for _, h := range handles {
		i := slices.IndexFunc(f.others, func(o store.ListGoingAtStartRow) bool { return o.Handle == strings.ToLower(h) })
		if i < 0 {
			return nil, fmt.Errorf("%w: no such person in the outing", ErrInvalid)
		}
		out = append(out, f.others[i].AccountID)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// withFeedback runs fn in a transaction with the outing locked, once the person
// is known to have been going at the start and feedback is open, and commits what
// fn wrote.
func (s *Service) withFeedback(ctx context.Context, accountID, outingID string, fn func(*store.Queries, feedbackScope) (Feedback, error)) (Feedback, error) {
	if !uuid.MatchString(outingID) {
		return Feedback{}, ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)
	o, seat, err := s.lockSeat(ctx, q, accountID, outingID)
	if err != nil {
		return Feedback{}, err
	}
	w, err := windowOf(o.Date, o.StartsAt, o.EndsAt, o.TimeZone)
	if err != nil {
		return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
	}
	// Who was going at the start comes from the seat's history: a person who
	// cancelled after the start was going at it, and one who cancelled before
	// it was not, whatever the state of the seat now.
	if seat.GoingAt == nil || seat.GoingAt.After(w.start) || (seat.LeftGoingAt != nil && !seat.LeftGoingAt.After(w.start)) {
		return Feedback{}, ErrNotThere
	}
	now := s.clock.Now()
	switch {
	case now.Before(w.end):
		return Feedback{}, ErrFeedbackNotOpen
	case !now.Before(w.feedbackCloses):
		return Feedback{}, ErrFeedbackClosed
	}
	present, err := q.ListGoingAtStart(ctx, store.ListGoingAtStartParams{OutingID: outingID, StartsAt: w.start})
	if err != nil {
		return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
	}
	f := feedbackScope{outing: outingID, account: accountID, closes: w.feedbackCloses}
	for _, p := range present {
		if p.AccountID != accountID {
			f.others = append(f.others, p)
		}
	}
	out, err := fn(q, f)
	if err != nil {
		return Feedback{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
	}
	return out, nil
}

// form is the person's form as it stands, read in the transaction that answers.
func (s *Service) form(ctx context.Context, q *store.Queries, f feedbackScope) (Feedback, error) {
	out := Feedback{OutingID: f.outing, ClosesAt: f.closes}
	ids := make([]string, len(f.others))
	handles := make(map[string]string, len(f.others)) // account -> handle
	for i, o := range f.others {
		ids[i] = o.AccountID
		handles[o.AccountID] = o.Handle
	}
	names, err := s.people.FirstNames(ctx, ids)
	if err != nil {
		return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
	}
	for _, o := range f.others {
		out.Others = append(out.Others, Companion{Handle: o.Handle, FirstName: names[o.AccountID]})
	}
	slices.SortFunc(out.Others, func(a, b Companion) int {
		if c := strings.Compare(a.FirstName, b.FirstName); c != 0 {
			return c
		}
		return strings.Compare(a.Handle, b.Handle)
	})

	own, err := q.GetFeedback(ctx, store.GetFeedbackParams{OutingID: f.outing, AccountID: f.account})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
	}
	out.Answered, out.Went = true, own.Went
	unticked, err := q.ListUnticks(ctx, store.ListUnticksParams{OutingID: f.outing, RaterID: f.account})
	if err != nil {
		return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
	}
	picked, err := q.ListPicksBy(ctx, store.ListPicksByParams{OutingID: f.outing, PickerID: f.account})
	if err != nil {
		return Feedback{}, fmt.Errorf("outings: feedback: %w", err)
	}
	for _, id := range unticked {
		out.NotThere = append(out.NotThere, handles[id])
	}
	for _, id := range picked {
		out.MeetAgain = append(out.MeetAgain, handles[id])
	}
	slices.Sort(out.NotThere)
	slices.Sort(out.MeetAgain)
	return out, nil
}
