package rpc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
)

type joinRig struct {
	*signupRig
	links isshov1connect.JoinLinkServiceClient
}

func newJoinRig(t *testing.T) *joinRig {
	t.Helper()
	r := newSignupRig(t)
	return &joinRig{signupRig: r, links: isshov1connect.NewJoinLinkServiceClient(r.srv.Client(), r.srv.URL)}
}

func (r *joinRig) create(token string) (*isshov1.JoinLink, error) {
	res, err := r.links.CreateJoinLink(context.Background(), authed(token, &isshov1.CreateJoinLinkRequest{}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetLink(), nil
}

func (r *joinRig) cancel(token, id string) error {
	_, err := r.links.CancelJoinLink(context.Background(), authed(token, &isshov1.CancelJoinLinkRequest{Id: id}))
	return err
}

func (r *joinRig) list(token string) (*isshov1.ListMyJoinLinksResponse, error) {
	res, err := r.links.ListMyJoinLinks(context.Background(), authed(token, &isshov1.ListMyJoinLinksRequest{}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (r *joinRig) redeem(token, code string) error {
	_, err := r.links.RedeemJoinLink(context.Background(), authed(token, &isshov1.RedeemJoinLinkRequest{Code: code}))
	return err
}

// linkReason is the JoinLinkProblem reason on a refusal, or unspecified.
func linkReason(err error) isshov1.JoinLinkProblemReason {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_UNSPECIFIED
	}
	for _, d := range ce.Details() {
		if v, err := d.Value(); err == nil {
			if p, ok := v.(*isshov1.JoinLinkProblem); ok {
				return p.GetReason()
			}
		}
	}
	return isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_UNSPECIFIED
}

// An admitted member makes 3 links over the wire, sees them, a fourth is
// refused with NONE_LEFT, and cancelling one frees a place.
func TestAMemberMakesThreeLinksSeesThemAndCancelsOne(t *testing.T) {
	r := newJoinRig(t)
	tok := r.signedIn(t, ada, "ada@example.com")
	r.admit(t, "ada@example.com")

	var made []*isshov1.JoinLink
	for i := 0; i < 3; i++ {
		l, err := r.create(tok)
		if err != nil {
			t.Fatal(err)
		}
		if l.GetCode() == "" || l.GetState() != isshov1.JoinLinkState_JOIN_LINK_STATE_OUTSTANDING {
			t.Fatalf("link = %v", l)
		}
		made = append(made, l)
	}
	_, err := r.create(tok)
	wantCode(t, err, connect.CodeFailedPrecondition)
	if got := linkReason(err); got != isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_NONE_LEFT {
		t.Fatalf("reason = %v, want NONE_LEFT", got)
	}

	listed, err := r.list(tok)
	if err != nil || len(listed.GetLinks()) != 3 || listed.GetRemaining() != 0 {
		t.Fatalf("list = %v, %v; want 3 links and none remaining", listed, err)
	}
	if want := made[0].GetCreatedAt().AsTime().Add(14 * 24 * time.Hour); !made[0].GetExpiresAt().AsTime().Equal(want) {
		t.Errorf("expires at %v, want 14 days after it was made (%v)", made[0].GetExpiresAt().AsTime(), want)
	}

	if err := r.cancel(tok, made[1].GetId()); err != nil {
		t.Fatal(err)
	}
	listed, _ = r.list(tok)
	if len(listed.GetLinks()) != 2 || listed.GetRemaining() != 1 {
		t.Fatalf("after a cancel: %v, want 2 links and 1 remaining", listed)
	}
	// A link that isn't the caller's, or isn't there, is NOT_FOUND.
	other := r.member(t, grace, "grace@example.com")
	wantCode(t, r.cancel(other, made[0].GetId()), connect.CodeNotFound)
	wantCode(t, r.cancel(tok, "not-an-id"), connect.CodeNotFound)
}

// Someone signed in but not admitted has no links to give.
func TestSomeoneNotAdmittedIsToldSoWhenMakingALink(t *testing.T) {
	r := newJoinRig(t)
	tok := r.signedIn(t, ada, "ada@example.com")
	_, err := r.create(tok)
	wantCode(t, err, connect.CodeFailedPrecondition)
	if got := linkReason(err); got != isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_NOT_ADMITTED {
		t.Fatalf("reason = %v, want NOT_ADMITTED", got)
	}
}

// The whole way in: a friend signs in, can read plans and is refused a signup,
// redeems the link, and then signs up. A second redeem of the link fails.
func TestAFriendReadsPlansIsRefusedASignupThenRedeemsAndSignsUp(t *testing.T) {
	r := newJoinRig(t)
	owner := r.signedIn(t, ada, "ada@example.com")
	r.admit(t, "ada@example.com")
	link, err := r.create(owner)
	if err != nil {
		t.Fatal(err)
	}
	friend := r.signedIn(t, grace, "grace@example.com")
	late := r.signedIn(t, third, "third@example.com")

	// Plans are readable by someone who isn't admitted (and by no one signed in).
	listed, err := r.plans.ListPlans(context.Background(), authed(friend, &isshov1.ListPlansRequest{City: "da-nang-hoi-an"}))
	if err != nil || len(listed.Msg.GetPlans()) == 0 {
		t.Fatalf("ListPlans = %v, %v", listed, err)
	}
	if _, err := r.plans.GetPlan(context.Background(), authed(friend, &isshov1.GetPlanRequest{Id: lanterns})); err != nil {
		t.Fatalf("GetPlan: %v", err)
	}
	// ...and refused a signup, even with a profile: it is not admitted.
	if _, err := r.save(friend, goodSave()); err != nil {
		t.Fatal(err)
	}
	_, err = r.signUp(friend, lanterns, tick("2026-01-09", "evening"))
	wantCode(t, err, connect.CodeFailedPrecondition)
	if got := signupReason(err); got != isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_NOT_ADMITTED {
		t.Fatalf("reason = %v, want NOT_ADMITTED", got)
	}
	if got := r.mine(t, friend); len(got) != 0 {
		t.Fatalf("a refused signup was kept: %v", got)
	}

	if err := r.redeem(friend, link.GetCode()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.signUp(friend, lanterns, tick("2026-01-09", "evening")); err != nil {
		t.Fatalf("an admitted friend was refused a signup: %v", err)
	}

	// The link is used: the second redeem fails, over the wire, and says no more
	// than that the link can't be used.
	err = r.redeem(late, link.GetCode())
	wantCode(t, err, connect.CodeFailedPrecondition)
	if got := linkReason(err); got != isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_LINK_UNUSABLE {
		t.Fatalf("reason = %v, want LINK_UNUSABLE", got)
	}
	_, err = r.signUp(late, lanterns, tick("2026-01-09", "evening"))
	wantCode(t, err, connect.CodeFailedPrecondition)

	// The member's link now reads used, and stays out of their 3.
	got, err := r.list(owner)
	if err != nil || len(got.GetLinks()) != 1 || got.GetLinks()[0].GetState() != isshov1.JoinLinkState_JOIN_LINK_STATE_USED || got.GetRemaining() != 2 {
		t.Fatalf("list = %v, %v", got, err)
	}
}

// A link that is no link's, or older than 14 days on the fake clock, can't be
// used, and the member's 3 get it back.
func TestAnExpiredLinkAndAMadeUpCodeAreUnusableAndTheExpiredOneReturns(t *testing.T) {
	r := newJoinRig(t)
	owner := r.signedIn(t, ada, "ada@example.com")
	r.admit(t, "ada@example.com")
	var codes []string
	for i := 0; i < 3; i++ {
		l, err := r.create(owner)
		if err != nil {
			t.Fatal(err)
		}
		codes = append(codes, l.GetCode())
	}
	r.clk.Advance(14 * 24 * time.Hour)
	friend := r.signedIn(t, grace, "grace@example.com")
	// Signed in after the clock moved, so the token is fresh.
	friend = r.issuer.Token(r.clk.Now(), grace, "grace@example.com")
	owner = r.issuer.Token(r.clk.Now(), ada, "ada@example.com")

	for _, code := range append(codes, "made-up") {
		err := r.redeem(friend, code)
		wantCode(t, err, connect.CodeFailedPrecondition)
		if got := linkReason(err); got != isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_LINK_UNUSABLE {
			t.Fatalf("code %q: reason = %v, want LINK_UNUSABLE", code, got)
		}
	}
	got, err := r.list(owner)
	if err != nil || got.GetRemaining() != 3 {
		t.Fatalf("list = %v, %v; want all 3 back", got, err)
	}
	if _, err := r.create(owner); err != nil {
		t.Fatalf("a member whose links expired could not make one: %v", err)
	}
}

// Someone already admitted who opens a link is told so, and the link isn't
// used.
func TestAnAdmittedMemberOpeningALinkLeavesItUnused(t *testing.T) {
	r := newJoinRig(t)
	owner := r.signedIn(t, ada, "ada@example.com")
	r.admit(t, "ada@example.com")
	link, _ := r.create(owner)
	err := r.redeem(owner, link.GetCode())
	wantCode(t, err, connect.CodeFailedPrecondition)
	if got := linkReason(err); got != isshov1.JoinLinkProblemReason_JOIN_LINK_PROBLEM_REASON_ALREADY_ADMITTED {
		t.Fatalf("reason = %v, want ALREADY_ADMITTED", got)
	}
	got, _ := r.list(owner)
	if got.GetLinks()[0].GetState() != isshov1.JoinLinkState_JOIN_LINK_STATE_OUTSTANDING {
		t.Fatalf("the link was used: %v", got)
	}
}

func TestEveryJoinLinkRPCNeedsASignedInCaller(t *testing.T) {
	r := newJoinRig(t)
	_, err := r.create("")
	wantCode(t, err, connect.CodeUnauthenticated)
	wantCode(t, r.cancel("", "x"), connect.CodeUnauthenticated)
	_, err = r.list("")
	wantCode(t, err, connect.CodeUnauthenticated)
	wantCode(t, r.redeem("", "x"), connect.CodeUnauthenticated)
}

// Fields that would say who brought in whom: none of them is in any message an
// RPC sends.
var broughtInNames = []string{"brought", "referr", "inviter", "invited_by", "admitted_by", "redeemed_by", "used_by", "member_id", "account_id"}

// Issho records who brought in whom and only a moderator sees it. This walks
// every RPC the API serves, as TestNoRPCReturnsAnotherPersonsEmail does for
// emails: no response message, however deep, has a field that could hold it.
// The one message that holds an account id is the caller's own account.
func TestNoRPCReturnsWhoBroughtInWhom(t *testing.T) {
	ownAccount := map[string]bool{"issho.v1.AccountService.GetAccount": true}
	var offenders []string
	walked := map[string]bool{}
	protoregistry.GlobalFiles.RangeFilesByPackage("issho.v1", func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Services().Len(); i++ {
			svc := fd.Services().Get(i)
			for j := 0; j < svc.Methods().Len(); j++ {
				m := svc.Methods().Get(j)
				walked[string(m.FullName())] = true
				if name := namesWhoBroughtInWhom(m.Output(), map[protoreflect.FullName]bool{}); name != "" && !ownAccount[string(m.FullName())] {
					offenders = append(offenders, string(m.FullName())+" ("+name+")")
				}
			}
		}
		return true
	})
	if len(offenders) > 0 {
		t.Fatalf("RPCs whose response could say who brought in whom: %v", offenders)
	}
	// The join-link RPCs are the ones that could, so the walk must be seen to
	// reach them.
	for _, rpc := range []string{"issho.v1.JoinLinkService.CreateJoinLink", "issho.v1.JoinLinkService.CancelJoinLink", "issho.v1.JoinLinkService.ListMyJoinLinks", "issho.v1.JoinLinkService.RedeemJoinLink"} {
		if !walked[rpc] {
			t.Errorf("the walk did not reach %s", rpc)
		}
	}
}

func namesWhoBroughtInWhom(msg protoreflect.MessageDescriptor, seen map[protoreflect.FullName]bool) string {
	if seen[msg.FullName()] {
		return ""
	}
	seen[msg.FullName()] = true
	for i := 0; i < msg.Fields().Len(); i++ {
		f := msg.Fields().Get(i)
		name := strings.ToLower(string(f.Name()))
		for _, bad := range broughtInNames {
			if strings.Contains(name, bad) {
				return string(msg.FullName()) + "." + string(f.Name())
			}
		}
		if f.Message() != nil {
			if n := namesWhoBroughtInWhom(f.Message(), seen); n != "" {
				return n
			}
		}
	}
	return ""
}

// The descriptor walk finds a field that could say it; this reads what the
// join-link RPCs actually send. Two people go through every call, one bringing
// in the other, and in no response, to either of them, is the other's account
// id or email.
func TestTheJoinLinkRPCsNeverSendWhoBroughtInWhom(t *testing.T) {
	r := newJoinRig(t)
	owner := r.signedIn(t, ada, "ada@example.com")
	r.admit(t, "ada@example.com")
	friend := r.signedIn(t, grace, "grace@example.com")

	var sent []string
	record := func(m proto.Message) {
		b, err := protojson.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		sent = append(sent, string(b))
	}
	made, err := r.create(owner)
	if err != nil {
		t.Fatal(err)
	}
	record(made)
	extra, _ := r.create(owner)
	if err := r.cancel(owner, extra.GetId()); err != nil {
		t.Fatal(err)
	}
	if err := r.redeem(friend, made.GetCode()); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{owner, friend} {
		got, err := r.list(tok)
		if err != nil {
			t.Fatal(err)
		}
		record(got)
	}
	// What a refused call says is read too.
	if err := r.redeem(friend, made.GetCode()); err == nil {
		t.Fatal("redeemed a used link")
	} else {
		sent = append(sent, err.Error())
	}

	all := strings.Join(sent, "\n")
	for _, leak := range []string{"@", "example.com", ada, grace} {
		if strings.Contains(all, leak) {
			t.Errorf("a join-link response carries %q: %s", leak, all)
		}
	}
	// The member's own link is what is meant to be shown, so the scan can see it.
	if !strings.Contains(all, made.GetCode()) {
		t.Errorf("the link's code is not in the responses, so the scan proved nothing: %s", all)
	}
}
