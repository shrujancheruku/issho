package chat

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// TestIntegrationRoomWrittenWithOwner pins that a group and a crew get their
// room in the same transaction: the queries that create them write the room
// too, so a rolled-back save leaves neither.
func TestIntegrationRoomWrittenWithOwner(t *testing.T) {
	f := testfixture.New(t, "zz-chat-roomtx", "UTC", 1)
	ctx := context.Background()
	now := time.Date(2030, 3, 1, 12, 0, 0, 0, time.UTC)
	var eventID uuid.UUID
	if err := f.Pool.QueryRow(ctx, `
		with p as (
		  insert into app.plans (city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline)
		  values ($1, $2, $3, 'en', 6, 1, $4::timestamptz::date, 14, $4::timestamptz) returning id)
		insert into app.events (plan_id, round, date, block, starts_at, confirm_by, forced)
		select id, 1, $4::timestamptz::date, 'evening', $4::timestamptz, $4::timestamptz, false from p returning id`, f.City, f.Area, f.Act, now).Scan(&eventID); err != nil {
		t.Fatal(err)
	}

	tx, err := f.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op once rolled back
	q := dbgen.New(tx)
	groupID, err := q.InsertEventGroup(ctx, dbgen.InsertEventGroupParams{EventID: eventID, Quorum: 2, MinAttendance: 2, Score: 1})
	if err != nil {
		t.Fatal(err)
	}
	ts := pgtype.Timestamptz{Time: now, Valid: true}
	crewID, err := q.CreateCrew(ctx, dbgen.CreateCrewParams{CityID: f.City, ActivityID: &f.Act, OfferUntil: ts, At: ts})
	if err != nil {
		t.Fatal(err)
	}
	// Inside the transaction each has its room, found by its id.
	if _, err := q.GetGroupRoom(ctx, pgtype.UUID{Bytes: groupID, Valid: true}); err != nil {
		t.Errorf("group room inside the transaction: %v", err)
	}
	if _, err := q.GetCrewRoom(ctx, pgtype.UUID{Bytes: crewID, Valid: true}); err != nil {
		t.Errorf("crew room inside the transaction: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := f.Pool.QueryRow(ctx, `select count(*) from app.rooms`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d rooms left after the rollback, want none", left)
	}
}
