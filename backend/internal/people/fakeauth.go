package people

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/shrujancheruku/issho/backend/internal/clock"
)

// FakeIssuer stands in for a project's Supabase Auth in tests: it holds an
// ES256 key, serves its public half as a JWKS and mints access tokens that
// Verifier accepts. It is the one fake every test that needs a signed-in
// caller uses, so none writes its own signing code.
//
// Each FakeIssuer has a key of its own, so a token from one is a token "signed
// with another key" to a Verifier built from another.
type FakeIssuer struct {
	key *ecdsa.PrivateKey
}

// FakeIssuerURL is the issuer, and the base of the key set's address, that a
// FakeIssuer's tokens and Verifier use.
const FakeIssuerURL = "https://fake.supabase.test/auth/v1"

const fakeKeyID = "fake-key-1"

// NewFakeIssuer makes a new key.
func NewFakeIssuer() *FakeIssuer {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err) // the system's randomness is gone
	}
	return &FakeIssuer{key: key}
}

// Verifier returns a Verifier that trusts this issuer's key. The key set is
// served in-process, so there is no network and nothing to close.
func (f *FakeIssuer) Verifier(clk clock.Clock) *Verifier {
	return NewVerifier(clk, VerifierConfig{
		JWKSURL:    FakeIssuerURL + "/.well-known/jwks.json",
		Issuer:     FakeIssuerURL,
		HTTPClient: &http.Client{Transport: f},
	})
}

// RoundTrip serves the key set to the Verifier's client.
func (f *FakeIssuer) RoundTrip(*http.Request) (*http.Response, error) {
	pad := func(b []byte) []byte { return append(make([]byte, 32-len(b)), b...) }
	set, _ := json.Marshal(map[string]any{"keys": []map[string]any{{
		"kty": "EC", "crv": "P-256", "alg": "ES256", "use": "sig", "kid": fakeKeyID,
		"x": b64(pad(f.key.X.Bytes())), "y": b64(pad(f.key.Y.Bytes())),
	}}})
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(set)),
	}, nil
}

// Token is an access token for the account with this id and email, issued at
// now and good for an hour, as Supabase Auth issues them after an email code.
func (f *FakeIssuer) Token(now time.Time, sub, email string) string {
	return f.sign(now, sub, email, "aal1")
}

// SecondStepToken is the same for a person who took the second sign-in step.
func (f *FakeIssuer) SecondStepToken(now time.Time, sub, email string) string {
	return f.sign(now, sub, email, "aal2")
}

func (f *FakeIssuer) sign(now time.Time, sub, email, aal string) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": FakeIssuerURL, "aud": audience, "role": audience, "sub": sub, "email": email, "aal": aal,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = fakeKeyID
	out, err := tok.SignedString(f.key)
	if err != nil {
		panic(err)
	}
	return out
}
