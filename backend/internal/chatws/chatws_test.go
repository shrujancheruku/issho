package chatws

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/crews"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

const testIssuer = "http://test.local"

// TestIntegrationSocket opens a chat socket as someone going, receives the
// earlier messages, then a message another person sends (through NOTIFY),
// then that person typing on their own socket; a bad token and someone not
// going are turned away with their close codes.
func TestIntegrationSocket(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	going, other, invited := uuid.New(), uuid.New(), uuid.New()
	eventID := seed(t, ctx, pool, going, other, invited)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier := auth.NewVerifierWithKeyfunc(func(*jwt.Token) (any, error) { return &key.PublicKey, nil }, testIssuer)
	token := func(u uuid.UUID) string {
		s, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
			"sub": u.String(), "aud": "authenticated", "iss": testIssuer + "/auth/v1",
			"exp": time.Now().Add(time.Hour).Unix(),
		}).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &plans.Service{DB: pool}
	hub := NewHub()
	go hub.Run(ctx, pool, log)
	srv := httptest.NewServer(&Handler{Hub: hub, Plans: svc, Crews: &crews.Service{DB: pool}, Verifier: verifier, Log: log})
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	if _, err := svc.SendChat(ctx, other, eventID, "first", time.Now()); err != nil {
		t.Fatal(err)
	}
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	hello, _ := json.Marshal(map[string]any{"token": token(going), "eventId": eventID.String(), "afterId": 0})
	if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
		t.Fatal(err)
	}
	// A frame: messages, the ready notice or someone typing. Not t.Fatal:
	// it also runs on other goroutines.
	type frame struct {
		batch
		Typing bool   `json:"typing"`
		Name   string `json:"name"`
	}
	readFrom := func(c *websocket.Conn) (frame, error) {
		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		var f frame
		_, raw, err := c.Read(rctx)
		if err == nil {
			err = json.Unmarshal(raw, &f)
		}
		return f, err
	}
	read := func() batch {
		f, err := readFrom(conn)
		if err != nil {
			t.Error(err)
		}
		return f.batch
	}
	if f, err := readFrom(conn); err != nil || f.Type != "ready" || !f.Typing {
		t.Fatalf("first frame = %+v, %v; want ready", f, err)
	}
	if b := read(); b.Type != "messages" || len(b.Messages) != 1 || b.Messages[0].Body != "first" || b.Messages[0].Mine || b.Messages[0].Name != "Other" {
		t.Errorf("history = %+v", b)
	}
	// A message sent afterwards arrives without asking. The hub may still be
	// starting to listen, so send until one gets through.
	var pushed batch
	got := make(chan batch, 1)
	go func() { got <- read() }()
	for i := 0; ; i++ {
		if _, err := svc.SendChat(ctx, other, eventID, "second", time.Now()); err != nil {
			t.Fatal(err)
		}
		select {
		case pushed = <-got:
		case <-time.After(500 * time.Millisecond):
			if i < 10 {
				continue
			}
			t.Fatal("no message pushed")
		}
		break
	}
	if len(pushed.Messages) == 0 || pushed.Messages[len(pushed.Messages)-1].Body != "second" {
		t.Errorf("pushed = %+v", pushed)
	}

	// The other person types on their own socket: going is told who, and
	// the other person isn't told about themselves.
	otherConn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer otherConn.CloseNow()
	last := pushed.Messages[len(pushed.Messages)-1].ID
	otherHello, _ := json.Marshal(map[string]any{"token": token(other), "eventId": eventID.String(), "afterId": last})
	if err := otherConn.Write(ctx, websocket.MessageText, otherHello); err != nil {
		t.Fatal(err)
	}
	if f, err := readFrom(otherConn); err != nil || f.Type != "ready" {
		t.Fatalf("other's first frame = %+v, %v", f, err)
	}
	typingFrame, _ := json.Marshal(map[string]string{"type": "typing"})
	if err := otherConn.Write(ctx, websocket.MessageText, typingFrame); err != nil {
		t.Fatal(err)
	}
	otherGot := make(chan frame, 1)
	go func() {
		if f, err := readFrom(otherConn); err == nil {
			otherGot <- f
		}
	}()
	if f, err := readFrom(conn); err != nil || f.Type != "typing" || f.Name != "Other" {
		t.Errorf("typing = %+v, %v; want Other typing", f, err)
	}
	select {
	case f := <-otherGot:
		t.Errorf("the typer was sent %+v", f)
	case <-time.After(300 * time.Millisecond):
	}

	// Turned away: a bad token, and someone only invited.
	for _, c := range []struct {
		name  string
		hello map[string]any
		code  websocket.StatusCode
	}{
		{"bad token", map[string]any{"token": "nope", "eventId": eventID.String()}, closeUnauthorized},
		{"not going", map[string]any{"token": token(invited), "eventId": eventID.String()}, closeForbidden},
		{"unknown event", map[string]any{"token": token(going), "eventId": uuid.NewString()}, closeNotFound},
	} {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(c.hello)
		_ = conn.Write(ctx, websocket.MessageText, b)
		_, _, err = conn.Read(ctx)
		var ce websocket.CloseError
		if !errors.As(err, &ce) || ce.Code != c.code {
			t.Errorf("%s: %v, want close %d", c.name, err, c.code)
		}
		conn.CloseNow()
	}
}

