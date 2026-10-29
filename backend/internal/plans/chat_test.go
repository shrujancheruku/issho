package plans

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/chat"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationChat: two people going and one invited. The chat is
// closed until 48 h before the start, open until 24 h after, readable but
// not writable after that, only for people going; messages are deleted 30
// days after the event.
func TestIntegrationChat(t *testing.T) {
	f := newFixture(t, "zz-plans-chat", "UTC", 3)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	a, b, c := f.users[0], f.users[1], f.users[2]
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	plan := matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: plan, Audience: 6, Score: 1}, Window{Start: day(7), Days: 14, Deadline: day(14)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	starts := day(12).Add(14 * time.Hour)
	ids := []matching.UserID{matching.UserID(a.String()), matching.UserID(b.String()), matching.UserID(c.String())}
	eventID, _, err := SaveEvent(ctx, f.pool, Event{
		PlanID: planID, Round: 1, StartsAt: starts, ConfirmBy: day(11), Now: day(5),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(12), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: ids, Quorum: 3, MinAttendance: 2, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: f.pool}
	for _, u := range []uuid.UUID{a, b} {
		if err := svc.Respond(ctx, u, eventID, Confirm, day(6)); err != nil {
			t.Fatal(err)
		}
	}

	// Closed until 48 h before.
	early := starts.Add(-chat.OpensBefore - time.Minute)
	if _, err := svc.SendChat(ctx, a, eventID, "hi", early); !errors.Is(err, ErrChatClosed) {
		t.Errorf("before it opens: %v, want ErrChatClosed", err)
	}
	if msgs, st, err := svc.ChatMessages(ctx, a, eventID, 0, early); err != nil || len(msgs) != 0 || st.Open || !st.OpensAt.Equal(starts.Add(-chat.OpensBefore)) {
		t.Errorf("reading before it opens: %v, %+v, %v", msgs, st, err)
	}

	open := starts.Add(-24 * time.Hour)
	first, err := svc.SendChat(ctx, a, eventID, "  Shall we meet at the north exit?  ", open)
	if err != nil {
		t.Fatal(err)
	}
	if first.Body != "Shall we meet at the north exit?" || !first.Mine {
		t.Errorf("sent = %+v (body should be trimmed)", first)
	}
	msgs, st, err := svc.ChatMessages(ctx, b, eventID, 0, open)
	if err != nil || !st.Open || len(msgs) != 1 || msgs[0].Mine || msgs[0].Name != "Test" || msgs[0].Body != first.Body {
		t.Errorf("b reads = %+v, %+v, %v", msgs, st, err)
	}
	for name, body := range map[string]string{"empty": "   ", "too long": strings.Repeat("あ", chat.MaxMessage+1)} {
		var inv *InvalidError
		if _, err := svc.SendChat(ctx, a, eventID, body, open); !errors.As(err, &inv) {
			t.Errorf("%s message: %v, want InvalidError", name, err)
		}
	}
	if _, err := svc.SendChat(ctx, a, eventID, strings.Repeat("あ", chat.MaxMessage), open); err != nil {
		t.Errorf("a message of exactly %d characters: %v", chat.MaxMessage, err)
	}
	// Only people going.
	if _, _, err := svc.ChatMessages(ctx, c, eventID, 0, open); !errors.Is(err, ErrNotGoing) {
		t.Errorf("someone only invited: %v, want ErrNotGoing", err)
	}
	if _, err := svc.SendChat(ctx, c, eventID, "hello?", open); !errors.Is(err, ErrNotGoing) {
		t.Errorf("someone only invited sending: %v, want ErrNotGoing", err)
	}
	// Reading after a message returns only what's newer.
	if _, err := svc.SendChat(ctx, b, eventID, "Yes, see you there", open); err != nil {
		t.Fatal(err)
	}
	if newer, _, err := svc.ChatMessages(ctx, a, eventID, first.ID, open); err != nil || len(newer) != 2 || newer[1].Body != "Yes, see you there" {
		t.Errorf("after the first message: %+v, %v", newer, err)
	}

	// After 24 h past the start: readable, not writable.
	late := starts.Add(chat.ClosesAfter)
	if _, err := svc.SendChat(ctx, a, eventID, "thanks!", late); !errors.Is(err, ErrChatClosed) {
		t.Errorf("after it closes: %v, want ErrChatClosed", err)
	}
	if msgs, st, err := svc.ChatMessages(ctx, a, eventID, 0, late); err != nil || st.Open || len(msgs) != 3 {
		t.Errorf("reading after it closes: %d messages, %+v, %v", len(msgs), st, err)
	}

	// Deleted 30 days after the event, not before.
	if n, err := chat.DeleteOld(ctx, f.q, f.city, day(12).Add(chat.Keep)); err != nil || n != 0 {
		t.Errorf("at 30 days: deleted %d, %v; want 0", n, err)
	}
	if n, err := chat.DeleteOld(ctx, f.q, f.city, day(13).Add(chat.Keep)); err != nil || n != 3 {
		t.Errorf("after 30 days: deleted %d, %v; want 3", n, err)
	}
}
