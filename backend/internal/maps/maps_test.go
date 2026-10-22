package maps

import (
	"testing"
	"time"
)

func TestOpenAt(t *testing.T) {
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	at := func(day, hour, minute int) time.Time { return time.Date(2030, 1, 6+day, hour, minute, 0, 0, tokyo) } // 6 Jan 2030 is a Sunday
	// Open 11:00–22:00 Monday to Saturday, and a bar open Saturday 20:00 to Sunday 02:00.
	cafe := Hours{Known: true}
	for d := time.Monday; d <= time.Saturday; d++ {
		cafe.Periods = append(cafe.Periods, Period{Open: WeekTime{d, 11, 0}, Close: WeekTime{d, 22, 0}})
	}
	bar := Hours{Known: true, Periods: []Period{{Open: WeekTime{time.Saturday, 20, 0}, Close: WeekTime{time.Sunday, 2, 0}}}}
	for _, c := range []struct {
		name string
		h    Hours
		at   time.Time
		want bool
	}{
		{"cafe, Monday 14:00", cafe, at(1, 14, 0), true},
		{"cafe, Monday 10:59", cafe, at(1, 10, 59), false},
		{"cafe, Monday 22:00 (closing time)", cafe, at(1, 22, 0), false},
		{"cafe, Sunday", cafe, at(0, 14, 0), false},
		{"bar, Saturday 23:00", bar, at(6, 23, 0), true},
		{"bar, Sunday 01:30 (after the week wraps)", bar, at(7, 1, 30), true},
		{"bar, Sunday 03:00", bar, at(7, 3, 0), false},
		{"24/7", Hours{Known: true, Periods: []Period{{Open: WeekTime{time.Sunday, 0, 0}, NoClose: true}}}, at(3, 4, 0), true},
		{"no hours known (a park)", Hours{}, at(3, 4, 0), true},
		{"temporarily closed", Hours{Known: true, Closed: true, Periods: cafe.Periods}, at(1, 14, 0), false},
	} {
		if got := c.h.OpenAt(c.at); got != c.want {
			t.Errorf("%s: OpenAt = %v, want %v", c.name, got, c.want)
		}
	}
}
