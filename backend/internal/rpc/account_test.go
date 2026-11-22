package rpc_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/fixtures"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

var accountStart = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

const (
	ada   = "11111111-1111-4111-8111-111111111111"
	grace = "22222222-2222-4222-8222-222222222222"
)

// accountRig is the API with real accounts on a real database, and a fake
// Supabase Auth whose tokens it verifies.
type accountRig struct {
	pool   *pgxpool.Pool
	clk    *clock.Fake
	issuer *people.FakeIssuer
	people *people.Service
	// forming forms the outings the OutingService lists.
	forming *outings.Service
	srv     *httptest.Server
	client  isshov1connect.AccountServiceClient
}

func newAccountRig(t *testing.T) *accountRig {
	t.Helper()
	pool := testdb.NewDatabase(t)
	if err := db.Up(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(accountStart)
	issuer := people.NewFakeIssuer()
	if err := cities.LoadFixture(context.Background(), pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	if err := plans.LoadFixture(context.Background(), pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	cs := cities.New(pool)
	svc := people.New(clk, pool, cs, people.Config{DefaultCity: "da-nang-hoi-an"})
	signups := plans.NewSignups(clk, pool, cs)
	outs := outings.New(clk, pool, signups, svc)
	srv := httptest.NewServer(rpc.NewHandler(rpc.Config{
		Version: "test",
		Auth:    rpc.NewAuthenticator(issuer.Verifier(clk)),
		People:  svc,
		Cities:  cs,
		Plans:   plans.New(clk, pool, cs),
		Signups: signups,
		Outings: outs,
	}))
	t.Cleanup(srv.Close)
	return &accountRig{pool: pool, clk: clk, issuer: issuer, people: svc, forming: outs, srv: srv, client: isshov1connect.NewAccountServiceClient(srv.Client(), srv.URL)}
}

// admit admits the account that signed in with the email, as the seed command
// does for the owner. Tests that sign someone up use it, because a signup needs
// admission; the rule itself is never relaxed.
func (r *accountRig) admit(t *testing.T, email string) {
	t.Helper()
	if err := people.AdmitByEmail(context.Background(), r.pool, email, r.clk.Now()); err != nil {
		t.Fatal(err)
	}
}

func (r *accountRig) get(token string, req *isshov1.GetAccountRequest) (*isshov1.Account, error) {
	creq := connect.NewRequest(req)
	if token != "" {
		creq.Header().Set("Authorization", "Bearer "+token)
	}
	res, err := r.client.GetAccount(context.Background(), creq)
	if err != nil {
		return nil, err
	}
	return res.Msg.GetAccount(), nil
}

func wantCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	if got := connect.CodeOf(err); err == nil || got != code {
		t.Fatalf("err = %v (code %v), want %v", err, got, code)
	}
}

// The first call with a valid token makes the account, with the token's email.
func TestFirstCallWithAValidTokenMakesTheAccount(t *testing.T) {
	r := newAccountRig(t)
	got, err := r.get(r.issuer.Token(r.clk.Now(), ada, "Ada@Example.com"), &isshov1.GetAccountRequest{City: "da-nang-hoi-an", DisplayLanguage: "vi"})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetId() != ada || got.GetEmail() != "ada@example.com" || got.GetCity() != "da-nang-hoi-an" || got.GetDisplayLanguage() != "vi" {
		t.Fatalf("account = %v", got)
	}
	stored, err := r.people.Account(context.Background(), ada)
	if err != nil || stored.Email != "ada@example.com" {
		t.Fatalf("stored = %+v, %v", stored, err)
	}
}

func TestFirstCallWithNothingElseGetsTheDefaultCityAndEnglish(t *testing.T) {
	r := newAccountRig(t)
	got, err := r.get(r.issuer.Token(r.clk.Now(), ada, "ada@example.com"), &isshov1.GetAccountRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetCity() != "da-nang-hoi-an" || got.GetDisplayLanguage() != "en" {
		t.Fatalf("account = %v", got)
	}
}

// Only the listed city and "vi" or "en" make an account: anything else is
// InvalidArgument, and the refused call leaves no account behind.
func TestAnAccountThatIsNotValidIsInvalidArgument(t *testing.T) {
	r := newAccountRig(t)
	token := r.issuer.Token(r.clk.Now(), ada, "ada@example.com")
	for name, req := range map[string]*isshov1.GetAccountRequest{
		"language not listed": {DisplayLanguage: "fr"},
		"city not listed":     {City: "hanoi"},
		"city not a slug":     {City: "Da Nang"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := r.get(token, req)
			wantCode(t, err, connect.CodeInvalidArgument)
			if _, err := r.people.Account(context.Background(), ada); err == nil {
				t.Fatal("a refused call made the account")
			}
		})
	}
}

// A member RPC returns the caller's own account: each caller sees their own,
// and the request names no account to ask for another's.
func TestEachCallerGetsTheirOwnAccountAndNobodyElses(t *testing.T) {
	r := newAccountRig(t)
	adaTok := r.issuer.Token(r.clk.Now(), ada, "ada@example.com")
	graceTok := r.issuer.Token(r.clk.Now(), grace, "grace@example.com")
	for _, tok := range []string{adaTok, graceTok} {
		if _, err := r.get(tok, &isshov1.GetAccountRequest{}); err != nil {
			t.Fatal(err)
		}
	}
	for tok, want := range map[string]string{adaTok: "ada@example.com", graceTok: "grace@example.com"} {
		got, err := r.get(tok, &isshov1.GetAccountRequest{})
		if err != nil || got.GetEmail() != want {
			t.Fatalf("email = %q, %v; want %q", got.GetEmail(), err, want)
		}
	}
	fields := (&isshov1.GetAccountRequest{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		if n := string(fields.Get(i).Name()); strings.Contains(n, "id") || strings.Contains(n, "email") {
			t.Fatalf("GetAccountRequest has a field %q that could name another account", n)
		}
	}
}

func TestRefusesACallerWithoutAGoodToken(t *testing.T) {
	r := newAccountRig(t)
	stranger := people.NewFakeIssuer() // another key, same key id and issuer
	for name, tok := range map[string]string{
		"no token":    "",
		"garbage":     "forged",
		"another key": stranger.Token(r.clk.Now(), ada, "ada@example.com"),
		"expired":     r.issuer.Token(r.clk.Now().Add(-2*time.Hour), ada, "ada@example.com"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := r.get(tok, &isshov1.GetAccountRequest{})
			wantCode(t, err, connect.CodeUnauthenticated)
		})
	}
	if _, err := r.people.Account(context.Background(), ada); err == nil {
		t.Fatal("a refused call made an account")
	}
}

// A token that was good is refused once the fake clock passes its expiry.
func TestAnExpiredTokenIsRefused(t *testing.T) {
	r := newAccountRig(t)
	tok := r.issuer.Token(r.clk.Now(), ada, "ada@example.com")
	if _, err := r.get(tok, &isshov1.GetAccountRequest{}); err != nil {
		t.Fatal(err)
	}
	r.clk.Advance(time.Hour + time.Second)
	_, err := r.get(tok, &isshov1.GetAccountRequest{})
	wantCode(t, err, connect.CodeUnauthenticated)
}

// The API takes no account id and returns no one's email but the caller's.
// This walks every RPC the API serves: an email in any response, however
// deep, must be one of these, which return only the caller's own.
func TestNoRPCReturnsAnotherPersonsEmail(t *testing.T) {
	ownEmail := map[string]bool{"issho.v1.AccountService.GetAccount": true}
	var offenders []string
	walked := map[string]bool{}
	protoregistry.GlobalFiles.RangeFilesByPackage("issho.v1", func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Services().Len(); i++ {
			svc := fd.Services().Get(i)
			for j := 0; j < svc.Methods().Len(); j++ {
				m := svc.Methods().Get(j)
				walked[string(m.FullName())] = true
				if holdsEmail(m.Output(), map[protoreflect.FullName]bool{}) && !ownEmail[string(m.FullName())] {
					offenders = append(offenders, string(m.FullName()))
				}
			}
		}
		return true
	})
	if len(offenders) > 0 {
		t.Fatalf("RPCs whose response carries an email, and are not the caller's own: %v", offenders)
	}
	// The walk is over everything registered, so a new RPC is checked without
	// anyone listing it; these are the outing RPCs that show other people's
	// first names, which it must be seen to reach.
	for _, rpc := range []string{"issho.v1.OutingService.ListMyOutings", "issho.v1.OutingService.AnswerInvitation", "issho.v1.OutingService.CancelSeat"} {
		if !walked[rpc] {
			t.Errorf("the walk did not reach %s", rpc)
		}
	}
}

func holdsEmail(msg protoreflect.MessageDescriptor, seen map[protoreflect.FullName]bool) bool {
	if seen[msg.FullName()] {
		return false
	}
	seen[msg.FullName()] = true
	for i := 0; i < msg.Fields().Len(); i++ {
		f := msg.Fields().Get(i)
		if strings.Contains(strings.ToLower(string(f.Name())), "email") {
			return true
		}
		if f.Message() != nil && holdsEmail(f.Message(), seen) {
			return true
		}
	}
	return false
}

// The API serves accounts only when it has them to serve: with no people, as
// before, only the public RPCs exist.
func TestNoPeopleMeansNoAccountService(t *testing.T) {
	srv := httptest.NewServer(rpc.NewHandler(rpc.Config{Version: "test"}))
	t.Cleanup(srv.Close)
	client := isshov1connect.NewAccountServiceClient(srv.Client(), srv.URL)
	_, err := client.GetAccount(context.Background(), connect.NewRequest(&isshov1.GetAccountRequest{}))
	if err == nil {
		t.Fatal("GetAccount answered with no people configured")
	}
	if code := connect.CodeOf(err); code != connect.CodeUnimplemented && code != connect.CodeNotFound {
		t.Fatalf("code = %v", code)
	}
}
