package chatws

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/chat"
	"github.com/shrujancheruku/issho/backend/internal/crews"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// TestIntegrationSocketSuspendedOnNextSend: a socket opened before its person
// was suspended is closed with 4403 by their next send (a typing notice), with
// nothing pushed from the hub in between.
func TestIntegrationSocketSuspendedOnNextSend(t *testing.T) {
	url := testfixture.DatabaseURL(t)
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
	token, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": going.String(), "aud": "authenticated", "iss": testIssuer + "/auth/v1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	var suspended atomic.Bool
	hub := NewHub()
	go hub.Run(ctx, pool, log)
	srv := httptest.NewServer(&Handler{
		Hub: hub, Verifier: verifier, Log: log,
		Chat: &chat.Service{DB: pool, Event: plans.ChatAccess{}, Crew: crews.ChatAccess{}},
		Suspended: func(context.Context, uuid.UUID) (bool, error) {
			return suspended.Load(), nil
		},
	})
	defer srv.Close()

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	hello, _ := json.Marshal(map[string]any{"token": token, "room": map[string]any{"eventId": eventID.String()}})
	if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
		t.Fatal(err)
	}
	if f := readFrame(t, ctx, conn); f.Type != "ready" {
		t.Fatalf("first frame = %+v", f)
	}

	suspended.Store(true)
	typing, _ := json.Marshal(map[string]string{"type": "typing"})
	if err := conn.Write(ctx, websocket.MessageText, typing); err != nil {
		t.Fatal(err)
	}
	if code := closeCode(t, ctx, conn); code != closeForbidden {
		t.Errorf("send after being suspended: closed with %d, want %d", code, closeForbidden)
	}
}
