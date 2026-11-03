package chat

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// TestIntegrationDeleteOldCrewChats: a crew's messages go 30 days after it
// closed or lapsed. An active crew's stay, even after a member left it.
func TestIntegrationDeleteOldCrewChats(t *testing.T) {
	f := testfixture.New(t, "zz-chat-crewret", "UTC", 2)
	ctx := context.Background()
	now := time.Date(2030, 3, 1, 12, 0, 0, 0, time.UTC)
	ago := func(days int) *time.Time {
		at := now.Add(-time.Duration(days) * 24 * time.Hour)
		return &at
	}
	// crew makes a crew of both users with one message from each.
	crew := func(status string, closedAt *time.Time) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := f.Pool.QueryRow(ctx, `insert into app.crews (city_id, activity_id, status, offer_until, closed_at) values ($1, $2, $3, $4, $5) returning id`,
			f.City, f.Act, status, now, closedAt).Scan(&id); err != nil {
			t.Fatal(err)
		}
		for _, u := range f.Users {
			if _, err := f.Pool.Exec(ctx, `insert into app.crew_members (crew_id, user_id) values ($1, $2)`, id, u); err != nil {
				t.Fatal(err)
			}
			if _, err := f.Pool.Exec(ctx, `insert into app.crew_messages (crew_id, user_id, body) values ($1, $2, 'hi')`, id, u); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	closed31 := crew("closed", ago(31))
	closed29 := crew("closed", ago(29))
	lapsed31 := crew("lapsed", ago(31))
	active := crew("active", nil)
	// A member leaves the active crew; the messages stay with the crew.
	if _, err := f.Pool.Exec(ctx, `delete from app.crew_members where crew_id = $1 and user_id = $2`, active, f.Users[1]); err != nil {
		t.Fatal(err)
	}
	n, err := DeleteOldCrewChats(ctx, f.Q, now)
	if err != nil || n != 4 {
		t.Fatalf("deleted %d, %v; want 4 (two crews of two messages)", n, err)
	}
	for name, c := range map[string]struct {
		id   uuid.UUID
		want int
	}{"closed 31 days ago": {closed31, 0}, "lapsed 31 days ago": {lapsed31, 0}, "closed 29 days ago": {closed29, 2}, "active, a member left": {active, 2}} {
		var got int
		if err := f.Pool.QueryRow(ctx, `select count(*) from app.crew_messages where crew_id = $1`, c.id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s: %d messages left, want %d", name, got, c.want)
		}
	}
}
