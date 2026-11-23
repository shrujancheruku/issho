package rpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
)

type profileRig struct {
	*accountRig
	profiles isshov1connect.ProfileServiceClient
}

func newProfileRig(t *testing.T) *profileRig {
	t.Helper()
	r := newAccountRig(t)
	return &profileRig{accountRig: r, profiles: isshov1connect.NewProfileServiceClient(r.srv.Client(), r.srv.URL)}
}

// signedIn makes the account (the app's first call) and returns the token.
func (r *profileRig) signedIn(t *testing.T, id, email string) string {
	t.Helper()
	tok := r.issuer.Token(r.clk.Now(), id, email)
	if _, err := r.get(tok, &isshov1.GetAccountRequest{}); err != nil {
		t.Fatal(err)
	}
	return tok
}

func (r *profileRig) save(token string, req *isshov1.SaveProfileRequest) (*isshov1.Profile, error) {
	creq := connect.NewRequest(req)
	if token != "" {
		creq.Header().Set("Authorization", "Bearer "+token)
	}
	res, err := r.profiles.SaveProfile(context.Background(), creq)
	if err != nil {
		return nil, err
	}
	return res.Msg.GetProfile(), nil
}

func (r *profileRig) read(token string) (*isshov1.Profile, error) {
	creq := connect.NewRequest(&isshov1.GetProfileRequest{})
	if token != "" {
		creq.Header().Set("Authorization", "Bearer "+token)
	}
	res, err := r.profiles.GetProfile(context.Background(), creq)
	if err != nil {
		return nil, err
	}
	return res.Msg.GetProfile(), nil
}

func goodSave() *isshov1.SaveProfileRequest {
	return &isshov1.SaveProfileRequest{
		Name: "Ada", DateOfBirth: "1990-06-15", Gender: "woman", Languages: []string{"en", "vi"},
		LivesInCity: true, AcceptedCodeOfConduct: true,
	}
}

// problemOf is the ProfileProblem reason on a refusal, or unspecified.
func problemOf(err error) isshov1.ProfileProblemReason {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return isshov1.ProfileProblemReason_PROFILE_PROBLEM_REASON_UNSPECIFIED
	}
	for _, d := range ce.Details() {
		v, err := d.Value()
		if err != nil {
			continue
		}
		if p, ok := v.(*isshov1.ProfileProblem); ok {
			return p.GetReason()
		}
	}
	return isshov1.ProfileProblemReason_PROFILE_PROBLEM_REASON_UNSPECIFIED
}

// A caller with no profile reads none (the app then asks the questions); once
// saved it reads back as given, with no date of birth in it.
func TestSavingAProfileAndReadingItBack(t *testing.T) {
	r := newProfileRig(t)
	tok := r.signedIn(t, ada, "ada@example.com")
	if p, err := r.read(tok); err != nil || p != nil {
		t.Fatalf("before saving: %v, %v; want no profile", p, err)
	}
	saved, err := r.save(tok, goodSave())
	if err != nil {
		t.Fatal(err)
	}
	read, err := r.read(tok)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []*isshov1.Profile{saved, read} {
		if p.GetName() != "Ada" || p.GetGender() != "woman" || strings.Join(p.GetLanguages(), ",") != "en,vi" ||
			!p.GetLivesInCity() || p.GetOnlyWomenAndNonBinary() || p.GetCodeOfConductVersion() != 1 {
			t.Fatalf("profile = %v", p)
		}
	}
	if strings.Contains(read.String(), "1990") {
		t.Fatalf("the profile carries the date of birth: %v", read)
	}
}

// Under 18 on the day is INVALID_ARGUMENT with the reason the app acts on, and
// nothing is saved.
func TestUnderEighteenIsRefusedWithAReasonTheAppCanActOn(t *testing.T) {
	r := newProfileRig(t)
	tok := r.signedIn(t, ada, "ada@example.com")
	req := goodSave()
	req.DateOfBirth = "2010-03-01" // the fake clock is 2026-01-05
	_, err := r.save(tok, req)
	wantCode(t, err, connect.CodeInvalidArgument)
	if got := problemOf(err); got != isshov1.ProfileProblemReason_PROFILE_PROBLEM_REASON_UNDER_AGE {
		t.Fatalf("reason = %v, want UNDER_AGE", got)
	}
	if p, err := r.read(tok); err != nil || p != nil {
		t.Fatalf("a refused profile was saved: %v, %v", p, err)
	}
}

func TestOnlyWomenAndNonBinaryIsRefusedOverTheWireForAnyoneElse(t *testing.T) {
	r := newProfileRig(t)
	tok := r.signedIn(t, ada, "ada@example.com")
	req := goodSave()
	req.Gender = "man"
	req.OnlyWomenAndNonBinary = true
	_, err := r.save(tok, req)
	wantCode(t, err, connect.CodeInvalidArgument)
	if got := problemOf(err); got != isshov1.ProfileProblemReason_PROFILE_PROBLEM_REASON_WOMEN_ONLY_NOT_AVAILABLE {
		t.Fatalf("reason = %v, want WOMEN_ONLY_NOT_AVAILABLE", got)
	}
	req.Gender = "non-binary"
	if p, err := r.save(tok, req); err != nil || !p.GetOnlyWomenAndNonBinary() {
		t.Fatalf("a non-binary person: %v, %v; want the setting on", p, err)
	}
}

