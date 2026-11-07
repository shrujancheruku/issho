package chat

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// TestIntegrationDeleteOldCrewMessages: the one retention delete covers both
// kinds of room. A crew's messages go 30 days after it closed or lapsed (an
// active crew's stay, even after a member left it); an outing's go 30 days
// after its event.
func TestIntegrationDeleteOldCrewMessages(t *testing.T) {
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
		if err := f.Pool.QueryRow(ctx, `with c as (insert into app.crews (city_id, activity_id, status, offer_until, closed_at) values ($1, $2, $3, $4, $5) returning id),
			r as (insert into app.rooms (kind, crew_id) select 'crew', id from c) select id from c`,
			f.City, f.Act, status, now, closedAt).Scan(&id); err != nil {
			t.Fatal(err)
		}
		for _, u := range f.Users {
			if _, err := f.Pool.Exec(ctx, `insert into app.crew_members (crew_id, user_id) values ($1, $2)`, id, u); err != nil {
				t.Fatal(err)
			}
			if _, err := f.Pool.Exec(ctx, `insert into app.messages (room_id, user_id, body) select id, $2, 'hi' from app.rooms where crew_id = $1`, id, u); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	// outing makes an outing room with one message, for an event on the date.
	outing := func(date time.Time) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := f.Pool.QueryRow(ctx, `
			with p as (
			  insert into app.plans (city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline)
			  values ($1, $2, $3, 'en', 6, 1, $4::timestamptz::date, 14, $4::timestamptz) returning id),
			e as (
			  insert into app.events (plan_id, round, date, block, starts_at, confirm_by, forced)
			  select id, 1, $4::timestamptz::date, 'evening', $4::timestamptz, $4::timestamptz, false from p returning id),
			g as (
			  insert into app.event_groups (event_id, quorum, min_attendance, score) select id, 2, 2, 1 from e returning id),
			r as (
			  insert into app.rooms (kind, group_id) select 'outing', id from g returning id),
			m as (
			  insert into app.messages (room_id, user_id, body) select id, $5, 'hi' from r)
			select id from r`, f.City, f.Area, f.Act, date, f.Users[0]).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	outing31 := outing(now.AddDate(0, 0, -31))
	outing29 := outing(now.AddDate(0, 0, -29))
	closed31 := crew("closed", ago(31))
	closed29 := crew("closed", ago(29))
	lapsed31 := crew("lapsed", ago(31))
	active := crew("active", nil)
	// A member leaves the active crew; the messages stay with the crew.
	if _, err := f.Pool.Exec(ctx, `delete from app.crew_members where crew_id = $1 and user_id = $2`, active, f.Users[1]); err != nil {
		t.Fatal(err)
	}
	n, err := DeleteOld(ctx, f.Q, now)
	if err != nil || n != 5 {
		t.Fatalf("deleted %d, %v; want 5 (two crews of two messages, one outing of one)", n, err)
	}
	for name, c := range map[string]struct {
		id   uuid.UUID
		want int
	}{"outing 31 days ago": {outing31, 0}, "outing 29 days ago": {outing29, 1}, "closed 31 days ago": {closed31, 0}, "lapsed 31 days ago": {lapsed31, 0}, "closed 29 days ago": {closed29, 2}, "active, a member left": {active, 2}} {
		var got int
		if err := f.Pool.QueryRow(ctx, `select count(*) from app.messages m join app.rooms r on r.id = m.room_id where r.id = $1 or r.crew_id = $1`, c.id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s: %d messages left, want %d", name, got, c.want)
		}
	}
}
