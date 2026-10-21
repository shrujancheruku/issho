package profile

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

// ConductVersion is the code of conduct the app shows today. Bump it when the
// text changes in a way people must agree to again; GetProfile then reports
// the newer version and the app asks for acceptance.
const ConductVersion = 1

const (
	// MinAge is the youngest a person can be this year. Only the birth year is
	// asked, so someone who turns 18 later this year passes; identity
	// verification (M9) is the real check.
	MinAge = 18
	// maxAge rejects typos like 1099; not a product limit.
	maxAge = 100
	// MinActivities makes sure the feed has something to rank by.
	MinActivities = 3
	maxNameLen    = 40
)

// Profile is a whole profile as the app edits it: one row in app.profiles
// plus the person's languages, availability and activity preferences.
type Profile struct {
	DisplayName       string
	BirthYear         int
	Gender            string // app.profiles.gender: woman, man, nonbinary, unspecified
	SameGenderOnly    bool
	CityID            string // derived from HomeAreaID on save
	HomeAreaID        string
	MaxTravelMin      int
	Languages         []Language
	PrimaryLanguage   string
	SecondaryLanguage string // "" = none
	Availability      []Slot
	Prefs             map[string]int // activity id → 0 open to try, 1 like, 2 love
	SizePref          string         // small, medium, large
	Energy            int
	Budget            int
	ConductVersion    int
}

// Language is a language the person speaks (app.user_languages).
type Language struct {
	Code   string
	Fluent bool
	Level  int // learners only: 1..3
}

// Slot is a weekday (0 = Sunday) and part of the day.
type Slot struct {
	Weekday int
	Block   string // morning, afternoon, evening
}

// InvalidError is a problem with the caller's input, safe to show them.
type InvalidError struct {
	Field  string
	Reason string
}

func (e *InvalidError) Error() string { return e.Field + ": " + e.Reason }

func invalid(field, format string, args ...any) error {
	return &InvalidError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// IsInvalid reports whether err is an InvalidError.
func IsInvalid(err error) bool {
	var ie *InvalidError
	return errors.As(err, &ie)
}

var blocks = []string{"morning", "afternoon", "evening"}

// validate checks everything that doesn't need the database (the area and
// activity ids are checked in Save) and normalises the input: trimmed name,
// lowercase language codes. year is the current year.
func validate(p *Profile, year int) error {
	p.DisplayName = strings.TrimSpace(p.DisplayName)
	if p.DisplayName == "" {
		return invalid("display_name", "required")
	}
	if utf8.RuneCountInString(p.DisplayName) > maxNameLen {
		return invalid("display_name", "at most %d characters", maxNameLen)
	}
	if p.BirthYear > year-MinAge {
		return invalid("birth_year", "must be at least %d", MinAge)
	}
	if p.BirthYear < year-maxAge {
		return invalid("birth_year", "out of range")
	}
	switch p.Gender {
	case "woman", "man", "nonbinary":
	case "unspecified":
		// Same-gender-only groups need a gender to match on.
		if p.SameGenderOnly {
			return invalid("same_gender_only", "needs a gender")
		}
	default:
		return invalid("gender", "required")
	}
	if p.HomeAreaID == "" {
		return invalid("home_area_id", "required")
	}
	if p.MaxTravelMin < 5 || p.MaxTravelMin > 180 {
		return invalid("max_travel_min", "must be 5–180")
	}

	if len(p.Languages) == 0 {
		return invalid("spoken_languages", "required")
	}
	seen := map[string]bool{}
	fluent := false
	for i := range p.Languages {
		l := &p.Languages[i]
		l.Code = strings.ToLower(strings.TrimSpace(l.Code))
		if !i18n.Valid(l.Code) {
			return invalid("spoken_languages", "%q is not an ISO 639-1 code", l.Code)
		}
		if seen[l.Code] {
			return invalid("spoken_languages", "%q listed twice", l.Code)
		}
		seen[l.Code] = true
		if l.Fluent {
			fluent = true
			l.Level = 0
		} else if l.Level < 1 || l.Level > 3 {
			return invalid("spoken_languages", "level for %q must be 1–3", l.Code)
		}
	}
	// Matching pools people by a language they speak fluently.
	if !fluent {
		return invalid("spoken_languages", "at least one fluent language")
	}

	p.PrimaryLanguage = strings.ToLower(p.PrimaryLanguage)
	p.SecondaryLanguage = strings.ToLower(p.SecondaryLanguage)
	if !i18n.Valid(p.PrimaryLanguage) {
		return invalid("primary_language", "required (ISO 639-1)")
	}
	if p.SecondaryLanguage != "" {
		if !i18n.Valid(p.SecondaryLanguage) {
			return invalid("secondary_language", "not an ISO 639-1 code")
		}
		if p.SecondaryLanguage == p.PrimaryLanguage {
			return invalid("secondary_language", "same as primary_language")
		}
	}

	if len(p.Availability) == 0 {
		return invalid("availability", "at least one slot")
	}
	slots := map[Slot]bool{}
	for _, s := range p.Availability {
		if s.Weekday < 0 || s.Weekday > 6 || !slices.Contains(blocks, s.Block) {
			return invalid("availability", "unknown slot")
		}
		if slots[s] {
			return invalid("availability", "slot listed twice")
		}
		slots[s] = true
	}

	if len(p.Prefs) < MinActivities {
		return invalid("activities", "pick at least %d", MinActivities)
	}
	for id, w := range p.Prefs {
		if w < 0 || w > 2 {
			return invalid("activities", "unknown interest for %q", id)
		}
	}

	switch p.SizePref {
	case "small", "medium", "large":
	default:
		return invalid("group_size", "required")
	}
	if p.Energy < 1 || p.Energy > 3 {
		return invalid("energy", "must be 1–3")
	}
	if p.Budget < 0 || p.Budget > 3 {
		return invalid("budget", "must be 0–3")
	}
	if p.ConductVersion != ConductVersion {
		return invalid("conduct_version", "accept version %d", ConductVersion)
	}
	return nil
}
