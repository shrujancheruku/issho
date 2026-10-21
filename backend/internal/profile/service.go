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
	langs, err := q.ListUserLanguages(ctx, userID)
	if err != nil {
		return Profile{}, err
	}
	for _, l := range langs {
		lang := Language{Code: l.Lang, Fluent: l.Fluent}
		if l.Level != nil {
			lang.Level = int(*l.Level)
		}
		p.Languages = append(p.Languages, lang)
	}
	slots, err := q.ListAvailability(ctx, userID)
	if err != nil {
		return Profile{}, err
	}
	for _, sl := range slots {
		p.Availability = append(p.Availability, Slot{Weekday: int(sl.Weekday), Block: sl.Block})
	}
	prefs, err := q.ListActivityPrefs(ctx, userID)
	if err != nil {
		return Profile{}, err
	}
	for _, pr := range prefs {
		p.Prefs[pr.ActivityID] = int(pr.Weight)
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
		city, err := q.GetAreaCity(ctx, p.HomeAreaID)
		if errors.Is(err, pgx.ErrNoRows) {
			return invalid("home_area_id", "unknown area")
		}
		if err != nil {
			return err
		}
		ids := slices.Sorted(maps.Keys(p.Prefs))
		enabled, err := q.ListEnabledActivityIDs(ctx, dbgen.ListEnabledActivityIDsParams{CityID: city, Ids: ids})
		if err != nil {
			return err
		}
		for _, id := range ids {
			if !slices.Contains(enabled, id) {
				return invalid("activities", "%q isn't offered in %s", id, city)
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
		langs := dbgen.InsertUserLanguagesParams{UserID: userID}
		for _, l := range p.Languages {
			langs.Langs = append(langs.Langs, l.Code)
			langs.Fluent = append(langs.Fluent, l.Fluent)
			langs.Levels = append(langs.Levels, int16(l.Level))
		}
		slots := dbgen.InsertAvailabilityParams{UserID: userID}
		for _, sl := range p.Availability {
			slots.Weekdays = append(slots.Weekdays, int16(sl.Weekday))
			slots.Blocks = append(slots.Blocks, sl.Block)
		}
		prefs := dbgen.InsertActivityPrefsParams{UserID: userID}
		for _, id := range ids {
			prefs.ActivityIds = append(prefs.ActivityIds, id)
			prefs.Weights = append(prefs.Weights, int16(p.Prefs[id]))
		}
		for _, step := range []func() error{
			func() error { return q.DeleteUserLanguages(ctx, userID) },
			func() error { return q.InsertUserLanguages(ctx, langs) },
			func() error { return q.DeleteAvailability(ctx, userID) },
			func() error { return q.InsertAvailability(ctx, slots) },
			func() error { return q.DeleteActivityPrefs(ctx, userID) },
			func() error { return q.InsertActivityPrefs(ctx, prefs) },
		} {
			if err := step(); err != nil {
				return err
			}
		}
		saved, err = get(ctx, q, userID)
		return err
	})
	return saved, err
}
