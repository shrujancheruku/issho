package people

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/shrujancheruku/issho/backend/internal/clock"
)

const (
	// audience is what Supabase Auth puts in an access token for a signed-in
	// person. The project's anon and service keys are JWTs too, with other roles.
	audience = "authenticated"

	// keyRefreshAfter is how old a fetched key set is before the next token
	// makes the API fetch it again, beside the request.
	keyRefreshAfter = time.Hour
	// KeySetMaxAge is the hard limit on how long a fetched key set is trusted,
	// counted from its last successful fetch. Past it no token verifies until
	// a fetch succeeds (fail closed), so a key Auth has revoked stops working
	// within a bound we can state even while the key-set host is unreachable.
	// Three hours is two failed hourly refreshes' worth of outage, retried
	// every refetchEvery, before sign-in stops; if Supabase Auth is down for
	// longer nobody can sign in anyway.
	KeySetMaxAge = 3 * time.Hour
	// refetchEvery is the least time between two fetches: a token naming a key
	// id the API hasn't seen (a rotation, or a forgery) can't make it fetch on
	// every request.
	refetchEvery = time.Minute
	// fetchTimeout bounds one fetch of the key set.
	fetchTimeout = 10 * time.Second
	// maxJWKSBytes bounds what a fetch reads; a project's set is a few hundred bytes.
	maxJWKSBytes = 1 << 20

	// What a published RSA key must be to be used: a public exponent no
	// smaller than 3 that fits an int, on a modulus of at least 2048 bits.
	minRSAExponent = 3 //policylint:ignore an RSA exponent bound, not a group size
	maxRSAExponent = 1<<31 - 1
	minRSABits     = 2048
)

// VerifierConfig says where a project's keys are and who issues its tokens.
type VerifierConfig struct {
	// JWKSURL is the project's public key set, e.g.
	// https://<ref>.supabase.co/auth/v1/.well-known/jwks.json. The keys are
	// public: the API never holds the project's JWT secret.
	JWKSURL string
	// Issuer is the `iss` of the project's tokens, e.g. https://<ref>.supabase.co/auth/v1.
	Issuer string
	// HTTPClient fetches the key set; nil uses a client with a 10-second timeout.
	HTTPClient *http.Client
	// Log says when a token is refused because the key set is past
	// KeySetMaxAge; nil uses slog.Default(). It never gets a token.
	Log *slog.Logger
}

// Verifier checks the access tokens Supabase Auth issues. It accepts ES256 and
// RS256 signatures, from a key in the project's JWKS, and nothing else: not
// HS256, which the API would need the JWT secret for, and not "none".
type Verifier struct {
	clock  clock.Clock
	cfg    VerifierConfig
	client *http.Client

	mu        sync.Mutex
	keys      map[string]crypto.PublicKey
	fetchedAt time.Time // when keys was last replaced
	triedAt   time.Time // when a fetch was last started, good or not
	flight    *flight   // the fetch in progress, if any
}

// NewVerifier returns a Verifier that reads the time from clk, so a test can
// expire a token by moving the fake clock.
func NewVerifier(clk clock.Clock, cfg VerifierConfig) *Verifier {
	if cfg.JWKSURL == "" || cfg.Issuer == "" {
		panic("people: VerifierConfig needs a JWKSURL and an Issuer")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &Verifier{clock: clk, cfg: cfg, client: client}
}

// Verify returns who a token says the caller is, or an error that says nothing
// a caller may act on: the token is refused. It checks the signature, the
// expiry and not-before (against the injected clock), the issuer, the
// audience, and that the token is a signed-in person's.
func (v *Verifier) Verify(ctx context.Context, raw string) (Claims, error) {
	var mc jwt.MapClaims
	_, err := jwt.ParseWithClaims(raw, &mc, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token names no key")
		}
		return v.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"ES256", "RS256"}),
		jwt.WithTimeFunc(v.clock.Now),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(v.cfg.Issuer),
		jwt.WithAudience(audience),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("people: token refused: %w", err)
	}
	if role, _ := mc["role"].(string); role != audience {
		return Claims{}, errors.New("people: token refused: not a signed-in person's")
	}
	sub, _ := mc["sub"].(string)
	if sub == "" {
		return Claims{}, errors.New("people: token refused: no subject")
	}
	email, _ := mc["email"].(string)
	aal, _ := mc["aal"].(string)
	return Claims{Subject: sub, Email: email, SecondStep: aal == "aal2"}, nil
}

