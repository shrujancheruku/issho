package notifications_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/notifications"
)

// byUserRenderer answers per person: users in skip are Skip, users in
// fail are errors, everyone else gets content.
type byUserRenderer struct{ skip, fail map[uuid.UUID]bool }

func (r *byUserRenderer) Render(_ context.Context, items []notifications.Pending) []notifications.Result {
	out := make([]notifications.Result, len(items))
	for i, p := range items {
		switch {
		case r.fail[p.UserID]:
			out[i].Err = errors.New("render failed")
		case r.skip[p.UserID]:
			out[i].Skip = true
		default:
			out[i].Content = notifications.Content{Title: "T", Body: "B"}
		}
	}
	return out
}

// TestIntegrationRenderPerItem: one item the renderer errors on doesn't stop
// the batch. The rest are sent, the failed one's attempts go up by one and
// it stays queued, and one whose subject is gone (skip) is finished with no
// push.
func TestIntegrationRenderPerItem(t *testing.T) {
	f, crew := setup(t, "zz-notif-peritem", 4)
	ctx := context.Background()
	if err := notifications.Enqueue(ctx, f.Q, kind, f.Users, notifications.Ref{CrewID: crew}); err != nil {
		t.Fatal(err)
	}
	sender := &batchSender{size: 100}
	n := newNotifier(f, sender, &byUserRenderer{
		fail: map[uuid.UUID]bool{f.Users[1]: true},
		skip: map[uuid.UUID]bool{f.Users[2]: true},
	})
	// Other packages' tests share the database, so their pending
	// notifications may be in this pass too: judge only this test's own.
	if _, err := n.Send(ctx, time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	for i, want := range []struct {
		sent     bool
		attempts int
	}{{true, 0}, {false, 1}, {true, 0}, {true, 0}} {
		if sent, attempts := state(t, f, f.Users[i]); sent != want.sent || attempts != want.attempts {
			t.Errorf("user %d: sent=%v attempts=%d, want %+v", i, sent, attempts, want)
		}
	}
	pushed := map[string]bool{}
	for _, m := range sender.sent {
		pushed[m.To] = true
	}
	if !pushed[tok(0)] || !pushed[tok(3)] || pushed[tok(1)] || pushed[tok(2)] {
		t.Errorf("pushed to %v; want just users 0 and 3 (1 failed to render, 2 was skipped)", pushed)
	}
}
