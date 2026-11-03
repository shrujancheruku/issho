package plans

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/maps/mapstest"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// byVenue answers each venue with its own hours (none = unknown).
func byVenue(hours map[string]maps.Hours) *mapstest.Fake {
	return &mapstest.Fake{HoursFunc: func(id string) maps.Hours { return hours[id] }}
}

// TestCheckVenuesWindow: venues must be open for the activity's own hours
// from the start, and the follow-up's from where the activity really ends.
func TestCheckVenuesWindow(t *testing.T) {
	jst := time.FixedZone("JST", 9*3600)
	sat := func(h, m int) time.Time { return time.Date(2030, 1, 12, h, m, 0, 0, jst) } // a Saturday
	allDay := maps.Hours{Known: true, Periods: []maps.Period{{Open: maps.WeekTime{Day: time.Sunday}, NoClose: true}}}
	// Open Saturday 20:00 to Sunday 02:00 only.
	bar := maps.Hours{Known: true, Periods: []maps.Period{{Open: maps.WeekTime{Day: time.Saturday, Hour: 20}, Close: maps.WeekTime{Day: time.Sunday, Hour: 2}}}}
	// Open 9–12 and 13–22 every day: a lunch closure.
	lunch := maps.Hours{Known: true}
	for d := time.Sunday; d <= time.Saturday; d++ {
		lunch.Periods = append(lunch.Periods,
			maps.Period{Open: maps.WeekTime{Day: d, Hour: 9}, Close: maps.WeekTime{Day: d, Hour: 12}},
			maps.Period{Open: maps.WeekTime{Day: d, Hour: 13}, Close: maps.WeekTime{Day: d, Hour: 22}})
	}
	act := func(hours float64) *matching.Activity { return &matching.Activity{ID: "a", Hours: hours} }
	next := func(hours float64) *matching.Activity { return &matching.Activity{ID: "b", Hours: hours} }
	none := maps.Hours{}
	// Closed 12:10–12:25 only, between the samples at 12:00 and 12:30. This
	// documents the 30-minute sampling (a short break goes unseen), not a
	// requirement.
	shortBreak := maps.Hours{Known: true}
	for d := time.Sunday; d <= time.Saturday; d++ {
		shortBreak.Periods = append(shortBreak.Periods,
			maps.Period{Open: maps.WeekTime{Day: d, Hour: 9}, Close: maps.WeekTime{Day: d, Hour: 12, Minute: 10}},
			maps.Period{Open: maps.WeekTime{Day: d, Hour: 12, Minute: 25}, Close: maps.WeekTime{Day: d, Hour: 22}})
	}

	for _, c := range []struct {
		name         string
		activity     *matching.Activity // nil = default length
		followUp     *matching.Activity // nil = no follow-up
		start        time.Time
		venue, after maps.Hours
		want         venueProblem
	}{
		{"3h activity, venue closes at 16:00 partway through", act(3), nil, sat(14, 0), everyDay(9, 16), none, venueClosedAt},
		{"1h activity, venue closes at 16:00 after it ends", act(1), nil, sat(14, 0), everyDay(9, 16), none, venuesOK},
		{"activity ends exactly at closing time", act(2), nil, sat(14, 0), everyDay(9, 16), none, venuesOK},
		{"no stored hours: the default two hours", nil, nil, sat(14, 0), everyDay(9, 15), none, venueClosedAt},
		{"1h activity, follow-up opens when it ends", act(1), next(1), sat(14, 0), allDay, everyDay(15, 17), venuesOK},
		{"1h activity, follow-up not yet open at the real end", act(1), next(1), sat(14, 0), allDay, everyDay(16, 22), venueClosedAt},
		{"follow-up closes before its own hours are up", act(1), next(3), sat(14, 0), allDay, everyDay(15, 17), venueClosedAt},
		{"overnight: bar open past midnight", act(3), nil, sat(22, 0), bar, none, venuesOK},
		{"overnight: bar closes at 02:00 before a 5h activity ends", act(5), nil, sat(22, 0), bar, none, venueClosedAt},
		{"closes for lunch in the middle of a long stay", act(3), nil, sat(11, 0), lunch, none, venueClosedAt},
		{"a 15-minute break between samples is not seen (documents the sampling)", act(3), nil, sat(11, 0), shortBreak, none, venuesOK},
		{"venue with no hours data is assumed open", act(6), nil, sat(14, 0), none, none, venuesOK},
		{"an 8h activity needs a venue open the whole slot", act(8), nil, sat(16, 0), everyDay(10, 22), none, venueClosedAt},
		{"an 8h activity at a venue open all day", act(8), nil, sat(10, 0), allDay, none, venuesOK},
	} {
		acts := map[matching.ActivityID]*matching.Activity{}
		p := Open{Venue: "v"}
		if c.activity != nil {
			acts["a"] = c.activity
			p.Activity = "a"
		}
		if c.followUp != nil {
			acts["b"] = c.followUp
			p.FollowUp, p.FollowUpVenue = "b", "w"
		}
		r := &Runner{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Hours: byVenue(map[string]maps.Hours{"v": c.venue, "w": c.after})}
		if got := r.checkVenues(context.Background(), p, acts, c.start, map[string]*maps.Hours{}); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
