// Package profile stores what matching needs to know about a person: home
// area and travel time, languages, availability, activity preferences and
// group preferences. Each person reads and writes only their own profile.
package profile

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

var (
	// ErrNotFound means the person hasn't finished onboarding.
	ErrNotFound = errors.New("no profile yet")
	// ErrNoAccount means the app user row doesn't exist (GetMe wasn't called,
	// or the account was deleted).
	ErrNoAccount = errors.New("no account: call IdentityService.GetMe first")
)

type Service struct {
	DB  *pgxpool.Pool
	Now func() time.Time // nil = time.Now
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Get returns the person's profile, or ErrNotFound.
func (s *Service) Get(ctx context.Context, userID uuid.UUID) (Profile, error) {
	return get(ctx, dbgen.New(s.DB), userID)
}

func get(ctx context.Context, q *dbgen.Queries, userID uuid.UUID) (Profile, error) {
	row, err := q.GetProfile(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	p := Profile{
		DisplayName:     row.DisplayName,
		BirthYear:       int(row.BirthYear),
		Gender:          row.Gender,
		SameGenderOnly:  row.SameGenderOnly,
		CityID:          row.CityID,
		HomeAreaID:      row.HomeAreaID,
		MaxTravelMin:    int(row.MaxTravelMin),
		PrimaryLanguage: row.PrimaryLanguage,
		SizePref:        row.SizePref,
		Energy:          int(row.Energy),
		Budget:          int(row.Budget),
		ConductVersion:  int(row.ConductVersion),
		Prefs:           map[string]int{},
	}
	if row.SecondaryLanguage != nil {
		p.SecondaryLanguage = *row.SecondaryLanguage
	}
	for i, code := range row.LangCodes {
		p.Languages = append(p.Languages, Language{Code: code, Fluent: row.LangFluent[i], Level: int(row.LangLevels[i])})
	}
	for i, w := range row.SlotWeekdays {
		p.Availability = append(p.Availability, Slot{Weekday: int(w), Block: row.SlotBlocks[i]})
	}
	for i, a := range row.PrefActivities {
		p.Prefs[a] = int(row.PrefWeights[i])
	}
	return p, nil
}

// Save validates p and replaces the person's whole profile with it in one
// transaction, then returns what was stored. Input problems are
// InvalidErrors.
func (s *Service) Save(ctx context.Context, userID uuid.UUID, p Profile) (Profile, error) {
	if err := validate(&p, s.now().Year()); err != nil {
		return Profile{}, err
	}
	var saved Profile
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		ids := slices.Sorted(maps.Keys(p.Prefs))
		area, err := q.GetAreaEnabledActivities(ctx, dbgen.GetAreaEnabledActivitiesParams{AreaID: p.HomeAreaID, Ids: ids})
		if errors.Is(err, pgx.ErrNoRows) {
			return invalid("home_area_id", "unknown area")
		}
		if err != nil {
			return err
		}
		for _, id := range ids {
			if !slices.Contains(area.Enabled, id) {
				return invalid("activities", "%q isn't offered in %s", id, area.CityID)
			}
		}

		var secondary *string
		if p.SecondaryLanguage != "" {
			secondary = &p.SecondaryLanguage
		}
		err = q.UpsertProfile(ctx, dbgen.UpsertProfileParams{
			UserID:            userID,
			DisplayName:       p.DisplayName,
			BirthYear:         int16(p.BirthYear),
			Gender:            p.Gender,
			SameGenderOnly:    p.SameGenderOnly,
			HomeAreaID:        p.HomeAreaID,
			MaxTravelMin:      int16(p.MaxTravelMin),
			Energy:            int16(p.Energy),
			Budget:            int16(p.Budget),
			SizePref:          p.SizePref,
			PrimaryLanguage:   p.PrimaryLanguage,
			SecondaryLanguage: secondary,
			ConductVersion:    int16(p.ConductVersion),
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation && pgErr.ConstraintName == "profiles_user_id_fkey" {
				return ErrNoAccount
			}
			return err
		}

		// Replace the child rows wholesale: the app always sends the full lists.
		lists := dbgen.ReplaceProfileListsParams{UserID: userID}
		for _, l := range p.Languages {
			lists.Langs = append(lists.Langs, l.Code)
			lists.Fluent = append(lists.Fluent, l.Fluent)
			lists.Levels = append(lists.Levels, int16(l.Level))
		}
		for _, sl := range p.Availability {
			lists.Weekdays = append(lists.Weekdays, int16(sl.Weekday))
			lists.Blocks = append(lists.Blocks, sl.Block)
		}
		for _, id := range ids {
			lists.ActivityIds = append(lists.ActivityIds, id)
			lists.Weights = append(lists.Weights, int16(p.Prefs[id]))
		}
		if err := q.ReplaceProfileLists(ctx, lists); err != nil {
			return err
		}
		saved, err = get(ctx, q, userID)
		return err
	})
	return saved, err
}
