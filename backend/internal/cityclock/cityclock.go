// Package cityclock is the one helper that turns an instant into the city's
// wall-clock terms, and a city's wall-clock terms into an instant, from the
// city's time zone ("One city clock" in docs/spec/architecture.md). Every
// "09:00 on the day", quiet-hours check and part of the day goes through it,
// so daylight saving and the date line are handled in one place.
//
// It is pure: it never reads the system clock. Callers pass the instant, which
// they get from an injected clock.Clock.
package cityclock

import (
	"fmt"
	"time"
)

// Part is a part of the day, in the city's clock hours.
type Part string

const (
	// None is an instant in none of the city's three parts (before the morning
	// starts or after the evening ends).
	None      Part = "none"
	Morning   Part = "morning"
	Afternoon Part = "afternoon"
	Evening   Part = "evening"
)

// Span is a half-open range of clock times on one day, "HH:MM" to "HH:MM": it
// includes From and excludes To, and does not cross midnight.
type Span struct{ From, To string }

// DayParts are a city's clock hours of morning, afternoon and evening, a city
// setting (docs/spec/city-settings.md).
type DayParts struct{ Morning, Afternoon, Evening Span }

// City is one city's clock. Build it with New.
type City struct {
	loc   *time.Location
	parts [3]struct {
		part     Part
		from, to int // minutes after midnight
	}
}

// New builds the clock for an IANA time zone name such as "Asia/Ho_Chi_Minh".
func New(zone string, parts DayParts) (*City, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, fmt.Errorf("cityclock: time zone %q: %w", zone, err)
	}
	c := &City{loc: loc}
	for i, p := range []struct {
		part Part
		span Span
	}{{Morning, parts.Morning}, {Afternoon, parts.Afternoon}, {Evening, parts.Evening}} {
		from, err := minutes(p.span.From)
		if err != nil {
			return nil, fmt.Errorf("cityclock: %s: %w", p.part, err)
		}
		to, err := minutes(p.span.To)
		if err != nil {
			return nil, fmt.Errorf("cityclock: %s: %w", p.part, err)
		}
		c.parts[i].part, c.parts[i].from, c.parts[i].to = p.part, from, to
	}
	return c, nil
}

// Date is the city's calendar date ("2006-01-02") at the instant.
func (c *City) Date(t time.Time) string {
	return t.In(c.loc).Format(time.DateOnly)
}

// At is the instant of "HH:MM" on a city date ("2006-01-02"). A time the clocks
// skip (spring forward) lands after the gap, as the clocks then show it; a time
// that happens twice (fall back) is the first one.
func (c *City) At(date, hhmm string) (time.Time, error) {
	d, err := time.ParseInLocation(time.DateOnly, date, c.loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("cityclock: date %q: %w", date, err)
	}
	m, err := minutes(hhmm)
	if err != nil {
		return time.Time{}, err
	}
	t := time.Date(d.Year(), d.Month(), d.Day(), m/60, m%60, 0, 0, c.loc)
	// Go leaves the pick in a gap or an overlap unspecified. Pin both. In a
	// gap the result shows an earlier wall time than asked for: move on by the
	// difference, which lands after the gap.
	if shown := t.Hour()*60 + t.Minute(); shown != m {
		t = t.Add(time.Duration(m-shown) * time.Minute)
	}
	// If an hour earlier shows the same wall time, we landed on the second
	// occurrence of a repeated time, so take the first.
	if prev := t.Add(-time.Hour); sameWall(prev, t) {
		return prev, nil
	}
	return t, nil
}

// PartOfDay is the part of the day the instant falls in on the city's clock.
func (c *City) PartOfDay(t time.Time) Part {
	m := c.minuteOfDay(t)
	for _, p := range c.parts {
		if m >= p.from && m < p.to {
			return p.part
		}
	}
	return None
}

// InWindow reports whether the instant's city clock time is in [from, to).
// When to is before from the window crosses midnight (22:00 to 07:00 holds
// 23:30 and 03:00). Equal ends are an empty window.
func (c *City) InWindow(t time.Time, from, to string) (bool, error) {
	f, err := minutes(from)
	if err != nil {
		return false, err
	}
	e, err := minutes(to)
	if err != nil {
		return false, err
	}
	m := c.minuteOfDay(t)
	if f <= e {
		return m >= f && m < e, nil
	}
	return m >= f || m < e, nil
}

func (c *City) minuteOfDay(t time.Time) int {
	l := t.In(c.loc)
	return l.Hour()*60 + l.Minute()
}

func sameWall(a, b time.Time) bool {
	return a.Hour() == b.Hour() && a.Minute() == b.Minute() && a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// minutes parses a strict "HH:MM" into minutes after midnight.
func minutes(hhmm string) (int, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil || len(hhmm) != 5 {
		return 0, fmt.Errorf("cityclock: clock time %q is not HH:MM", hhmm)
	}
	return t.Hour()*60 + t.Minute(), nil
}