func TestAnInvalidProfileIsInvalidArgumentAndSavesNothing(t *testing.T) {
	r := newProfileRig(t)
	tok := r.signedIn(t, ada, "ada@example.com")
	for name, change := range map[string]func(*isshov1.SaveProfileRequest){
		"no name":        func(q *isshov1.SaveProfileRequest) { q.Name = "" },
		"long name":      func(q *isshov1.SaveProfileRequest) { q.Name = strings.Repeat("a", 41) },
		"no gender":      func(q *isshov1.SaveProfileRequest) { q.Gender = "" },
		"no languages":   func(q *isshov1.SaveProfileRequest) { q.Languages = nil },
		"no conduct":     func(q *isshov1.SaveProfileRequest) { q.AcceptedCodeOfConduct = false },
		"no birth date":  func(q *isshov1.SaveProfileRequest) { q.DateOfBirth = "" },
		"not a birthday": func(q *isshov1.SaveProfileRequest) { q.DateOfBirth = "last year" },
	} {
		t.Run(name, func(t *testing.T) {
			req := goodSave()
			change(req)
			_, err := r.save(tok, req)
			wantCode(t, err, connect.CodeInvalidArgument)
			if p, err := r.read(tok); err != nil || p != nil {
				t.Fatalf("a refused profile was saved: %v, %v", p, err)
			}
		})
	}
}

// A signup (any RPC that needs a profile) calls rpc.RequireProfile, and a
// caller with none gets FAILED_PRECONDITION with the reason that tells the app
// to ask the profile's questions.
func TestAnAccountWithNoProfileIsRefusedWithProfileRequired(t *testing.T) {
	r := newProfileRig(t)
	r.signedIn(t, ada, "ada@example.com")
	err := rpc.RequireProfile(context.Background(), r.people, ada)
	wantCode(t, err, connect.CodeFailedPrecondition)
	if got := problemOf(err); got != isshov1.ProfileProblemReason_PROFILE_PROBLEM_REASON_PROFILE_REQUIRED {
		t.Fatalf("reason = %v, want PROFILE_REQUIRED", got)
	}
	// The error crosses the wire with the detail intact: TestSigningUpWithNoProfile
	// IsRefusedWithProfileRequired (signup_test.go) calls it through the signup RPC.

	if _, err := r.save(r.issuer.Token(r.clk.Now(), ada, "ada@example.com"), goodSave()); err != nil {
		t.Fatal(err)
	}
	if err := rpc.RequireProfile(context.Background(), r.people, ada); err != nil {
		t.Fatalf("a person with a profile is refused: %v", err)
	}
}

func TestProfileCallsNeedAGoodToken(t *testing.T) {
	r := newProfileRig(t)
	r.signedIn(t, ada, "ada@example.com")
	_, err := r.save("", goodSave())
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = r.read("forged")
	wantCode(t, err, connect.CodeUnauthenticated)
}

// A caller with a token but no account yet is told to make it, not given an
// empty profile as if all were well.
func TestProfileCallsNeedAnAccountFirst(t *testing.T) {
	r := newProfileRig(t)
	tok := r.issuer.Token(r.clk.Now(), ada, "ada@example.com")
	_, err := r.save(tok, goodSave())
	wantCode(t, err, connect.CodeFailedPrecondition)
}

// Each caller reads and changes their own profile and never another's: the
// requests name no account.
func TestEachCallerHasTheirOwnProfile(t *testing.T) {
	r := newProfileRig(t)
	adaTok := r.signedIn(t, ada, "ada@example.com")
	graceTok := r.signedIn(t, grace, "grace@example.com")
	if _, err := r.save(adaTok, goodSave()); err != nil {
		t.Fatal(err)
	}
	if p, err := r.read(graceTok); err != nil || p != nil {
		t.Fatalf("grace read %v, %v; want no profile (ada's isn't hers)", p, err)
	}
	g := goodSave()
	g.Name = "Grace"
	if _, err := r.save(graceTok, g); err != nil {
		t.Fatal(err)
	}
	if p, _ := r.read(adaTok); p.GetName() != "Ada" {
		t.Fatalf("ada's profile = %v", p)
	}
	if p, _ := r.read(graceTok); p.GetName() != "Grace" {
		t.Fatalf("grace's profile = %v", p)
	}
}

// No RPC returns a profile except the caller's own, and no profile RPC takes a
// field that could name a person: the schema says so for every RPC, now and as
// later ones are added.
func TestNoRPCReturnsAnotherPersonsProfile(t *testing.T) {
	own := map[string]bool{
		"issho.v1.ProfileService.GetProfile":  true,
		"issho.v1.ProfileService.SaveProfile": true,
	}
	var offenders []string
	protoregistry.GlobalFiles.RangeFilesByPackage("issho.v1", func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Services().Len(); i++ {
			svc := fd.Services().Get(i)
			for j := 0; j < svc.Methods().Len(); j++ {
				m := svc.Methods().Get(j)
				if holdsProfile(m.Output(), map[protoreflect.FullName]bool{}) && !own[string(m.FullName())] {
					offenders = append(offenders, string(m.FullName()))
				}
				if own[string(m.FullName())] {
					for k := 0; k < m.Input().Fields().Len(); k++ {
						name := strings.ToLower(string(m.Input().Fields().Get(k).Name()))
						if strings.Contains(name, "id") || strings.Contains(name, "account") || strings.Contains(name, "email") {
							offenders = append(offenders, string(m.FullName())+" takes "+name)
						}
					}
				}
			}
		}
		return true
	})
	if len(offenders) > 0 {
		t.Fatalf("RPCs that return a profile and aren't the caller's own, or can name a person: %v", offenders)
	}
}

func holdsProfile(msg protoreflect.MessageDescriptor, seen map[protoreflect.FullName]bool) bool {
	if msg.FullName() == "issho.v1.Profile" {
		return true
	}
	if seen[msg.FullName()] {
		return false
	}
	seen[msg.FullName()] = true
	for i := 0; i < msg.Fields().Len(); i++ {
		if f := msg.Fields().Get(i); f.Message() != nil && holdsProfile(f.Message(), seen) {
			return true
		}
	}
	return false
}
