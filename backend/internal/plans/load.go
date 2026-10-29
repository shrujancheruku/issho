package plans

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

// City is everything matching needs about one city.
type City struct {
	ID         string
	Location   *time.Location
	Config     matching.Config
	Areas      []matching.AreaID
	Travel     matching.TravelTimes
	Activities []matching.Activity // enabled and in season
	Venues     matching.Venues
	// WalkThemes are the walking activities' themes (activities.walk_theme),
	// for composing their plans.
	WalkThemes map[matching.ActivityID]string
	Users      []matching.User
	// Relations: mutual meet-again picks and pairs blocked by a report.
	Relations matching.Relations
	// Review: admins review the city's plans before they open (00021).
	Review bool

	// Lookups into Activities and Users, built when they're loaded.
	activities map[matching.ActivityID]*matching.Activity
	users      map[matching.UserID]*matching.User
}

// Activity returns one of the city's activities (enabled, and in season
// if loaded for a month), or nil.
func (c *City) Activity(id matching.ActivityID) *matching.Activity { return c.activities[id] }

// ActivitiesByID maps the city's activities by id, for matching's inputs.
// It's shared (a City may be cached): don't modify it.
func (c *City) ActivitiesByID() map[matching.ActivityID]*matching.Activity { return c.activities }

// UsersByID maps the people matched in the city by id (LoadCity only). It's
// shared: don't modify it.
func (c *City) UsersByID() map[matching.UserID]*matching.User { return c.users }

// LoadCity reads a city for matching, with the activities enabled there and
// in season in the given month (0 = any month).
func LoadCity(ctx context.Context, q *dbgen.Queries, cityID string, month time.Month) (*City, error) {
	c, err := loadPlaces(ctx, q, cityID, month)
	if err != nil {
		return nil, err
	}
	if err := loadUsers(ctx, q, c); err != nil {
		return nil, err
	}
	if err := loadRelations(ctx, q, c); err != nil {
		return nil, err
	}
	return c, nil
}

// loadPlaces reads everything about a city except its people: settings,
// areas, travel times, activities and venues.
func loadPlaces(ctx context.Context, q *dbgen.Queries, cityID string, month time.Month) (*City, error) {
	row, err := q.GetCityMatching(ctx, cityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("city %q not found", cityID)
		}
		return nil, err
	}
	c := &City{ID: cityID, Travel: matching.TravelTimes{}, Venues: matching.Venues{}, WalkThemes: map[matching.ActivityID]string{}, Review: row.Review}
	if c.Location, err = time.LoadLocation(row.Timezone); err != nil {
		return nil, fmt.Errorf("city %s: %w", cityID, err)
	}
	if c.Config, err = matching.ParseConfig(row.MatchingWeights); err != nil {
		return nil, fmt.Errorf("city %s: %w", cityID, err)
	}

	areas, err := q.ListCityAreaIDs(ctx, cityID)
	if err != nil {
		return nil, err
	}
	for _, a := range areas {
		c.Areas = append(c.Areas, matching.AreaID(a))
	}
	travel, err := q.ListCityTravelTimes(ctx, cityID)
	if err != nil {
		return nil, err
	}
	for _, t := range travel {
		c.Travel[matching.AreaPair{From: matching.AreaID(t.FromAreaID), To: matching.AreaID(t.ToAreaID)}] = int(t.Minutes)
	}

	acts, err := q.ListMatchingActivities(ctx, dbgen.ListMatchingActivitiesParams{CityID: cityID, Month: int16(month)})
	if err != nil {
		return nil, err
	}
	for _, a := range acts {
		m := matching.Activity{
			ID: matching.ActivityID(a.ID), Category: a.Category, CostLevel: int(a.CostLevel),
			Conversational: a.Conversational, MinSize: int(a.MinGroupSize), MaxSize: int(a.MaxGroupSize),
			Walk: a.WalkTheme != nil, Energy: int(a.Energy), Hours: a.Hours,
		}
		if a.WalkTheme != nil {
			c.WalkThemes[m.ID] = *a.WalkTheme
		}
		for _, v := range a.VenueCategories {
			m.VenueCategories = append(m.VenueCategories, matching.CategoryID(v))
		}
		for _, b := range a.Blocks {
			m.Blocks = append(m.Blocks, matching.Block(b))
		}
		c.Activities = append(c.Activities, m)
	}
	c.activities = make(map[matching.ActivityID]*matching.Activity, len(c.Activities))
	for i := range c.Activities {
		c.activities[c.Activities[i].ID] = &c.Activities[i]
	}
	venues, err := q.ListCityVenueCounts(ctx, cityID)
	if err != nil {
		return nil, err
	}
	for _, v := range venues {
		area := matching.AreaID(v.AreaID)
		if c.Venues[area] == nil {
			c.Venues[area] = map[matching.CategoryID]int{}
		}
		c.Venues[area][matching.CategoryID(v.CategoryID)] = int(v.Venues)
	}
	return c, nil
}

// Caller is what the plan API needs about the person asking, besides
// their matching profile.
type Caller struct {
	City string
	// Display languages (i18n.ForApp): what plan titles are shown in.
	Primary, Secondary string
}

