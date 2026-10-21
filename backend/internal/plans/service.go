package plans

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

var (
	// ErrNoProfile means the caller hasn't finished onboarding.
	ErrNoProfile = errors.New("no profile yet")
	// ErrNotFound means no open plan with that id in the caller's city.
	ErrNotFound = errors.New("plan not found")
	// ErrNotForYou means the caller doesn't speak the plan's language(s).
	ErrNotForYou = errors.New("you don't speak this plan's language")
)

// InvalidError is a signup the caller has to correct.
type InvalidError struct{ msg string }

func (e *InvalidError) Error() string { return e.msg }

func invalid(format string, a ...any) error { return &InvalidError{fmt.Sprintf(format, a...)} }

// Service is what PlanService does for one caller.
type Service struct {
	DB *pgxpool.Pool
}

// FeedItem is an open plan as one person sees it.
type FeedItem struct {
	Open
	Level    matching.Level
	Minutes  int // from their home area; -1 = no route
	SignedUp bool
}

// feed loads the caller and ranks their city's open plans for them.
func (s *Service) feed(ctx context.Context, q *dbgen.Queries, userID uuid.UUID) ([]FeedItem, *matching.User, error) {
	u, cityID, err := LoadUser(ctx, q, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNoProfile
	}
	if err != nil {
		return nil, nil, err
	}
	// Any month: a plan that's open stays shown whatever the season.
	c, err := loadPlaces(ctx, q, cityID, 0)
	if err != nil {
		return nil, nil, err
	}
	acts := make(map[matching.ActivityID]*matching.Activity, len(c.Activities))
	for i := range c.Activities {
		acts[c.Activities[i].ID] = &c.Activities[i]
	}
	open, err := ListOpen(ctx, q, cityID)
	if err != nil {
		return nil, nil, err
	}
	// The worker withdraws plans whose activity was disabled; until it runs,
	// they're hidden.
	open = slices.DeleteFunc(open, func(p Open) bool { return acts[p.Activity] == nil })
	mine, err := q.ListUserOpenSignups(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	plans := make([]matching.OpenPlan, len(open))
	// matching.Feed returns plans, not ids; Suggest never opens the same
	// plan twice, but a queue per plan keeps this right regardless.
	byPlan := map[matching.Plan][]Open{}
	for i, p := range open {
		plans[i] = p.OpenPlan
		byPlan[p.Plan] = append(byPlan[p.Plan], p)
	}
	ranked := matching.Feed(u, plans, acts, c.Travel, c.Config)
	out := make([]FeedItem, len(ranked))
	for i, r := range ranked {
		p := byPlan[r.Plan][0]
		byPlan[r.Plan] = byPlan[r.Plan][1:]
		out[i] = FeedItem{Open: p, Level: r.Level, Minutes: r.Minutes, SignedUp: slices.Contains(mine, p.ID)}
	}
	return out, u, nil
}

// Feed returns every open plan in the caller's city, best for them first.
func (s *Service) Feed(ctx context.Context, userID uuid.UUID) ([]FeedItem, error) {
	items, _, err := s.feed(ctx, dbgen.New(s.DB), userID)
	return items, err
}

// Plan returns one open plan as the caller sees it, with their signup (nil
// if they haven't signed up).
func (s *Service) Plan(ctx context.Context, userID, planID uuid.UUID) (FeedItem, *matching.Signup, error) {
	q := dbgen.New(s.DB)
	item, _, err := s.find(ctx, q, userID, planID)
	if err != nil || !item.SignedUp {
		return item, nil, err
	}
	signup, err := loadSignup(ctx, q, planID, userID)
	return item, signup, err
}

func (s *Service) find(ctx context.Context, q *dbgen.Queries, userID, planID uuid.UUID) (FeedItem, *matching.User, error) {
	items, u, err := s.feed(ctx, q, userID)
	if err != nil {
		return FeedItem{}, nil, err
	}
	for _, it := range items {
		if it.ID == planID {
			return it, u, nil
		}
	}
	return FeedItem{}, nil, ErrNotFound
}

// SignUp signs the caller up for a plan, or replaces the times they gave.
// signup.User is set from userID.
func (s *Service) SignUp(ctx context.Context, userID, planID uuid.UUID, signup matching.Signup) (FeedItem, error) {
	q := dbgen.New(s.DB)
	item, _, err := s.find(ctx, q, userID, planID)
	if err != nil {
		return FeedItem{}, err
	}
	if item.Level == matching.NotForYou {
		return FeedItem{}, ErrNotForYou
	}
	end := item.Window.Start.AddDate(0, 0, item.Window.Days)
	for _, o := range signup.Unavailable {
		if o.Date.Before(item.Window.Start) || !o.Date.Before(end) {
			return FeedItem{}, invalid("unavailable: %s is outside the plan's window (%s to %s)",
				o.Date.Format(time.DateOnly), item.Window.Start.Format(time.DateOnly), end.AddDate(0, 0, -1).Format(time.DateOnly))
		}
	}
	signup.User = matching.UserID(userID.String())
	if err := SaveSignup(ctx, s.DB, planID, signup); err != nil {
		if errors.Is(err, ErrClosed) {
			return FeedItem{}, ErrNotFound
		}
		return FeedItem{}, err
	}
	if !item.SignedUp {
		item.SignedUp = true
		item.Signups++
	}
	return item, nil
}

// Withdraw removes the caller's signup, if any.
func (s *Service) Withdraw(ctx context.Context, userID, planID uuid.UUID) error {
	return Withdraw(ctx, dbgen.New(s.DB), planID, userID)
}

// loadSignup reads one person's signup times for a plan, as they gave them
// (without the events they're already going to, which LoadSignups adds).
func loadSignup(ctx context.Context, q *dbgen.Queries, planID, userID uuid.UUID) (*matching.Signup, error) {
	key := dbgen.ListUserSignupSlotsParams{PlanID: planID, UserID: userID}
	slots, err := q.ListUserSignupSlots(ctx, key)
	if err != nil {
		return nil, err
	}
	out := &matching.Signup{User: matching.UserID(userID.String())}
	for _, r := range slots {
		out.Pattern = append(out.Pattern, matching.Slot{Weekday: time.Weekday(r.Weekday), Block: matching.Block(r.Block)})
	}
	unavailable, err := q.ListUserSignupUnavailable(ctx, dbgen.ListUserSignupUnavailableParams(key))
	if err != nil {
		return nil, err
	}
	for _, r := range unavailable {
		b := matching.Block(r.Block)
		if r.Block == wholeDay {
			b = ""
		}
		out.Unavailable = append(out.Unavailable, matching.Occasion{Date: r.Date.Time, Block: b})
	}
	return out, nil
}
