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
}

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
	c := &City{ID: cityID, Travel: matching.TravelTimes{}, Venues: matching.Venues{}, WalkThemes: map[matching.ActivityID]string{}}
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
			Walk: a.WalkTheme != nil,
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

// LoadUser reads one person for matching, with their city and display
// languages. pgx.ErrNoRows means they have no profile.
func LoadUser(ctx context.Context, q *dbgen.Queries, userID uuid.UUID) (*matching.User, Caller, error) {
	r, err := q.GetMatchingUser(ctx, userID)
	if err != nil {
		return nil, Caller{}, err
	}
	primary, secondary := i18n.ForApp(r.PrimaryLanguage, deref(r.SecondaryLanguage))
	caller := Caller{City: r.CityID, Primary: primary, Secondary: secondary}
	u := &matching.User{
		ID:             matching.UserID(userID.String()),
		HomeArea:       matching.AreaID(r.HomeAreaID),
		MaxTravelMin:   int(r.MaxTravelMin),
		Gender:         matching.Gender(r.Gender),
		SameGenderOnly: r.SameGenderOnly,
		Size:           matching.SizePref(r.SizePref),
		Budget:         int(r.Budget),
		Prefs:          map[matching.ActivityID]int{},
		Reliability:    float64(r.Reliability),
	}
	langs, err := q.ListUserLanguages(ctx, userID)
	if err != nil {
		return nil, Caller{}, err
	}
	for _, l := range langs {
		level := 0
		if l.Level != nil {
			level = int(*l.Level)
		}
		u.Languages = append(u.Languages, matching.Language{Code: l.Lang, Fluent: l.Fluent, Level: level})
	}
	slots, err := q.ListAvailability(ctx, userID)
	if err != nil {
		return nil, Caller{}, err
	}
	for _, s := range slots {
		u.Availability = append(u.Availability, matching.Slot{Weekday: time.Weekday(s.Weekday), Block: matching.Block(s.Block)})
	}
	prefs, err := q.ListActivityPrefs(ctx, userID)
	if err != nil {
		return nil, Caller{}, err
	}
	for _, p := range prefs {
		u.Prefs[matching.ActivityID(p.ActivityID)] = int(p.Weight)
	}
	return u, caller, nil
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
	index := make(map[matching.UserID]int, len(rows))
	for i, r := range rows {
		id := matching.UserID(r.UserID.String())
		index[id] = i
		c.Users[i] = matching.User{
			ID:             id,
			HomeArea:       matching.AreaID(r.HomeAreaID),
			MaxTravelMin:   int(r.MaxTravelMin),
			Gender:         matching.Gender(r.Gender),
			SameGenderOnly: r.SameGenderOnly,
			Size:           matching.SizePref(r.SizePref),
			Budget:         int(r.Budget),
			Prefs:          map[matching.ActivityID]int{},
			Reliability:    float64(r.Reliability),
			EventsAttended: int(r.EventsAttended),
		}
	}
	// A profile saved between two of these queries may be missing from the
	// first; its rows in the later ones are skipped until the next pass.
	user := func(id fmt.Stringer) *matching.User {
		if i, ok := index[matching.UserID(id.String())]; ok {
			return &c.Users[i]
		}
		return nil
	}

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
