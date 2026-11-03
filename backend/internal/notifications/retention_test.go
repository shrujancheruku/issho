package notifications_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/notifications"
)

// TestIntegrationRetention: sent and exhausted notifications are deleted
// once 30 days old; unsent ones with attempts left, and recent ones, stay.
func TestIntegrationRetention(t *testing.T) {
	f, crew := setup(t, "zz-notif-retention", 5)
	ctx := context.Background()
	now := time.Date(2030, 3, 1, 0, 0, 0, 0, time.UTC)
	ago := func(days int) time.Time { return now.Add(-time.Duration(days) * 24 * time.Hour) }
	// user 0: sent, 31 days; 1: exhausted, 31 days; 2: unsent with attempts
	// left, 31 days; 3: sent, 29 days; 4: exhausted, 29 days.
	rows := []struct {
		created time.Time
		sent    bool
		tries   int
	}{{ago(31), true, 1}, {ago(31), false, 5}, {ago(31), false, 4}, {ago(29), true, 1}, {ago(29), false, 5}}
	for i, r := range rows {
		var sentAt *time.Time
		if r.sent {
			sentAt = &r.created
		}
		if _, err := f.Pool.Exec(ctx, `insert into app.notifications (user_id, kind, crew_id, created_at, sent_at, attempts) values ($1, $2, $3, $4, $5, $6)`,
			f.Users[i], kind, crew, r.created, sentAt, r.tries); err != nil {
			t.Fatal(err)
		}
	}
	n, err := notifications.DeleteOld(ctx, f.Q, now)
	if err != nil || n != 2 {
		t.Fatalf("deleted %d, %v; want 2", n, err)
	}
	rs, err := f.Pool.Query(ctx, `select user_id from app.notifications where crew_id = $1`, crew)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	kept := map[uuid.UUID]bool{}
	for rs.Next() {
		var u uuid.UUID
		if err := rs.Scan(&u); err != nil {
			t.Fatal(err)
		}
		kept[u] = true
	}
	for i, want := range []bool{false, false, true, true, true} {
		if kept[f.Users[i]] != want {
			t.Errorf("user %d's notification kept = %v, want %v", i, kept[f.Users[i]], want)
		}
	}
}
