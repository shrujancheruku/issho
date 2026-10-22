package plans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
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
	// Titles are composed in English and translated on demand; nil shows
	// them in English.
	Translations *translate.Service
	// Venue names and addresses, fetched live; nil leaves them out.
	Places maps.PlaceLookup
	Log    *slog.Logger // nil = slog.Default()
}

// FeedItem is an open plan as one person sees it.
type FeedItem struct {
	Open
	Level    matching.Level
	Minutes  int // from their home area; -1 = no route
	SignedUp bool
	// ClosedReason (a Closed* constant) and ClosedAt are set for a plan that
	// has closed; "" while it's open.
	ClosedReason string
	ClosedAt     time.Time
	// Title in the caller's two display languages (secondary "" when the
	// same or not chosen).
	Title, SecondaryTitle string
}

// Step is one stop of a plan: an activity at its venue, or (Meeting) the
// place people meet when the first activity has no venue.
type Step struct {
	Meeting  bool
	Activity matching.ActivityID // "" for the meeting point
	// Venue in the primary display language; zero when there's none or it
	// couldn't be fetched. SecondaryName is its name in the secondary one.
	Venue         maps.PlaceDetails
	SecondaryName string
	WalkMinutes   int // from the previous stop's venue; 0 = first or unknown
}

// feed loads the caller and ranks their city's open plans for them.
func (s *Service) feed(ctx context.Context, q *dbgen.Queries, userID uuid.UUID) ([]FeedItem, *matching.User, Caller, error) {
	u, caller, err := LoadUser(ctx, q, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, caller, ErrNoProfile
	}
	if err != nil {
		return nil, nil, caller, err
	}
	cityID := caller.City
	// Any month: a plan that's open stays shown whatever the season.
	c, err := loadPlaces(ctx, q, cityID, 0)
	if err != nil {
		return nil, nil, caller, err
	}
	acts := make(map[matching.ActivityID]*matching.Activity, len(c.Activities))
	for i := range c.Activities {
		acts[c.Activities[i].ID] = &c.Activities[i]
	}
	open, err := ListOpen(ctx, q, cityID)
	if err != nil {
		return nil, nil, caller, err
	}
	// The worker withdraws plans whose activity was disabled; until it runs,
	// they're hidden.
	open = slices.DeleteFunc(open, func(p Open) bool { return acts[p.Activity] == nil })
	mine, err := q.ListUserOpenSignups(ctx, userID)
	if err != nil {
		return nil, nil, caller, err
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
	plans = make([]matching.OpenPlan, len(out))
	for i := range out {
		plans[i] = out[i].OpenPlan
	}
	titles, secondary, err := s.titles(ctx, q, cityID, caller, plans)
	if err != nil {
		return nil, nil, caller, err
	}
	for i := range out {
		out[i].Title, out[i].SecondaryTitle = titles[i], secondary[i]
	}
	return out, u, caller, nil
}

// titleContext tells the translator what the titles are.
const titleContext = "short title of a small group outing, listing its activities in order"

// titles returns each plan's title in the caller's two display languages
// (secondary "" when the same or not chosen): composed in English from the
// activities' English names, then translated (and cached) per language.
func (s *Service) titles(ctx context.Context, q *dbgen.Queries, cityID string, caller Caller, items []matching.OpenPlan) (titles, secondaryTitles []string, err error) {
	rows, err := q.ListCityActivities(ctx, cityID)
	if err != nil {
		return nil, nil, err
	}
	english := make(map[string]string, len(rows))
	for _, r := range rows {
		english[r.ID] = i18n.Pick(r.Name, i18n.Authoring)
	}
	englishTitles := make([]string, len(items))
	texts := make([]translate.Text, len(items))
	for i, it := range items {
		englishTitles[i] = Title(english[string(it.Activity)], english[string(it.FollowUp)])
		raw, _ := json.Marshal(map[string]string{i18n.Authoring: englishTitles[i]})
		texts[i] = translate.Text{Raw: raw, Context: titleContext}
	}
	primary, secondary := make([]string, len(items)), make([]string, len(items))
	if s.Translations != nil {
		primary = s.Translations.Resolve(ctx, caller.Primary, texts)
		if caller.Secondary != "" {
			secondary = s.Translations.Resolve(ctx, caller.Secondary, texts)
		}
	}
	titles, secondaryTitles = make([]string, len(items)), make([]string, len(items))
	for i := range items {
		titles[i], secondaryTitles[i] = i18n.Pair(primary[i], secondary[i], englishTitles[i])
	}
	return titles, secondaryTitles, nil
}

// Title summarises a plan in English: "Climbing and bouldering, then lunch
// or dinner out". Area names are left out: they're proper nouns, which
// machine translation mangles, and the app shows them separately. Examples
// in brackets, which help on the onboarding tiles ("Food tour (ramen,
// desserts, markets …)"), are dropped.
func Title(activity, followUp string) string {
	activity = withoutExamples(activity)
	if followUp == "" {
		return activity
	}
	return activity + ", then " + lowerFirst(withoutExamples(followUp))
}

// withoutExamples drops a trailing "(…)" from a name.
func withoutExamples(name string) string {
	if i := strings.LastIndex(name, " ("); i > 0 && strings.HasSuffix(name, ")") {
		return name[:i]
	}
	return name
}

// lowerFirst lowercases the first letter for use mid-sentence, unless the
// first word is an acronym ("BBQ night").
func lowerFirst(s string) string {
	word, _, _ := strings.Cut(s, " ")
	if len([]rune(word)) > 1 && strings.ToUpper(word) == word {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}

// Feed returns every open plan in the caller's city, best for them first.
func (s *Service) Feed(ctx context.Context, userID uuid.UUID) ([]FeedItem, error) {
	items, _, _, err := s.feed(ctx, dbgen.New(s.DB), userID)
	return items, err
}

// Plan returns one open plan as the caller sees it, its steps, and their
// signup (nil if they haven't signed up).
func (s *Service) Plan(ctx context.Context, userID, planID uuid.UUID) (FeedItem, []Step, *matching.Signup, error) {
	q := dbgen.New(s.DB)
	item, _, caller, err := s.find(ctx, q, userID, planID)
	if errors.Is(err, ErrNotFound) {
		item, err = s.closedPlan(ctx, q, caller, userID, planID)
	}
	if err != nil {
		return item, nil, nil, err
	}
	steps := s.steps(ctx, item.Plan, item.Venue, item.FollowUpVenue, item.MeetingPlace, item.Route, caller)
	if !item.SignedUp {
		return item, steps, nil, nil
	}
	signup, err := loadSignup(ctx, q, planID, userID)
	return item, steps, signup, err
}

// steps lists the plan's stops with their venues fetched live in both
// display languages, all at once. A venue that can't be fetched is left
// out (logged): the plan still shows, just without the place's name.
func (s *Service) steps(ctx context.Context, p matching.Plan, venue, followUpVenue, meeting string, route *Route, caller Caller) []Step {
	var steps []Step
	var venues []string
	// A walk (no venue for the first activity) starts at its meeting place.
	if venue == "" && meeting != "" {
		steps, venues = append(steps, Step{Meeting: true}), append(venues, meeting)
	}
	steps, venues = append(steps, Step{Activity: p.Activity}), append(venues, venue)
	if p.FollowUp != "" {
		steps, venues = append(steps, Step{Activity: p.FollowUp}), append(venues, followUpVenue)
	}
	// The stored route ends at the follow-up's venue. Its time is only
	// shown when the stop before has a place: after a walk (meeting point →
	// photo walk → café) the walk itself is the time between, and the
	// station-to-café minutes would read as an extra walk.
	routeEnd := -1
	if last := len(steps) - 1; route != nil && p.FollowUp != "" && venues[last-1] != "" {
		routeEnd = last
	}
	if s.Places == nil {
		// No venue details, but a stored route still gives the walking time.
		if routeEnd > 0 {
			steps[routeEnd].WalkMinutes = route.Minutes()
		}
		return steps
	}
	log := s.Log
	if log == nil {
		log = slog.Default()
	}
	var wg sync.WaitGroup
	fetch := func(i int, lang string, set func(maps.PlaceDetails)) {
		if venues[i] == "" || lang == "" {
			return
		}
		wg.Go(func() {
			d, err := s.Places.Details(ctx, venues[i], lang)
			if err != nil {
				log.Warn("venue details", "place", venues[i], "lang", lang, "err", err)
				return
			}
			set(d)
		})
	}
	secondary := make([]maps.PlaceDetails, len(steps))
	for i := range steps {
		fetch(i, caller.Primary, func(d maps.PlaceDetails) { steps[i].Venue = d })
		fetch(i, caller.Secondary, func(d maps.PlaceDetails) { secondary[i] = d })
	}
	wg.Wait()
	for i := range steps {
		if steps[i].Venue.Name == "" {
			// Only the secondary came back: show it rather than nothing.
			steps[i].Venue, secondary[i] = secondary[i], maps.PlaceDetails{}
		}
		if n := secondary[i].Name; n != steps[i].Venue.Name {
			steps[i].SecondaryName = n
		}
		if i > 0 {
			// Google's walking time when the route is known; otherwise an
			// estimate from the straight line.
			if i == routeEnd {
				steps[i].WalkMinutes = route.Minutes()
			} else {
				steps[i].WalkMinutes = walkMinutes(steps[i-1].Venue.Location, steps[i].Venue.Location)
			}
		}
	}
	return steps
}

// walkMinutes estimates a walk between two venues from the straight-line
// distance: streets add about 30%, and people walk about 80 m a minute.
// No Routes call: it's a hint, and Maps content can't be cached anyway.
// 0 when either location is unknown.
func walkMinutes(a, b maps.LatLng) int {
	if (a == maps.LatLng{}) || (b == maps.LatLng{}) {
		return 0
	}
	return max(1, int(math.Ceil(maps.DistanceM(a, b)*1.3/80)))
}

func (s *Service) find(ctx context.Context, q *dbgen.Queries, userID, planID uuid.UUID) (FeedItem, *matching.User, Caller, error) {
	items, u, caller, err := s.feed(ctx, q, userID)
	if err != nil {
		return FeedItem{}, nil, caller, err
	}
	for _, it := range items {
		if it.ID == planID {
			return it, u, caller, nil
		}
	}
	return FeedItem{}, nil, caller, ErrNotFound
}

// ShowClosedFor is how long a closed plan's screen stays available, so a
// push or link about it explains what happened instead of "not found".
const ShowClosedFor = 14 * 24 * time.Hour

// closedPlan returns a plan in the caller's city that closed within
// ShowClosedFor, unranked (Level and Minutes don't matter once closed).
func (s *Service) closedPlan(ctx context.Context, q *dbgen.Queries, caller Caller, userID, planID uuid.UUID) (FeedItem, error) {
	r, err := q.GetRecentlyClosedPlan(ctx, dbgen.GetRecentlyClosedPlanParams{
		ID: planID, CityID: caller.City, UserID: userID,
		Since: pgtype.Timestamptz{Time: time.Now().Add(-ShowClosedFor), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return FeedItem{}, ErrNotFound
	}
	if err != nil {
		return FeedItem{}, err
	}
	it := FeedItem{
		Open: Open{
			ID: r.ID,
			OpenPlan: matching.OpenPlan{
				Plan: matching.Plan{
					Area:      matching.AreaID(r.AreaID),
					Activity:  matching.ActivityID(r.ActivityID),
					FollowUp:  matching.ActivityID(deref(r.FollowUpID)),
					Primary:   r.PrimaryLanguage,
					Secondary: deref(r.SecondaryLanguage),
				},
				Signups: int(r.Signups),
			},
			Round:         int(r.Round),
			Window:        Window{Start: r.WindowStart.Time, Days: int(r.WindowDays), Deadline: r.SignupDeadline.Time},
			Venue:         deref(r.VenuePlaceID),
			FollowUpVenue: deref(r.FollowUpVenuePlaceID),
			Route:         routeFrom(r.RoutePolyline, r.RouteSeconds),
			MeetingPlace:  deref(r.MeetingPlaceID),
		},
		Minutes:      -1,
		SignedUp:     r.SignedUp,
		ClosedReason: r.ClosedReason,
		ClosedAt:     r.ClosedAt.Time,
	}
	titles, secondary, err := s.titles(ctx, q, caller.City, caller, []matching.OpenPlan{it.OpenPlan})
	if err != nil {
		return FeedItem{}, err
	}
	it.Title, it.SecondaryTitle = titles[0], secondary[0]
	return it, nil
}

// SignUp signs the caller up for a plan, or replaces the times they gave.
// signup.User is set from userID.
func (s *Service) SignUp(ctx context.Context, userID, planID uuid.UUID, signup matching.Signup) (FeedItem, error) {
	q := dbgen.New(s.DB)
	item, _, _, err := s.find(ctx, q, userID, planID)
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
