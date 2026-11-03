package plans

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationListCaps: ListFeed and ListMyEvents return at most their
// cap, and warn when they hit it. The worker's ListOpen is never capped.
func TestIntegrationListCaps(t *testing.T) {
	f := newFixture(t, "zz-plans-cap", "UTC", 2)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	day := func(d, h int) time.Time { return time.Date(2030, 1, d, h, 0, 0, 0, time.UTC) }
	plan := matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}
	var lastEvent uuid.UUID
	ids := []matching.UserID{matching.UserID(f.users[0].String()), matching.UserID(f.users[1].String())}
	for i := 1; i <= 3; i++ {
		planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: plan, Audience: 6, Score: 1}, Window{Start: day(7, 0), Days: 14, Deadline: day(14, 0)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		id, _, err := SaveEvent(ctx, f.pool, Event{
			PlanID: planID, Round: 1, StartsAt: day(10+i, 14), ConfirmBy: day(9, 0), Now: day(5, 0),
			Scheduled: matching.Scheduled{
				Occasion: matching.Occasion{Date: day(10+i, 0), Block: matching.Afternoon},
				Groups:   []matching.Group{{Invited: ids, Quorum: 2, MinAttendance: 2, Score: 1}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		lastEvent = id
	}
	// Scheduling closes a plan's round, so open ones for the feed are new.
	for i := 0; i < 3; i++ {
		if _, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: plan, Audience: 6, Score: 1}, Window{Start: day(7, 0), Days: 14, Deadline: day(14, 0)}, nil); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	svc := &Service{DB: f.pool, Log: slog.New(slog.NewTextHandler(&buf, nil))}
	now := day(8, 0)

	open, err := ListOpen(ctx, f.q, f.city)
	if err != nil || len(open) < 3 {
		t.Fatalf("ListOpen = %d plans, %v; want at least 3 and no cap", len(open), err)
	}
	if evs, err := svc.Events(ctx, f.users[0], now); err != nil || len(evs) != 3 {
		t.Fatalf("under the cap: %d events, %v; want 3", len(evs), err)
	}

	defer func(m int32) { EventsCap.Max = m }(EventsCap.Max)
	defer func(m int32) { FeedCap.Max = m }(FeedCap.Max)
	EventsCap.Max, FeedCap.Max = 2, 2

	evs, err := svc.Events(ctx, f.users[0], now)
	if err != nil || len(evs) != 2 {
		t.Fatalf("at the cap: %d events, %v; want 2", len(evs), err)
	}
	if !evs[0].StartsAt.Equal(day(11, 14)) {
		t.Errorf("first event starts %v; the cap should cut the latest, not the soonest", evs[0].StartsAt)
	}
	feed, err := svc.Feed(ctx, f.users[0])
	if err != nil || len(feed) != 2 {
		t.Fatalf("feed at the cap: %d plans, %v; want 2", len(feed), err)
	}
	for _, want := range []string{"rpc=ListMyEvents", "rpc=ListFeed", "cap=2"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, buf.String())
		}
	}

	// GetEvent reads the same query without a cap.
	EventsCap.Max = 1
	if _, _, _, err := svc.Event(ctx, f.users[0], lastEvent, now); err != nil {
		t.Errorf("Event with a cap of 1: %v", err)
	}
}
