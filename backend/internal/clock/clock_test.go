package clock_test

import (
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/clock"
)

func TestFakeReportsWhatTestsSetAndAdvance(t *testing.T) {
	start := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	var c clock.Clock = clock.NewFake(start)

	if got := c.Now(); !got.Equal(start) {
		t.Fatalf("Now() = %v, want %v", got, start)
	}

	f := c.(*clock.Fake)
	f.Advance(90 * time.Minute)
	if got, want := c.Now(), time.Date(2026, 11, 21, 10, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("after Advance, Now() = %v, want %v", got, want)
	}

	jump := time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC)
	f.Set(jump)
	if got := c.Now(); !got.Equal(jump) {
		t.Fatalf("after Set, Now() = %v, want %v", got, jump)
	}
}

// The fake does not move on its own: two reads with no Advance agree.
func TestFakeStandsStill(t *testing.T) {
	f := clock.NewFake(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	a := f.Now()
	time.Sleep(2 * time.Millisecond)
	if b := f.Now(); !a.Equal(b) {
		t.Fatalf("fake moved by itself: %v then %v", a, b)
	}
}

func TestRealFollowsTheSystemClock(t *testing.T) {
	before := time.Now()
	got := clock.Real{}.Now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Fatalf("Real.Now() = %v, outside [%v, %v]", got, before, after)
	}
}
