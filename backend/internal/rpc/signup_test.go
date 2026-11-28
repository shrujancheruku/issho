package rpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/gen/issho/v1/isshov1connect"
)

const (
	lanterns = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0001" // runs in the evening
	sunrise  = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0002" // runs in the morning
)

type signupRig struct {
	*profileRig
	signups isshov1connect.SignupServiceClient
	plans   isshov1connect.PlanServiceClient
}

func newSignupRig(t *testing.T) *signupRig {
	t.Helper()
	r := newProfileRig(t)
	return &signupRig{
		profileRig: r,
		signups:    isshov1connect.NewSignupServiceClient(r.srv.Client(), r.srv.URL),
		plans:      isshov1connect.NewPlanServiceClient(r.srv.Client(), r.srv.URL),
	}
}

// member is an admitted, signed-in account with a profile.
func (r *signupRig) member(t *testing.T, id, email string) string {
	t.Helper()
	tok := r.signedIn(t, id, email)
	r.admit(t, email)
	if _, err := r.save(tok, goodSave()); err != nil {
		t.Fatal(err)
	}
	return tok
}

func authed[T any](token string, msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	if token != "" {
		req.Header().Set("Authorization", "Bearer "+token)
	}
	return req
}

func (r *signupRig) signUp(token, plan string, ticks ...*isshov1.Tick) (*isshov1.Signup, error) {
	res, err := r.signups.SignUp(context.Background(), authed(token, &isshov1.SignUpRequest{PlanId: plan, Ticks: ticks}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetSignup(), nil
}

func tick(date, part string) *isshov1.Tick { return &isshov1.Tick{Date: date, Part: part} }

// signupReason is the SignupProblem reason on a refusal, or unspecified.
func signupReason(err error) isshov1.SignupProblemReason {
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_UNSPECIFIED
	}
	for _, d := range ce.Details() {
		if v, err := d.Value(); err == nil {
			if p, ok := v.(*isshov1.SignupProblem); ok {
				return p.GetReason()
			}
		}
	}
	return isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_UNSPECIFIED
}

// A signup by someone with no profile is refused, over the wire, with the
// reason that tells the app to ask the profile's questions (#613 proved
// rpc.RequireProfile on its own; this is the signup RPC calling it).
func TestSigningUpWithNoProfileIsRefusedWithProfileRequired(t *testing.T) {
	r := newSignupRig(t)
	tok := r.signedIn(t, ada, "ada@example.com")
	r.admit(t, "ada@example.com")

	_, err := r.signUp(tok, lanterns, tick("2026-01-09", "evening"))

	wantCode(t, err, connect.CodeFailedPrecondition)
	if got := problemOf(err); got != isshov1.ProfileProblemReason_PROFILE_PROBLEM_REASON_PROFILE_REQUIRED {
		t.Fatalf("reason = %v, want PROFILE_REQUIRED", got)
	}
	mine, err := r.signups.ListMySignups(context.Background(), authed(tok, &isshov1.ListMySignupsRequest{}))
	if err != nil || len(mine.Msg.GetSignups()) != 0 {
		t.Fatalf("a refused signup was kept: %v, %v", mine, err)
	}
}

// Admission is checked before the profile: someone not admitted and with no
// profile is told NOT_ADMITTED, not asked the profile's questions first
// (signup.proto, SignUp).
func TestSigningUpUnadmittedWithNoProfileIsRefusedWithNotAdmittedFirst(t *testing.T) {
	r := newSignupRig(t)
	tok := r.signedIn(t, ada, "ada@example.com")

	_, err := r.signUp(tok, lanterns, tick("2026-01-09", "evening"))

	wantCode(t, err, connect.CodeFailedPrecondition)
	if got := signupReason(err); got != isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_NOT_ADMITTED {
		t.Fatalf("reason = %v (profile reason %v), want NOT_ADMITTED", got, problemOf(err))
	}
}

func (r *signupRig) mine(t *testing.T, token string) []*isshov1.Signup {
	t.Helper()
	res, err := r.signups.ListMySignups(context.Background(), authed(token, &isshov1.ListMySignupsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.GetSignups()
}

// A member with a profile signs up to both fixture plans, each with its own
// times, and reads them back with the day they will hear by.
func TestAMemberSignsUpToBothPlansAndReadsThemBackWithTheDayTheyHearBy(t *testing.T) {
	r := newSignupRig(t)
	tok := r.member(t, ada, "ada@example.com")

	first, err := r.signUp(tok, lanterns, tick("2026-01-09", "evening"), tick("2026-01-08", "evening"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.signUp(tok, sunrise, tick("2026-01-10", "morning")); err != nil {
		t.Fatal(err)
	}

	// Ticks come back in time order, and the hear-by is the round's last day.
	if first.GetPlanId() != lanterns || first.GetHearBy() != "2026-01-11" ||
		len(first.GetTicks()) != 2 || first.GetTicks()[0].GetDate() != "2026-01-08" {
		t.Fatalf("signup = %v", first)
	}
	got := r.mine(t, tok)
	if len(got) != 2 || got[0].GetHearBy() != "2026-01-11" || got[1].GetHearBy() != "2026-01-11" {
		t.Fatalf("mine = %v, want both plans, each with its hear-by", got)
	}
}

// A signup is changed by signing up again and ended by withdrawing.
func TestAMemberChangesAndWithdrawsASignup(t *testing.T) {
	r := newSignupRig(t)
	tok := r.member(t, ada, "ada@example.com")
	if _, err := r.signUp(tok, lanterns, tick("2026-01-09", "evening")); err != nil {
		t.Fatal(err)
	}

	if _, err := r.signUp(tok, lanterns, tick("2026-01-12", "evening")); err != nil {
		t.Fatal(err)
	}
	got := r.mine(t, tok)
	if len(got) != 1 || len(got[0].GetTicks()) != 1 || got[0].GetTicks()[0].GetDate() != "2026-01-12" {
		t.Fatalf("after changing: %v", got)
	}

	if _, err := r.signups.WithdrawSignup(context.Background(), authed(tok, &isshov1.WithdrawSignupRequest{PlanId: lanterns})); err != nil {
		t.Fatal(err)
	}
	if got := r.mine(t, tok); len(got) != 0 {
		t.Fatalf("after withdrawing: %v", got)
	}
	_, err := r.signups.WithdrawSignup(context.Background(), authed(tok, &isshov1.WithdrawSignupRequest{PlanId: "nope"}))
	wantCode(t, err, connect.CodeNotFound)
}

// A tick that is too soon, or on a part of the day the plan doesn't allow, is
// refused with the reason the app can show, and nothing is saved.
func TestRefusedTicksCarryTheReason(t *testing.T) {
	r := newSignupRig(t)
	tok := r.member(t, ada, "ada@example.com")
	for name, c := range map[string]struct {
		plan string
		tick *isshov1.Tick
		want isshov1.SignupProblemReason
	}{
		"2 days ahead":       {lanterns, tick("2026-01-07", "evening"), isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_TICK_TOO_SOON},
		"a part not allowed": {lanterns, tick("2026-01-09", "morning"), isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_TICK_PART_NOT_ALLOWED},
		"too far ahead":      {lanterns, tick("2026-02-09", "evening"), isshov1.SignupProblemReason_SIGNUP_PROBLEM_REASON_TICK_TOO_FAR},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := r.signUp(tok, c.plan, c.tick)
			wantCode(t, err, connect.CodeInvalidArgument)
			if got := signupReason(err); got != c.want {
				t.Fatalf("reason = %v, want %v", got, c.want)
			}
		})
	}
	for name, ticks := range map[string][]*isshov1.Tick{
		"no ticks":          nil,
		"a date not a date": {tick("soon", "evening")},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := r.signUp(tok, lanterns, ticks...)
			wantCode(t, err, connect.CodeInvalidArgument)
		})
	}
	if got := r.mine(t, tok); len(got) != 0 {
		t.Fatalf("a refused signup was kept: %v", got)
	}
}

func TestSigningUpToAPlanThatIsNotThereIsNotFound(t *testing.T) {
	r := newSignupRig(t)
	tok := r.member(t, ada, "ada@example.com")
	_, err := r.signUp(tok, "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b00ff", tick("2026-01-09", "evening"))
	wantCode(t, err, connect.CodeNotFound)
}

// Every signup RPC is for signed-in members: with no token the call is refused
// before anything is read.
func TestTheSignupRPCsNeedASignedInCaller(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	_, err := r.signups.SignUp(ctx, connect.NewRequest(&isshov1.SignUpRequest{PlanId: lanterns, Ticks: []*isshov1.Tick{tick("2026-01-09", "evening")}}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = r.signups.WithdrawSignup(ctx, connect.NewRequest(&isshov1.WithdrawSignupRequest{PlanId: lanterns}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = r.signups.ListMySignups(ctx, connect.NewRequest(&isshov1.ListMySignupsRequest{}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

// A plan, read with no token, says how many are signed up and nothing of who;
// a member's own list holds only their own.
func TestAPlanReturnsHowManyAreSignedUpAndNeverWho(t *testing.T) {
	r := newSignupRig(t)
	adaTok := r.member(t, ada, "ada@example.com")
	graceTok := r.member(t, grace, "grace@example.com")
	for _, tok := range []string{adaTok, graceTok} {
		if _, err := r.signUp(tok, lanterns, tick("2026-01-09", "evening")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.signUp(adaTok, sunrise, tick("2026-01-09", "morning")); err != nil {
		t.Fatal(err)
	}

	listed, err := r.plans.ListPlans(context.Background(), connect.NewRequest(&isshov1.ListPlansRequest{City: "da-nang-hoi-an"}))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int32{}
	for _, p := range listed.Msg.GetPlans() {
		counts[p.GetId()] = p.GetSignedUp()
	}
	if counts[lanterns] != 2 || counts[sunrise] != 1 {
		t.Fatalf("counts = %v, want lanterns 2 and sunrise 1", counts)
	}
	one, err := r.plans.GetPlan(context.Background(), connect.NewRequest(&isshov1.GetPlanRequest{Id: lanterns}))
	if err != nil || one.Msg.GetPlan().GetSignedUp() != 2 {
		t.Fatalf("GetPlan = %v, %v; want 2 signed up", one, err)
	}
	// The only things on a Plan about signups are the count; nothing names or
	// lists a person.
	fields := one.Msg.GetPlan().ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		switch name := string(fields.Get(i).Name()); name {
		case "people", "signups", "members", "names", "accounts":
			t.Fatalf("Plan has a %q field", name)
		}
	}
	if graceOwn := r.mine(t, graceTok); len(graceOwn) != 1 || graceOwn[0].GetPlanId() != lanterns {
		t.Fatalf("grace's own signups = %v, want only the lantern plan", graceOwn)
	}
}

// A signup says when it was made or last changed, over the wire, in what
// SignUp and ListMySignups return: the app prefills the ticks from the newest.
func TestASignupSaysWhenItWasMadeOrLastChangedOverTheWire(t *testing.T) {
	r := newSignupRig(t)
	tok := r.member(t, ada, "ada@example.com")
	first := r.clk.Now()

	made, err := r.signUp(tok, lanterns, tick("2026-01-09", "evening"))
	if err != nil {
		t.Fatal(err)
	}
	if !made.GetSignedAt().AsTime().Equal(first) {
		t.Fatalf("SignUp signed_at = %v, want %v", made.GetSignedAt().AsTime(), first)
	}
	r.clk.Advance(time.Minute)
	if _, err := r.signUp(tok, sunrise, tick("2026-01-10", "morning")); err != nil {
		t.Fatal(err)
	}
	r.clk.Advance(time.Minute)
	changed := r.clk.Now()
	if _, err := r.signUp(tok, lanterns, tick("2026-01-10", "evening")); err != nil {
		t.Fatal(err)
	}

	got := map[string]time.Time{}
	for _, s := range r.mine(t, tok) {
		got[s.GetPlanId()] = s.GetSignedAt().AsTime()
	}
	if !got[lanterns].Equal(changed) || !got[sunrise].Equal(first.Add(time.Minute)) {
		t.Fatalf("mine signed_at = %v; want lanterns at %v (last changed) and sunrise a minute after the first", got, changed)
	}
}

// Both public reads say the plan's round's last day, to a visitor with no
// token: it is the same for everyone.
func TestAPlanSaysItsRoundsLastDayToAVisitor(t *testing.T) {
	r := newSignupRig(t)

	listed, err := r.plans.ListPlans(context.Background(), connect.NewRequest(&isshov1.ListPlansRequest{City: "da-nang-hoi-an"}))
	if err != nil || len(listed.Msg.GetPlans()) != 2 {
		t.Fatalf("ListPlans = %v, %v", listed, err)
	}
	for _, p := range listed.Msg.GetPlans() {
		if p.GetRoundEnds() != "2026-01-11" {
			t.Fatalf("listed plan %s round_ends = %q, want 2026-01-11", p.GetId(), p.GetRoundEnds())
		}
	}
	one, err := r.plans.GetPlan(context.Background(), connect.NewRequest(&isshov1.GetPlanRequest{Id: lanterns}))
	if err != nil || one.Msg.GetPlan().GetRoundEnds() != "2026-01-11" {
		t.Fatalf("GetPlan = %v, %v; want round_ends 2026-01-11", one, err)
	}
}
