package rpc_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
)

type outingRig struct {
	*signupRig
	outings isshov1connect.OutingServiceClient
}

func newOutingRig(t *testing.T) *outingRig {
	t.Helper()
	r := newSignupRig(t)
	return &outingRig{signupRig: r, outings: isshov1connect.NewOutingServiceClient(r.srv.Client(), r.srv.URL)}
}

// named is a signed-in account with a profile under its own name.
func (r *outingRig) named(t *testing.T, id, email, name string) string {
	t.Helper()
	tok := r.signedIn(t, id, email)
	req := goodSave()
	req.Name = name
	if _, err := r.save(tok, req); err != nil {
		t.Fatal(err)
	}
	return tok
}

func (r *outingRig) list(token string) (*isshov1.ListMyOutingsResponse, error) {
	res, err := r.outings.ListMyOutings(context.Background(), authed(token, &isshov1.ListMyOutingsRequest{}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

const (
	third  = "33333333-3333-4333-8333-333333333333"
	fourth = "44444444-4444-4444-8444-444444444444"
)

// Three people sign up to the lantern plan for one evening and matching forms
// their group.
func (r *outingRig) formedGroup(t *testing.T) (adaTok, graceTok, thirdTok string) {
	t.Helper()
	adaTok = r.named(t, ada, "ada@example.com", "Ada Lovelace")
	graceTok = r.named(t, grace, "grace@example.com", "Grace Hopper")
	thirdTok = r.named(t, third, "third@example.com", "Katherine Johnson")
	for _, tok := range []string{adaTok, graceTok, thirdTok} {
		if _, err := r.signUp(tok, lanterns, tick("2026-01-09", "evening")); err != nil {
			t.Fatal(err)
		}
	}
	if formed, err := r.forming.Form(context.Background(), lanterns); err != nil || formed != 1 {
		t.Fatalf("Form = %d, %v; want 1 outing", formed, err)
	}
	return adaTok, graceTok, thirdTok
}

// An invited person sees the date, the time and the languages the group shares,
// when the invitation went out and when its 48 hours end, and never a name.
func TestAnInvitedPersonSeesTheDateTheTimeAndTheSharedLanguagesNeverNames(t *testing.T) {
	r := newOutingRig(t)
	adaTok, _, _ := r.formedGroup(t)

	got, err := r.list(adaTok)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.GetOutings()) != 1 {
		t.Fatalf("outings = %v, want 1", got.GetOutings())
	}
	o := got.GetOutings()[0]
	sent := accountStart
	if o.GetPlanId() != lanterns || o.GetDate() != "2026-01-09" || o.GetPart() != "evening" || o.GetStartsAt() != "17:00" ||
		strings.Join(o.GetLanguages(), ",") != "en,vi" || o.GetSeatState() != isshov1.SeatState_SEAT_STATE_INVITED ||
		!o.GetInvitedAt().AsTime().Equal(sent) || !o.GetInvitationEndsAt().AsTime().Equal(sent.Add(48*time.Hour)) {
		t.Fatalf("outing = %v", o)
	}
	// Nothing of anyone else is in what the wire carries: no name, no account.
	wire, err := protojson.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []string{"Grace", "Hopper", "Katherine", "Johnson", grace, third} {
		if strings.Contains(string(wire), other) {
			t.Fatalf("the response names %q: %s", other, wire)
		}
	}
}

// No RPC returns an outing to someone with no seat in it: a member who isn't in
// the group lists none, and nobody can name an outing to ask for it (the
// request has no field for one).
func TestAMemberWithNoSeatInAnOutingIsNeverReturnedIt(t *testing.T) {
	r := newOutingRig(t)
	r.formedGroup(t)
	outsider := r.named(t, fourth, "fourth@example.com", "Mary Jackson")

	got, err := r.list(outsider)

	if err != nil || len(got.GetOutings()) != 0 {
		t.Fatalf("an outsider lists %v, %v; want none", got.GetOutings(), err)
	}
}

// Listing outings needs a signed-in caller.
func TestTheOutingRPCNeedsASignedInCaller(t *testing.T) {
	r := newOutingRig(t)
	_, err := r.list("")
	wantCode(t, err, connect.CodeUnauthenticated)
}
