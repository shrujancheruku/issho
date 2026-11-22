package people_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/people"
)

const issuer = "https://project.example/auth/v1"

// signer is a signing key as Supabase Auth would hold it, and the JWK that
// publishes its public half.
type signer struct {
	kid string
	alg string
	key crypto.Signer
}

func newES256(t *testing.T, kid string) signer {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return signer{kid: kid, alg: "ES256", key: k}
}

func newRS256(t *testing.T, kid string) signer {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return signer{kid: kid, alg: "RS256", key: k}
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (s signer) jwk() map[string]any {
	switch k := s.key.(type) {
	case *ecdsa.PrivateKey:
		return map[string]any{"kty": "EC", "crv": "P-256", "alg": s.alg, "use": "sig", "kid": s.kid,
			"x": b64(k.X.FillBytes(make([]byte, 32))), "y": b64(k.Y.FillBytes(make([]byte, 32)))}
	case *rsa.PrivateKey:
		return map[string]any{"kty": "RSA", "alg": s.alg, "use": "sig", "kid": s.kid,
			"n": b64(k.N.Bytes()), "e": b64(big.NewInt(int64(k.E)).Bytes())}
	}
	panic("unknown key")
}

// claims builds the claims of a good Supabase access token at now.
func claims(now time.Time, sub, email string) jwt.MapClaims {
	return jwt.MapClaims{
		"iss": issuer, "aud": "authenticated", "sub": sub, "email": email, "role": "authenticated",
		"aal": "aal1", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
}

func (s signer) token(t *testing.T, c jwt.MapClaims) string {
	t.Helper()
	var method jwt.SigningMethod = jwt.SigningMethodES256
	if s.alg == "RS256" {
		method = jwt.SigningMethodRS256
	}
	tok := jwt.NewWithClaims(method, c)
	tok.Header["kid"] = s.kid
	out, err := tok.SignedString(s.key)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// jwks serves a project's JWKS and counts the fetches.
type jwks struct {
	mu      sync.Mutex
	keys    []signer
	fetches atomic.Int64
	down    atomic.Bool
	srv     *httptest.Server
}

func newJWKS(t *testing.T, keys ...signer) *jwks {
	t.Helper()
	j := &jwks{keys: keys}
	j.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		j.fetches.Add(1)
		if j.down.Load() {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		j.mu.Lock()
		defer j.mu.Unlock()
		set := []map[string]any{}
		for _, k := range j.keys {
			set = append(set, k.jwk())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": set})
	}))
	t.Cleanup(j.srv.Close)
	return j
}

func (j *jwks) set(keys ...signer) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.keys = keys
}

func (j *jwks) verifier(clk clock.Clock) *people.Verifier {
	return people.NewVerifier(clk, people.VerifierConfig{JWKSURL: j.srv.URL, Issuer: issuer})
}

func TestVerifiesES256AndRS256Tokens(t *testing.T) {
	clk := clock.NewFake(start)
	ec, rs := newES256(t, "ec-1"), newRS256(t, "rs-1")
	v := newJWKS(t, ec, rs).verifier(clk)
	for _, s := range []signer{ec, rs} {
		t.Run(s.alg, func(t *testing.T) {
			got, err := v.Verify(context.Background(), s.token(t, claims(start, ada, "ada@example.com")))
			if err != nil {
				t.Fatal(err)
			}
			want := people.Claims{Subject: ada, Email: "ada@example.com"}
			if got != want {
				t.Fatalf("claims = %+v, want %+v", got, want)
			}
		})
	}
}

func TestSecondStepIsTheTokensAal2(t *testing.T) {
	clk := clock.NewFake(start)
	ec := newES256(t, "ec-1")
	v := newJWKS(t, ec).verifier(clk)
	c := claims(start, ada, "ada@example.com")
	c["aal"] = "aal2"
	got, err := v.Verify(context.Background(), ec.token(t, c))
	if err != nil || !got.SecondStep {
		t.Fatalf("claims = %+v, %v; want SecondStep", got, err)
	}
}

func TestRefusesATokenSignedWithAnotherKey(t *testing.T) {
	clk := clock.NewFake(start)
	real := newES256(t, "ec-1")
	v := newJWKS(t, real).verifier(clk)
	// The attacker's key claims the real key's id, and one of its own.
	forgedSameKid := newES256(t, "ec-1")
	forgedOtherKid := newES256(t, "ec-2")
	forgedRSA := newRS256(t, "ec-1")
	for name, s := range map[string]signer{"same kid": forgedSameKid, "unknown kid": forgedOtherKid, "other algorithm, same kid": forgedRSA} {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(context.Background(), s.token(t, claims(start, ada, "ada@example.com"))); err == nil {
				t.Fatal("token signed with another key was accepted")
			}
		})
	}
}

