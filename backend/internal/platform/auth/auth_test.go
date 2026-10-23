package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
)

const testURL = "https://example.supabase.co"

func newTestVerifier(t *testing.T) (*Verifier, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	kf := func(*jwt.Token) (any, error) { return &key.PublicKey, nil }
	return NewVerifierWithKeyfunc(kf, testURL), key
}

func sign(t *testing.T, key *ecdsa.PrivateKey, c jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodES256, c).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerify(t *testing.T) {
	v, key := newTestVerifier(t)
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	id := uuid.New()
	valid := func() jwt.MapClaims {
		return jwt.MapClaims{
			"sub": id.String(), "email": "a@example.com", "aud": "authenticated",
			"iss": testURL + "/auth/v1", "exp": time.Now().Add(time.Hour).Unix(),
		}
	}

	tests := []struct {
		name    string
		token   func() string
		wantErr bool
	}{
		{"valid", func() string { return sign(t, key, valid()) }, false},
		{"expired", func() string { c := valid(); c["exp"] = time.Now().Add(-time.Minute).Unix(); return sign(t, key, c) }, true},
		{"missing exp", func() string { c := valid(); delete(c, "exp"); return sign(t, key, c) }, true},
		{"wrong issuer", func() string { c := valid(); c["iss"] = "https://evil/auth/v1"; return sign(t, key, c) }, true},
		{"wrong audience", func() string { c := valid(); c["aud"] = "anon"; return sign(t, key, c) }, true},
		{"bad subject", func() string { c := valid(); c["sub"] = "not-a-uuid"; return sign(t, key, c) }, true},
		{"wrong key", func() string { return sign(t, other, valid()) }, true},
		{"HS256 rejected", func() string {
			s, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, valid()).SignedString([]byte("secret"))
			return s
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := v.Verify(tt.token())
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (c.UserID != id || c.Email != "a@example.com") {
				t.Fatalf("caller = %+v", c)
			}
		})
	}
}

func TestVerifyAdmin(t *testing.T) {
	v, key := newTestVerifier(t)
	claims := func(extra jwt.MapClaims) jwt.MapClaims {
		c := jwt.MapClaims{
			"sub": uuid.NewString(), "aud": "authenticated",
			"iss": testURL + "/auth/v1", "exp": time.Now().Add(time.Hour).Unix(),
		}
		for k, val := range extra {
			c[k] = val
		}
		return c
	}
	for _, tt := range []struct {
		name  string
		extra jwt.MapClaims
		want  bool
	}{
		{"app_metadata.admin", jwt.MapClaims{"app_metadata": map[string]any{"provider": "email", "admin": true}}, true},
		{"no app_metadata", nil, false},
		{"admin false", jwt.MapClaims{"app_metadata": map[string]any{"admin": false}}, false},
		// Users can edit their own user_metadata: it must never grant admin.
		{"user_metadata ignored", jwt.MapClaims{"user_metadata": map[string]any{"admin": true}}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, err := v.Verify(sign(t, key, claims(tt.extra)))
			if err != nil {
				t.Fatal(err)
			}
			if c.Admin != tt.want {
				t.Fatalf("Admin = %v, want %v", c.Admin, tt.want)
			}
		})
	}
}

func TestAdminOnly(t *testing.T) {
	// A real Connect server with unimplemented handlers: a call the
	// interceptor lets through comes back UNIMPLEMENTED.
	var caller *Caller
	setCaller := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if caller != nil {
				ctx = WithCaller(ctx, *caller)
			}
			return next(ctx, req)
		}
	})
	opts := connect.WithInterceptors(setCaller, AdminOnly(isshov1connect.ModerationServiceName))
	mux := http.NewServeMux()
	mux.Handle(isshov1connect.NewModerationServiceHandler(isshov1connect.UnimplementedModerationServiceHandler{}, opts))
	mux.Handle(isshov1connect.NewPushServiceHandler(isshov1connect.UnimplementedPushServiceHandler{}, opts))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	moderation := isshov1connect.NewModerationServiceClient(srv.Client(), srv.URL)
	push := isshov1connect.NewPushServiceClient(srv.Client(), srv.URL)

	user, admin := &Caller{UserID: uuid.New()}, &Caller{UserID: uuid.New(), Admin: true}
	for _, tt := range []struct {
		name   string
		caller *Caller
		call   func() error
		want   connect.Code
	}{
		{"admin", admin, func() error {
			_, err := moderation.ListReports(context.Background(), connect.NewRequest(&isshov1.ListReportsRequest{}))
			return err
		}, connect.CodeUnimplemented},
		{"user", user, func() error {
			_, err := moderation.ListReports(context.Background(), connect.NewRequest(&isshov1.ListReportsRequest{}))
			return err
		}, connect.CodePermissionDenied},
		{"no caller", nil, func() error {
			_, err := moderation.GetModerator(context.Background(), connect.NewRequest(&isshov1.GetModeratorRequest{}))
			return err
		}, connect.CodePermissionDenied},
		{"other service", user, func() error {
			_, err := push.RegisterPushToken(context.Background(), connect.NewRequest(&isshov1.RegisterPushTokenRequest{}))
			return err
		}, connect.CodeUnimplemented},
	} {
		t.Run(tt.name, func(t *testing.T) {
			caller = tt.caller
			if got := connect.CodeOf(tt.call()); got != tt.want {
				t.Fatalf("code = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContextRoundTrip(t *testing.T) {
	want := Caller{UserID: uuid.New()}
	got, ok := FromContext(WithCaller(context.Background(), want))
	if !ok || got != want {
		t.Fatalf("got %+v, %v", got, ok)
	}
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("expected no caller")
	}
}
