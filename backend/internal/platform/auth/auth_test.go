package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
