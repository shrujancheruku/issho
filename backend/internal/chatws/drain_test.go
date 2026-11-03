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
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/chat"
	"github.com/shrujancheruku/issho/backend/internal/crews"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// TestIntegrationSocketDrainAndRevoke: more than a page waiting is sent in
// pages straight away, and again when a signal finds more than a page that
// piled up; a person taken out of the group is closed with 4403 by the next
// signal, or by their next typing notice.
func TestIntegrationSocketDrainAndRevoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := testfixture.Pool(t)
	going, other, invited := uuid.New(), uuid.New(), uuid.New()
	eventID := seed(t, ctx, pool, going, other, invited)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier := auth.NewVerifierWithKeyfunc(func(*jwt.Token) (any, error) { return &key.PublicKey, nil }, testIssuer)
	token, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": going.String(), "aud": "authenticated", "iss": testIssuer + "/auth/v1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	rooms := &chat.Service{DB: pool, Event: plans.ChatAccess{}, Crew: crews.ChatAccess{}}
	room := chat.Room{Kind: chat.EventRoom, ID: eventID}
	hub := NewHub()
	go hub.Run(ctx, pool, log)
	srv := httptest.NewServer(&Handler{Hub: hub, Chat: rooms, Verifier: verifier, Log: log})
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	var groupID uuid.UUID
	if err := pool.QueryRow(ctx, "select id from app.event_groups where event_id = $1", eventID).Scan(&groupID); err != nil {
		t.Fatal(err)
	}
	add := func(n int, prefix string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `insert into app.chat_messages (group_id, user_id, body) select $1, $2, $3 || g from generate_series(1, $4) g`,
			groupID, other, prefix, n); err != nil {
			t.Fatal(err)
		}
	}
	dial := func(afterID int64) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.CloseNow() })
		hello, _ := json.Marshal(map[string]any{"token": token, "room": map[string]any{"eventId": eventID.String()}, "afterId": afterID})
		if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	// read returns the bodies of messages up to the one named `until`, and
	// how many frames they came in.
	read := func(conn *websocket.Conn, until string) (bodies []string, frames int) {
		t.Helper()
		for {
			rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, raw, err := conn.Read(rctx)
			cancel()
			if err != nil {
				t.Fatalf("waiting for %q after %d messages: %v", until, len(bodies), err)
			}
			var b batch
			if err := json.Unmarshal(raw, &b); err != nil {
				t.Fatal(err)
			}
			if b.Type != "messages" {
				continue
			}
			frames++
			for _, m := range b.Messages {
				bodies = append(bodies, m.Body)
				if m.Body == until {
					return
				}
			}
		}
	}

	// 250 waiting, asked for from before the first: pages of 100, in order.
	add(250, "old")
	var before int64
	if err := pool.QueryRow(ctx, "select min(id) - 1 from app.chat_messages where group_id = $1", groupID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	conn := dial(before)
	got, frames := read(conn, "old250")
	if len(got) != 250 || frames != 3 || got[0] != "old1" {
		t.Errorf("history: %d messages in %d frames, first %q; want 250 in 3 from old1", len(got), frames, got[0])
	}

	// More than a page piles up behind one signal.
	add(150, "new")
	if _, err := rooms.Send(ctx, other, room, "tail", time.Now()); err != nil {
		t.Fatal(err)
	}
	got, frames = read(conn, "tail")
	if len(got) != 151 || frames != 2 || got[0] != "new1" {
		t.Errorf("piled up: %d messages in %d frames, first %q; want 151 in 2 from new1", len(got), frames, got[0])
	}

	// Taken out of the group: the next typing notice closes a second
	// socket, and the next signal closes the first.
	typer := dial(1 << 40)
	if f := readFrame(t, ctx, typer); f.Type != "ready" {
		t.Fatalf("typer's first frame = %+v", f)
	}
	if _, err := pool.Exec(ctx, "update app.group_members set response = 'dropped' where group_id = $1 and user_id = $2", groupID, going); err != nil {
		t.Fatal(err)
	}
	typingFrame, _ := json.Marshal(map[string]string{"type": "typing"})
	if err := typer.Write(ctx, websocket.MessageText, typingFrame); err != nil {
		t.Fatal(err)
	}
	if code := closeCode(t, ctx, typer); code != closeForbidden {
		t.Errorf("typing after being removed: closed with %d, want %d", code, closeForbidden)
	}
	if _, err := rooms.Send(ctx, other, room, "gone?", time.Now()); err != nil {
		t.Fatal(err)
	}
	if code := closeCode(t, ctx, conn); code != closeForbidden {
		t.Errorf("after being removed: closed with %d, want %d", code, closeForbidden)
	}
}

func readFrame(t *testing.T, ctx context.Context, conn *websocket.Conn) batch {
	t.Helper()
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, raw, err := conn.Read(rctx)
	if err != nil {
		t.Fatal(err)
	}
	var b batch
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	return b
}

// closeCode reads until the server closes the socket and returns its code.
func closeCode(t *testing.T, ctx context.Context, conn *websocket.Conn) websocket.StatusCode {
	t.Helper()
	for {
		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, _, err := conn.Read(rctx)
		cancel()
		var ce websocket.CloseError
		if errors.As(err, &ce) {
			return ce.Code
		}
		if err != nil {
			t.Fatalf("waiting for a close: %v", err)
		}
	}
}
