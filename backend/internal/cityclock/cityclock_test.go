package cityclock_test

import (
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/cityclock"
)

// Zones under test: Ho Chi Minh City is UTC+7 all year; Los Angeles is behind
// UTC and changes for daylight saving (2026: clocks forward 8 Mar, back 1 Nov);
// Sydney is ahead of UTC and changes in the other season.
var parts = cityclock.DayParts{
	Morning:   cityclock.Span{From: "06:00", To: "12:00"},
	Afternoon: cityclock.Span{From: "12:00", To: "17:00"},
	Evening:   cityclock.Span{From: "17:00", To: "22:00"},
}

func load(t *testing.T, name string) *cityclock.City {
	t.Helper()
	c, err := cityclock.New(name, parts)
	if err != nil {
		t.Fatalf("New(%q): %v", name, err)
	}
	return c
}

func utc(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func TestNewRejectsBadDayParts(t *testing.T) {
	bad := parts
	bad.Morning = cityclock.Span{From: "nine", To: "12:00"}
	if _, err := cityclock.New("Asia/Ho_Chi_Minh", bad); err == nil {
		t.Fatal("want an error for an unreadable span")
	}
}

func TestNewRejectsAnUnknownZone(t *testing.T) {
	if _, err := cityclock.New("Mars/Olympus", parts); err == nil {
		t.Fatal("want an error for an unknown zone")
	}
}

func TestDateIsTheCitysDateNotUTCs(t *testing.T) {
	tests := []struct {
		name    string
		zone    string
		instant time.Time
		want    string
	}{
		// 18:00 UTC on the 20th is 01:00 on the 21st in Ho Chi Minh City.
		{"ahead of UTC", "Asia/Ho_Chi_Minh", utc(2026, 11, 20, 18, 0), "2026-11-21"},
		// 03:00 UTC on the 21st is 19:00 on the 20th in Los Angeles (PST).
		{"behind UTC", "America/Los_Angeles", utc(2026, 11, 21, 3, 0), "2026-11-20"},
		// The instant just before and just after the fall-back: still the 1st.
		{"on the fall-back day", "America/Los_Angeles", utc(2026, 11, 1, 9, 30), "2026-11-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := load(t, tt.zone).Date(tt.instant); got != tt.want {
				t.Fatalf("Date = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestAtIsAnInstantOnTheCitysClock(t *testing.T) {
	tests := []struct {
		name string
		zone string
		date string
		hhmm string
		want time.Time
	}{
		{"ahead of UTC", "Asia/Ho_Chi_Minh", "2026-11-21", "09:00", utc(2026, 11, 21, 2, 0)},
		{"behind UTC, standard time", "America/Los_Angeles", "2026-11-21", "09:00", utc(2026, 11, 21, 17, 0)},
		{"behind UTC, summer time", "America/Los_Angeles", "2026-07-01", "09:00", utc(2026, 7, 1, 16, 0)},
		// Sydney is on summer time (UTC+11) in November.
		{"ahead of UTC with daylight saving", "Australia/Sydney", "2026-11-21", "09:00", utc(2026, 11, 20, 22, 0)},
		// 8 Mar 2026: 02:00 to 03:00 does not exist in Los Angeles. 02:30
		// lands on the far side of the gap, as 03:30 PDT.
		{"skipped hour", "America/Los_Angeles", "2026-03-08", "02:30", utc(2026, 3, 8, 10, 30)},
		// 1 Nov 2026: 01:00 to 02:00 happens twice. The first one wins.
		{"repeated hour", "America/Los_Angeles", "2026-11-01", "01:30", utc(2026, 11, 1, 8, 30)},
		{"the day's day-length changes", "America/Los_Angeles", "2026-11-01", "09:00", utc(2026, 11, 1, 17, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := load(t, tt.zone).At(tt.date, tt.hhmm)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("At(%s, %s) = %v, want %v", tt.date, tt.hhmm, got.UTC(), tt.want)
			}
		})
	}
}

func TestAtRejectsBadInput(t *testing.T) {
	c := load(t, "Asia/Ho_Chi_Minh")
	for _, in := range [][2]string{{"2026-13-01", "09:00"}, {"21/11/2026", "09:00"}, {"2026-11-21", "9am"}, {"2026-11-21", "24:00"}, {"2026-11-21", "09:60"}} {
		if _, err := c.At(in[0], in[1]); err == nil {
			t.Errorf("At(%q, %q): want an error", in[0], in[1])
		}
	}
}

func TestPartOfDay(t *testing.T) {
	tests := []struct {
		name    string
		zone    string
		instant time.Time
		want    cityclock.Part
	}{
		{"morning, ahead of UTC", "Asia/Ho_Chi_Minh", utc(2026, 11, 21, 2, 0), cityclock.Morning},          // 09:00
		{"afternoon starts at its hour", "Asia/Ho_Chi_Minh", utc(2026, 11, 21, 5, 0), cityclock.Afternoon}, // 12:00
		{"just before afternoon", "Asia/Ho_Chi_Minh", utc(2026, 11, 21, 4, 59), cityclock.Morning},         // 11:59
		{"evening", "Asia/Ho_Chi_Minh", utc(2026, 11, 21, 11, 0), cityclock.Evening},                       // 18:00
		{"last minute of the evening", "Asia/Ho_Chi_Minh", utc(2026, 11, 21, 14, 59), cityclock.Evening},   // 21:59
		{"after the evening ends", "Asia/Ho_Chi_Minh", utc(2026, 11, 21, 15, 0), cityclock.None},           // 22:00
		{"small hours are in no part", "Asia/Ho_Chi_Minh", utc(2026, 11, 20, 20, 0), cityclock.None},       // 03:00
		// 09:00 in LA is 17:00 UTC: morning there, evening by UTC's hours.
		{"behind UTC", "America/Los_Angeles", utc(2026, 11, 21, 17, 0), cityclock.Morning},
		// 10:00 UTC on 1 Nov is 03:00 PST, after the clocks went back.
		// 20:00 UTC on 1 Nov is 12:00 PST, the day the clocks went back.
		{"after fall-back", "America/Los_Angeles", utc(2026, 11, 1, 20, 0), cityclock.Afternoon},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := load(t, tt.zone).PartOfDay(tt.instant); got != tt.want {
				t.Fatalf("PartOfDay = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInWindowCrossingMidnight(t *testing.T) {
	// Quiet hours, 22:00 to 07:00, are the usual window that crosses midnight.
	tests := []struct {
		name    string
		zone    string
		instant time.Time
		want    bool
	}{
		{"before midnight, inside", "Asia/Ho_Chi_Minh", utc(2026, 11, 21, 15, 30), true}, // 22:30
		{"after midnight, inside", "Asia/Ho_Chi_Minh", utc(2026, 11, 20, 20, 0), true},   // 03:00
		{"start is inside", "Asia/Ho_Chi_Minh", utc(2026, 11, 21, 15, 0), true},          // 22:00
		{"end is outside", "Asia/Ho_Chi_Minh", utc(2026, 11, 20, 0, 0), false},           // 07:00
		{"midday, outside", "Asia/Ho_Chi_Minh", utc(2026, 11, 21, 5, 0), false},          // 12:00
		// 22:30 in LA (PST) is 06:30 UTC next day: inside there, outside by UTC's hours.
		{"behind UTC, inside", "America/Los_Angeles", utc(2026, 11, 22, 6, 30), true},
		// 03:00 PDT on 8 Mar is the first instant after the gap, inside the window.
		{"across the spring-forward", "America/Los_Angeles", utc(2026, 3, 8, 10, 0), true},
		// 07:30 PST on 1 Nov, after the repeated hour: outside.
		{"after the fall-back", "America/Los_Angeles", utc(2026, 11, 1, 15, 30), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := load(t, tt.zone).InWindow(tt.instant, "22:00", "07:00")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("InWindow = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInWindowWithinOneDay(t *testing.T) {
	c := load(t, "Asia/Ho_Chi_Minh")
	for _, tt := range []struct {
		instant time.Time
		want    bool
	}{
		{utc(2026, 11, 21, 2, 0), true},   // 09:00
		{utc(2026, 11, 21, 10, 0), false}, // 17:00, the end
		{utc(2026, 11, 21, 1, 59), false}, // 08:59
	} {
		got, err := c.InWindow(tt.instant, "09:00", "17:00")
		if err != nil || got != tt.want {
			t.Errorf("InWindow(%v) = %v, %v; want %v", tt.instant, got, err, tt.want)
		}
	}
}

func TestInWindowRejectsBadClockTimes(t *testing.T) {
	if _, err := load(t, "Asia/Ho_Chi_Minh").InWindow(utc(2026, 1, 1, 0, 0), "late", "07:00"); err == nil {
		t.Fatal("want an error")
	}
}