// flight is one fetch of the key set in progress; every caller that needs the
// result waits on done.
type flight struct {
	done chan struct{}
	err  error // set before done is closed
}

// key returns the public key with the given id.
//
// A key the API already has is returned at once, even when the set is due for
// its hourly refresh: the refresh runs beside the request, so a slow or dead
// key-set host never holds up a signed-in person whose key is known (a key
// Auth has revoked is refused from the moment the new set arrives). That
// holds only while the set is younger than KeySetMaxAge: past it every key
// counts as unknown, so a caller waits for a fetch and is refused if it
// fails. A key id the API hasn't seen waits for a fetch, subject to
// refetchEvery; callers that need a fetch while one is in flight share it.
// The lock is never held while the fetch runs.
func (v *Verifier) key(ctx context.Context, kid string) (crypto.PublicKey, error) {
	v.mu.Lock()
	now := v.clock.Now()
	k, known := v.keys[kid]
	age := now.Sub(v.fetchedAt)
	// A set that was never fetched isn't "past its age": it is just absent.
	expired := !v.fetchedAt.IsZero() && age >= KeySetMaxAge
	known = known && !expired
	stale := age >= keyRefreshAfter
	fl := v.flight
	if fl == nil && (!known || stale) && now.Sub(v.triedAt) >= refetchEvery {
		fl = &flight{done: make(chan struct{})}
		v.flight, v.triedAt = fl, now
		// The fetch isn't the caller's: its result serves everyone waiting, so
		// it outlives the caller's cancellation and has its own deadline.
		go v.refresh(context.WithoutCancel(ctx), fl, now)
	}
	v.mu.Unlock()

	// refuse is every way out but a key: the refusal of an expired set is
	// logged here, once the wait for a fetch has failed. The log gets neither
	// the token nor its key id, both of which are the caller's to choose.
	refuse := func(err error) (crypto.PublicKey, error) {
		if expired {
			v.cfg.Log.Warn("token refused: the key set is past its maximum age and could not be refreshed",
				"age", age.Round(time.Second), "max_age", KeySetMaxAge)
		}
		return nil, err
	}
	if known {
		return k, nil
	}
	if fl == nil {
		return refuse(errors.New("unknown key id"))
	}
	select {
	case <-fl.done:
	case <-ctx.Done():
		return refuse(ctx.Err())
	}
	if fl.err != nil {
		return refuse(fmt.Errorf("fetch keys: %w", fl.err))
	}
	v.mu.Lock()
	k, known = v.keys[kid]
	v.mu.Unlock()
	if !known {
		// The fetch succeeded, so the set is fresh: this is just an unknown key.
		return nil, errors.New("unknown key id")
	}
	return k, nil
}

// refresh fetches the key set and, if it arrives, replaces the one in use. A
// failed fetch leaves the last good set working.
func (v *Verifier) refresh(ctx context.Context, fl *flight, now time.Time) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	keys, err := v.fetch(ctx)
	v.mu.Lock()
	if err == nil {
		v.keys, v.fetchedAt = keys, now
	}
	v.flight = nil
	v.mu.Unlock()
	fl.err = err
	close(fl.done)
}

func (v *Verifier) fetch(ctx context.Context) (map[string]crypto.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.JWKSURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSBytes))
	if err != nil {
		return nil, err
	}
	return parseJWKS(body)
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// parseJWKS reads the EC P-256 and RSA signing keys of a key set. Keys of other
// kinds, or meant for encryption, are left out: no token can name them.
func parseJWKS(body []byte) (map[string]crypto.PublicKey, error) {
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("key set: %w", err)
	}
	keys := map[string]crypto.PublicKey{}
	for _, k := range set.Keys {
		if k.Kid == "" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		switch k.Kty {
		case "EC":
			if k.Crv != "P-256" {
				continue
			}
			x, y := b64Int(k.X), b64Int(k.Y)
			if x == nil || y == nil || !elliptic.P256().IsOnCurve(x, y) {
				continue
			}
			keys[k.Kid] = &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
		case "RSA":
			n, e := b64Int(k.N), b64Int(k.E)
			if n == nil || e == nil || !e.IsInt64() || e.Int64() < minRSAExponent || e.Int64() > maxRSAExponent || n.BitLen() < minRSABits {
				continue
			}
			keys[k.Kid] = &rsa.PublicKey{N: n, E: int(e.Int64())}
		}
	}
	return keys, nil
}

func b64Int(s string) *big.Int {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) == 0 {
		return nil
	}
	return new(big.Int).SetBytes(b)
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