// seed creates a throwaway city, plan and event starting in an hour (so its
// chat is open), with one group: going and other confirmed, invited not.
// Everything is deleted when the test ends.
func seed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, going, other, invited uuid.UUID) uuid.UUID {
	t.Helper()
	var supabase bool
	if err := pool.QueryRow(ctx, "select exists (select 1 from pg_namespace where nspname = 'auth')").Scan(&supabase); err != nil {
		t.Fatal(err)
	}
	users := []uuid.UUID{going, other, invited}
	// An interrupted run may have left its users (random ids, found by their
	// home area), city and activity behind.
	if supabase {
		_, _ = pool.Exec(ctx, "delete from auth.users where id in (select user_id from app.profiles where home_area_id = 'zz-chatws-a')")
	}
	_, _ = pool.Exec(ctx, "delete from app.users where id in (select user_id from app.profiles where home_area_id = 'zz-chatws-a')")
	_, _ = pool.Exec(ctx, "delete from app.cities where id = 'zz-chatws'")
	_, _ = pool.Exec(ctx, "delete from app.activities where id = 'zz-chatws'")
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, "delete from app.users where id = any($1)", users)
		if supabase {
			_, _ = pool.Exec(ctx, "delete from auth.users where id = any($1)", users)
		}
		_, _ = pool.Exec(ctx, "delete from app.cities where id = 'zz-chatws'")
		_, _ = pool.Exec(ctx, "delete from app.activities where id = 'zz-chatws'")
	})
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`insert into app.cities (id, name, timezone, currency, languages, travel_modes) values ('zz-chatws', '{"en":"T"}', 'UTC', 'USD', '{en}', '{walk}')`)
	exec(`insert into app.areas (id, city_id, name, centroid) values ('zz-chatws-a', 'zz-chatws', '{"en":"A"}', 'POINT(0 0)')`)
	exec(`insert into app.activities (id, name, category, indoor, energy, cost_level, conversational, min_group_size, max_group_size)
	      values ('zz-chatws', '{"en":"T"}', 'test', true, 1, 0, true, 2, 6)`)
	for i, u := range users {
		if supabase {
			exec("insert into auth.users (id, email) values ($1, $2)", u, u.String()+"@example.com")
		}
		exec("insert into app.users (id) values ($1)", u)
		exec(`insert into app.profiles (user_id, display_name, birth_year, gender, home_area_id, max_travel_min, energy, budget, size_pref, primary_language, conduct_version)
		      values ($1, $2, 1995, 'unspecified', 'zz-chatws-a', 30, 2, 1, 'medium', 'en', 1)`, u, []string{"Going", "Other", "Invited"}[i])
	}
	var eventID uuid.UUID
	err := pool.QueryRow(ctx, `
		with p as (
		  insert into app.plans (city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline)
		  values ('zz-chatws', 'zz-chatws-a', 'zz-chatws', 'en', 6, 1, current_date, 14, now() + interval '7 days') returning id),
		e as (
		  insert into app.events (plan_id, round, date, block, starts_at, confirm_by, forced)
		  select id, 1, current_date, 'evening', now() + interval '1 hour', now() - interval '1 hour', false from p returning id)
		select id from e`).Scan(&eventID)
	if err != nil {
		t.Fatal(err)
	}
	var groupID uuid.UUID
	if err := pool.QueryRow(ctx, `insert into app.event_groups (event_id, quorum, min_attendance, score, status) values ($1, 2, 2, 1, 'confirmed') returning id`, eventID).Scan(&groupID); err != nil {
		t.Fatal(err)
	}
	for u, r := range map[uuid.UUID]string{going: "confirmed", other: "confirmed", invited: "invited"} {
		at := any(time.Now())
		if r == "invited" {
			at = nil
		}
		exec("insert into app.group_members (group_id, event_id, user_id, response, responded_at) values ($1, $2, $3, $4, $5)", groupID, eventID, u, r, at)
	}
	return eventID
}