func TestRefusesAnExpiredTokenOnTheFakeClock(t *testing.T) {
	clk := clock.NewFake(start)
	ec := newES256(t, "ec-1")
	v := newJWKS(t, ec).verifier(clk)
	tok := ec.token(t, claims(start, ada, "ada@example.com")) // expires an hour after start
	if _, err := v.Verify(context.Background(), tok); err != nil {
		t.Fatalf("fresh token refused: %v", err)
	}
	clk.Advance(time.Hour + time.Second)
	if _, err := v.Verify(context.Background(), tok); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestRefusesATokenNotYetValid(t *testing.T) {
	clk := clock.NewFake(start)
	ec := newES256(t, "ec-1")
	v := newJWKS(t, ec).verifier(clk)
	c := claims(start, ada, "ada@example.com")
	c["nbf"] = start.Add(time.Minute).Unix()
	if _, err := v.Verify(context.Background(), ec.token(t, c)); err == nil {
		t.Fatal("token before its nbf was accepted")
	}
}

// The token is for this project's signed-in people: not another project's, not
// the anon key's, and not one with no subject.
func TestRefusesATokenNotForSignedInPeopleOfThisProject(t *testing.T) {
	clk := clock.NewFake(start)
	ec := newES256(t, "ec-1")
	v := newJWKS(t, ec).verifier(clk)
	for name, edit := range map[string]func(jwt.MapClaims){
		"other issuer":   func(c jwt.MapClaims) { c["iss"] = "https://other.example/auth/v1" },
		"other audience": func(c jwt.MapClaims) { c["aud"] = "something-else" },
		"anon role":      func(c jwt.MapClaims) { c["role"] = "anon" },
		"no subject":     func(c jwt.MapClaims) { delete(c, "sub") },
		"no expiry":      func(c jwt.MapClaims) { delete(c, "exp") },
	} {
		t.Run(name, func(t *testing.T) {
			c := claims(start, ada, "ada@example.com")
			edit(c)
			if _, err := v.Verify(context.Background(), ec.token(t, c)); err == nil {
				t.Fatal("token was accepted")
			}
		})
	}
}

// Only ES256 and RS256 are accepted: a token that names HS256 (signed with the
// public key's bytes as the secret, the classic confusion attack) or "none" is
// refused even though its claims are good.
func TestRefusesOtherAlgorithms(t *testing.T) {
	clk := clock.NewFake(start)
	ec := newES256(t, "ec-1")
	v := newJWKS(t, ec).verifier(clk)

	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, claims(start, ada, "ada@example.com"))
	hs.Header["kid"] = "ec-1"
	hsTok, err := hs.SignedString([]byte("a shared secret"))
	if err != nil {
		t.Fatal(err)
	}
	none := jwt.NewWithClaims(jwt.SigningMethodNone, claims(start, ada, "ada@example.com"))
	none.Header["kid"] = "ec-1"
	noneTok, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	for name, tok := range map[string]string{"HS256": hsTok, "none": noneTok, "garbage": "not.a.token", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(context.Background(), tok); err == nil {
				t.Fatal("token was accepted")
			}
		})
	}
}

// Auth rotates its signing keys: a token with a key id the API hasn't seen
// makes it fetch the set again, once a minute at most, so a stream of
// forged ids can't make it fetch on every request.
func TestKeyRotationAndTheFetchLimit(t *testing.T) {
	clk := clock.NewFake(start)
	old, next := newES256(t, "ec-1"), newES256(t, "ec-2")
	j := newJWKS(t, old)
	v := j.verifier(clk)
	ctx := context.Background()

	if _, err := v.Verify(ctx, old.token(t, claims(start, ada, "ada@example.com"))); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(ctx, old.token(t, claims(start, ada, "ada@example.com"))); err != nil {
		t.Fatal(err)
	}
	if n := j.fetches.Load(); n != 1 {
		t.Fatalf("fetches after two good tokens = %d, want 1 (the set is cached)", n)
	}

	// Auth rotates; the new key's token arrives within the minute: refused, no refetch.
	j.set(old, next)
	clk.Advance(10 * time.Second)
	fresh := next.token(t, claims(clk.Now(), ada, "ada@example.com"))
	if _, err := v.Verify(ctx, fresh); err == nil {
		t.Fatal("unknown key id accepted before the set could be refetched")
	}
	if n := j.fetches.Load(); n != 1 {
		t.Fatalf("fetches = %d, want still 1", n)
	}
	// After the minute it refetches and accepts.
	clk.Advance(time.Minute)
	if _, err := v.Verify(ctx, fresh); err != nil {
		t.Fatalf("rotated key refused after refetch: %v", err)
	}
	if n := j.fetches.Load(); n != 2 {
		t.Fatalf("fetches = %d, want 2", n)
	}
}

func TestARetiredKeyStopsWorkingOnceTheSetIsRefreshed(t *testing.T) {
	clk := clock.NewFake(start)
	old, next := newES256(t, "ec-1"), newES256(t, "ec-2")
	j := newJWKS(t, old)
	v := j.verifier(clk)
	ctx := context.Background()
	tok := old.token(t, claims(start, ada, "ada@example.com"))
	if _, err := v.Verify(ctx, tok); err != nil {
		t.Fatal(err)
	}
	j.set(next) // the old key is revoked
	clk.Advance(2 * time.Hour)
	// The token has expired by now too; use a fresh one from the revoked key.
	tok = old.token(t, claims(clk.Now(), ada, "ada@example.com"))
	if _, err := v.Verify(ctx, tok); err == nil {
		t.Fatal("token from a retired key was accepted")
	}
}

func TestJWKSDownMeansRefusedUntilItIsUpAndKnownKeysKeepWorking(t *testing.T) {
	clk := clock.NewFake(start)
	ec := newES256(t, "ec-1")
	j := newJWKS(t, ec)
	j.down.Store(true)
	v := j.verifier(clk)
	ctx := context.Background()
	tok := ec.token(t, claims(start, ada, "ada@example.com"))
	if _, err := v.Verify(ctx, tok); err == nil {
		t.Fatal("token accepted with no key set")
	}
	j.down.Store(false)
	clk.Advance(time.Minute)
	if _, err := v.Verify(ctx, tok); err != nil {
		t.Fatalf("refused after the set came up: %v", err)
	}
	// It goes down again after the cache has aged out: the last good set is used.
	j.down.Store(true)
	clk.Advance(2 * time.Hour)
	tok = ec.token(t, claims(clk.Now(), ada, "ada@example.com"))
	if _, err := v.Verify(ctx, tok); err != nil {
		t.Fatalf("known key refused while Auth is unreachable: %v", err)
	}
}