// loadCaller reads who's asking (their city and display languages) without
// their matching profile. ErrNoProfile means they have no profile.
func loadCaller(ctx context.Context, q *dbgen.Queries, userID uuid.UUID) (Caller, error) {
	r, err := q.GetDisplayLanguages(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Caller{}, ErrNoProfile
	}
	if err != nil {
		return Caller{}, err
	}
	primary, secondary := i18n.ForApp(r.PrimaryLanguage, deref(r.SecondaryLanguage))
	return Caller{City: r.CityID, Primary: primary, Secondary: secondary}, nil
}

// LoadUser reads one person for their feed, with their city and display
// languages. It leaves out Reliability and EventsAttended, which only group
// assembly (LoadCity) uses. pgx.ErrNoRows means they have no profile.
func LoadUser(ctx context.Context, q *dbgen.Queries, userID uuid.UUID) (*matching.User, Caller, error) {
	r, err := q.GetMatchingUser(ctx, userID)
	if err != nil {
		return nil, Caller{}, err
	}
	primary, secondary := i18n.ForApp(r.PrimaryLanguage, deref(r.SecondaryLanguage))
	caller := Caller{City: r.CityID, Primary: primary, Secondary: secondary}
	u := newUser(userID, r.HomeAreaID, r.MaxTravelMin, r.Gender, r.SameGenderOnly, r.SizePref, r.Budget)
	for i, code := range r.LangCodes {
		u.Languages = append(u.Languages, matching.Language{Code: code, Fluent: r.LangFluent[i], Level: int(r.LangLevels[i])})
	}
	for i, w := range r.SlotWeekdays {
		u.Availability = append(u.Availability, matching.Slot{Weekday: time.Weekday(w), Block: matching.Block(r.SlotBlocks[i])})
	}
	for i, a := range r.PrefActivities {
		u.Prefs[matching.ActivityID(a)] = int(r.PrefWeights[i])
	}
	return &u, caller, nil
}

// newUser is a profile as matching sees it, before its languages,
// availability and preferences are added.
func newUser(id uuid.UUID, homeArea string, maxTravelMin int16, gender string, sameGenderOnly bool, sizePref string, budget int16) matching.User {
	return matching.User{
		ID:             matching.UserID(id.String()),
		HomeArea:       matching.AreaID(homeArea),
		MaxTravelMin:   int(maxTravelMin),
		Gender:         matching.Gender(gender),
		SameGenderOnly: sameGenderOnly,
		Size:           matching.SizePref(sizePref),
		Budget:         int(budget),
		Prefs:          map[matching.ActivityID]int{},
	}
}

// reliability is how often someone turned up when they'd confirmed: of
// their confirmed places at finished events, how many they attended, with a
// prior of two outings attended, so a newcomer starts at 1 and one miss
// doesn't sink them (one no-show in their first outing gives 2/3).
func reliability(attended, confirmed int) float64 {
	const prior = 2
	return float64(prior+attended) / float64(prior+confirmed)
}

// loadUsers reads every profile in the city with its languages,
// availability and activity preferences. Each list query is sorted by user,
// like the profiles, but rows are matched by id rather than position.
func loadUsers(ctx context.Context, q *dbgen.Queries, c *City) error {
	rows, err := q.ListMatchingUsers(ctx, c.ID)
	if err != nil {
		return err
	}
	c.Users = make([]matching.User, len(rows))
	for i, r := range rows {
		c.Users[i] = newUser(r.UserID, r.HomeAreaID, r.MaxTravelMin, r.Gender, r.SameGenderOnly, r.SizePref, r.Budget)
		c.Users[i].Reliability = reliability(int(r.DoneAttended), int(r.DoneConfirmed))
		c.Users[i].EventsAttended = int(r.EventsAttended)
	}
	c.users = make(map[matching.UserID]*matching.User, len(c.Users))
	for i := range c.Users {
		c.users[c.Users[i].ID] = &c.Users[i]
	}
	// A profile saved between two of these queries may be missing from the
	// first; its rows in the later ones are skipped until the next pass.
	user := func(id fmt.Stringer) *matching.User { return c.users[matching.UserID(id.String())] }

	langs, err := q.ListMatchingLanguages(ctx, c.ID)
	if err != nil {
		return err
	}
	for _, l := range langs {
		if u := user(l.UserID); u != nil {
			u.Languages = append(u.Languages, matching.Language{Code: l.Lang, Fluent: l.Fluent, Level: int(l.Level)})
		}
	}
	slots, err := q.ListMatchingAvailability(ctx, c.ID)
	if err != nil {
		return err
	}
	for _, s := range slots {
		if u := user(s.UserID); u != nil {
			u.Availability = append(u.Availability, matching.Slot{Weekday: time.Weekday(s.Weekday), Block: matching.Block(s.Block)})
		}
	}
	prefs, err := q.ListMatchingPrefs(ctx, c.ID)
	if err != nil {
		return err
	}
	for _, p := range prefs {
		if u := user(p.UserID); u != nil {
			u.Prefs[matching.ActivityID(p.ActivityID)] = int(p.Weight)
		}
	}
	return nil
}
