package notifications_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/notifications"
	"github.com/shrujancheruku/issho/backend/internal/push"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

const kind = "crew_formed" // any kind the table allows; the renderer here is a fake

// batchSender answers like Expo's client: it sends in batches of size, and
// when failAt is set the batch starting there fails, returning the tickets
// of the batches before it with the error.
type batchSender struct {
	size, failAt int
	sent         []push.Message
}

func (s *batchSender) Send(_ context.Context, msgs []push.Message) ([]push.Ticket, error) {
	var out []push.Ticket
	for start := 0; start < len(msgs); start += s.size {
		if s.failAt > 0 && start == s.failAt {
			return out, errors.New("expo unreachable")
		}
		for _, m := range msgs[start:min(start+s.size, len(msgs))] {
			s.sent = append(s.sent, m)
			out = append(out, push.Ticket{OK: true, ID: "ticket-" + m.To})
		}
	}
	return out, nil
}

func (s *batchSender) Receipts(context.Context, []string) (map[string]push.Receipt, error) {
	return nil, nil
}

type fakeRenderer struct{ calls int }

func (r *fakeRenderer) Render(_ context.Context, items []notifications.Pending) []notifications.Result {
	r.calls++
	out := make([]notifications.Result, len(items))
	for i, p := range items {
		out[i].Content = notifications.Content{Title: "T-" + p.Language, Body: "B"}
	}
	return out
}

// setup makes n users with a profile and one device each, and a crew
// to be notified about.
func setup(t *testing.T, id string, n int) (*testfixture.Fixture, uuid.UUID) {
	t.Helper()
	f := testfixture.New(t, id, "UTC", n)
	ctx := context.Background()
	for i, u := range f.Users {
		f.AddProfile(t, u, "ja")
		if err := f.Q.UpsertPushToken(ctx, dbgen.UpsertPushTokenParams{Token: tok(i), UserID: u, Platform: "android"}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	ts := pgtype.Timestamptz{Time: now, Valid: true}
	crew, err := f.Q.CreateCrew(ctx, dbgen.CreateCrewParams{CityID: f.City, ActivityID: &f.Act, OfferUntil: ts, At: ts})
	if err != nil {
		t.Fatal(err)
	}
	return f, crew
}

func tok(i int) string { return "ExponentPushToken[zz-notif-" + string(rune('a'+i)) + "]" }

func newNotifier(f *testfixture.Fixture, s push.Sender, r notifications.Renderer) *notifications.Notifier {
	n := &notifications.Notifier{DB: f.Pool, Sender: s, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if r != nil {
		n.Register(r, kind)
	}
	return n
}

func state(t *testing.T, f *testfixture.Fixture, u uuid.UUID) (sent bool, attempts int) {
	t.Helper()
	if err := f.Pool.QueryRow(context.Background(), "select sent_at is not null, attempts from app.notifications where user_id = $1", u).Scan(&sent, &attempts); err != nil {
		t.Fatal(err)
	}
	return sent, attempts
}

// TestIntegrationPartialSend: when Expo fails on the second batch, the
// notifications of the first batch are sent (with their tickets stored) and
// only the rest are marked failed, so nobody gets a push twice.
func TestIntegrationPartialSend(t *testing.T) {
	f, crew := setup(t, "zz-notif-partial", 5)
	ctx := context.Background()
	if err := notifications.Enqueue(ctx, f.Q, kind, f.Users, notifications.Ref{CrewID: crew}); err != nil {
		t.Fatal(err)
	}
	// Batches of 2: users 0-1 go out, the batch starting at 2 fails.
	sender := &batchSender{size: 2, failAt: 2}
	rend := &fakeRenderer{}
	n := newNotifier(f, sender, rend)
	now := time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)

	rep, err := n.Send(ctx, now)
	if err == nil {
		t.Fatal("want the send error")
	}
	if rep.Sent != 2 || rep.Failed != 3 {
		t.Errorf("report = %+v, want 2 sent and 3 failed", rep)
	}
	for i, u := range f.Users {
		sent, attempts := state(t, f, u)
		if wantSent := i < 2; sent != wantSent || attempts != map[bool]int{true: 0, false: 1}[wantSent] {
			t.Errorf("user %d: sent=%v attempts=%d", i, sent, attempts)
		}
	}
	var tickets int
	if err := f.Pool.QueryRow(ctx, "select count(*) from app.push_tickets where token like 'ExponentPushToken[zz-notif-%'").Scan(&tickets); err != nil || tickets != 2 {
		t.Errorf("%d tickets stored (%v), want 2", tickets, err)
	}
	if rend.calls != 1 {
		t.Errorf("renderer called %d times, want once for the whole pass", rend.calls)
	}
	if m := sender.sent[0]; m.Title != "T-ja" || m.Data["kind"] != kind || m.Data["crewId"] != crew.String() {
		t.Errorf("message = %+v", m)
	}

	// The next pass sends only the three left.
	sender.failAt, sender.sent = 0, nil
	if rep, err := n.Send(ctx, now.Add(time.Minute)); err != nil || rep.Sent != 3 || len(sender.sent) != 3 {
		t.Errorf("retry = %+v, %v, %d messages; want just the 3 left", rep, err, len(sender.sent))
	}
}

// TestIntegrationNoRenderer: a kind nobody registered fails (and runs out of
// attempts) instead of blocking the others or being sent blank.
func TestIntegrationNoRenderer(t *testing.T) {
	f, crew := setup(t, "zz-notif-norenderer", 1)
	ctx := context.Background()
	if err := notifications.Enqueue(ctx, f.Q, kind, f.Users, notifications.Ref{CrewID: crew}); err != nil {
		t.Fatal(err)
	}
	sender := &batchSender{size: 100}
	n := newNotifier(f, sender, nil)
	rep, err := n.Send(ctx, time.Now())
	if err != nil || rep.Failed != 1 || len(sender.sent) != 0 {
		t.Errorf("report = %+v, %v, %d sent; want one failed and nothing sent", rep, err, len(sender.sent))
	}
	if sent, attempts := state(t, f, f.Users[0]); sent || attempts != 1 {
		t.Errorf("sent=%v attempts=%d", sent, attempts)
	}
}
