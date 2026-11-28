package people_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/policy"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

const (
	hopper = "33333333-3333-4333-8333-333333333333"
)

// account makes the account of a first sign-in.
func (r *rig) account(t *testing.T, id, email string) {
	t.Helper()
	if _, err := r.svc.Join(context.Background(), people.Claims{Subject: id, Email: email}, people.Preferences{}); err != nil {
		t.Fatal(err)
	}
}

// admitted makes an account and admits it the way the seed command does, so it
// is a member with join links to give.
func (r *rig) admitted(t *testing.T, id, email string) {
	t.Helper()
	r.account(t, id, email)
	if err := people.AdmitByEmail(context.Background(), r.pool, email, r.clk.Now()); err != nil {
		t.Fatal(err)
	}
}

func (r *rig) makeLink(t *testing.T, member string) people.JoinLink {
	t.Helper()
	l, err := r.svc.MakeJoinLink(context.Background(), member)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func (r *rig) remaining(t *testing.T, member string) int {
	t.Helper()
	got, err := r.svc.MyJoinLinks(context.Background(), member)
	if err != nil {
		t.Fatal(err)
	}
	return got.Remaining
}

func wantErr(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
}

// Criterion: a member can make a link while they have fewer than 3, and a
// fourth is refused.
func TestAMemberCanMakeThreeLinksAndAFourthIsRefused(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	seen := map[string]bool{}
	for i := 0; i < policy.JoinLinksPerMember; i++ {
		l := r.makeLink(t, ada)
		if l.Code == "" || seen[l.Code] {
			t.Fatalf("link %d has code %q, which is empty or one already made", i, l.Code)
		}
		seen[l.Code] = true
		if l.State != people.JoinLinkOutstanding {
			t.Errorf("a new link is %q, want outstanding", l.State)
		}
	}
	_, err := r.svc.MakeJoinLink(context.Background(), ada)
	wantErr(t, err, people.ErrNoJoinLinksLeft)
	if got := r.remaining(t, ada); got != 0 {
		t.Errorf("remaining = %d after three, want 0", got)
	}
}

// Criterion: a member can cancel a link, and it returns to their 3.
func TestCancellingALinkReturnsItToTheMembersThree(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	first := r.makeLink(t, ada)
	r.makeLink(t, ada)
	r.makeLink(t, ada)

	if err := r.svc.CancelJoinLink(context.Background(), ada, first.ID); err != nil {
		t.Fatal(err)
	}
	if got := r.remaining(t, ada); got != 1 {
		t.Fatalf("remaining after a cancel = %d, want 1", got)
	}
	// The cancelled link no longer admits anyone.
	r.account(t, grace, "grace@example.com")
	wantErr(t, r.svc.RedeemJoinLink(context.Background(), grace, first.Code), people.ErrJoinLinkUnusable)
	// The freed one can be made again.
	r.makeLink(t, ada)
}

// A member can only cancel their own links, and only a link still outstanding.
func TestAMemberCannotCancelAnotherMembersLinkOrOneAlreadyUsed(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	r.admitted(t, grace, "grace@example.com")
	adas := r.makeLink(t, ada)

	wantErr(t, r.svc.CancelJoinLink(context.Background(), grace, adas.ID), people.ErrJoinLinkNotFound)
	wantErr(t, r.svc.CancelJoinLink(context.Background(), ada, "00000000-0000-4000-8000-000000000000"), people.ErrJoinLinkNotFound)
	wantErr(t, r.svc.CancelJoinLink(context.Background(), ada, "not-an-id"), people.ErrJoinLinkNotFound)

	r.account(t, hopper, "hopper@example.com")
	if err := r.svc.RedeemJoinLink(context.Background(), hopper, adas.Code); err != nil {
		t.Fatal(err)
	}
	wantErr(t, r.svc.CancelJoinLink(context.Background(), ada, adas.ID), people.ErrJoinLinkNotFound)
}

// An account that isn't admitted has no links to give.
func TestSomeoneNotAdmittedCannotMakeALink(t *testing.T) {
	r := newRig(t)
	r.account(t, ada, "ada@example.com")
	_, err := r.svc.MakeJoinLink(context.Background(), ada)
	wantErr(t, err, people.ErrNotAdmitted)
}

// An account that isn't admitted has none left to make, so the app never offers
// a link MakeJoinLink would refuse.
func TestSomeoneNotAdmittedHasNoLinksLeftToMake(t *testing.T) {
	r := newRig(t)
	r.account(t, ada, "ada@example.com")
	got, err := r.svc.MyJoinLinks(context.Background(), ada)
	if err != nil {
		t.Fatal(err)
	}
	if got.Remaining != 0 || len(got.Links) != 0 {
		t.Fatalf("MyJoinLinks = %+v, want no links and none left", got)
	}
}

// Criterion: redeeming a link admits the signed-in account and uses the link,
// and a second redeem of the same link fails.
func TestRedeemingALinkAdmitsTheAccountAndASecondRedeemFails(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	l := r.makeLink(t, ada)
	r.account(t, grace, "grace@example.com")
	r.account(t, hopper, "hopper@example.com")

	if ok, err := r.svc.Admitted(context.Background(), grace); err != nil || ok {
		t.Fatalf("Admitted before = %v, %v; want false", ok, err)
	}
	if err := r.svc.RedeemJoinLink(context.Background(), grace, l.Code); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.svc.Admitted(context.Background(), grace); err != nil || !ok {
		t.Fatalf("Admitted after = %v, %v; want true", ok, err)
	}

	// One use: another account holding the code is refused and not admitted.
	wantErr(t, r.svc.RedeemJoinLink(context.Background(), hopper, l.Code), people.ErrJoinLinkUnusable)
	if ok, _ := r.svc.Admitted(context.Background(), hopper); ok {
		t.Error("a second redeem of a used link admitted a second account")
	}
	// A used link is not returned to the member's 3.
	if got := r.remaining(t, ada); got != policy.JoinLinksPerMember-1 {
		t.Errorf("remaining = %d, want %d", got, policy.JoinLinksPerMember-1)
	}
}

func TestACodeThatIsNoLinkIsRefused(t *testing.T) {
	r := newRig(t)
	r.account(t, grace, "grace@example.com")
	for _, code := range []string{"", "nonsense", "AAAAAAAAAAAAAAAAAAAAAA"} {
		wantErr(t, r.svc.RedeemJoinLink(context.Background(), grace, code), people.ErrJoinLinkUnusable)
	}
	if ok, _ := r.svc.Admitted(context.Background(), grace); ok {
		t.Error("a code that is no link admitted someone")
	}
}

// Someone already a member who opens a link isn't admitted twice, and the link
// stays unused for the friend it was made for.
func TestAMemberRedeemingALinkLeavesItUnused(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	r.admitted(t, grace, "grace@example.com")
	l := r.makeLink(t, ada)

	wantErr(t, r.svc.RedeemJoinLink(context.Background(), grace, l.Code), people.ErrAlreadyAdmitted)
	wantErr(t, r.svc.RedeemJoinLink(context.Background(), ada, l.Code), people.ErrAlreadyAdmitted)

	r.account(t, hopper, "hopper@example.com")
	if err := r.svc.RedeemJoinLink(context.Background(), hopper, l.Code); err != nil {
		t.Fatalf("the friend the link was made for was refused: %v", err)
	}
}

func TestAnAccountThatDoesNotExistCannotRedeem(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	l := r.makeLink(t, ada)
	wantErr(t, r.svc.RedeemJoinLink(context.Background(), grace, l.Code), people.ErrNotFound)
	// And the link wasn't spent on it.
	r.account(t, grace, "grace@example.com")
	if err := r.svc.RedeemJoinLink(context.Background(), grace, l.Code); err != nil {
		t.Fatal(err)
	}
}

// Criterion: a link older than 14 days on the fake clock fails, and counts
// toward the member's 3 again. The expected 14 days are the spec's own words.
func TestALinkOlderThanFourteenDaysFailsAndReturnsToTheMembersThree(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	old := r.makeLink(t, ada)
	r.makeLink(t, ada)
	r.makeLink(t, ada)
	_, err := r.svc.MakeJoinLink(context.Background(), ada)
	wantErr(t, err, people.ErrNoJoinLinksLeft)

	r.clk.Advance(14*24*time.Hour - time.Second)
	r.account(t, grace, "grace@example.com")
	// Just inside the 14 days a link still admits (this one is spent here, so
	// the expiry below is checked on the other two).
	if err := r.svc.RedeemJoinLink(context.Background(), grace, old.Code); err != nil {
		t.Fatalf("a link 14 days less a second old was refused: %v", err)
	}

	r.clk.Advance(time.Second)
	r.account(t, hopper, "hopper@example.com")
	links, err := r.svc.MyJoinLinks(context.Background(), ada)
	if err != nil {
		t.Fatal(err)
	}
	if links.Remaining != 2 {
		t.Fatalf("remaining = %d with two expired and one used, want 2", links.Remaining)
	}
	for _, l := range links.Links {
		if l.Code == old.Code {
			continue
		}
		if l.State != people.JoinLinkExpired {
			t.Errorf("a link 14 days old is %q, want expired", l.State)
		}
		wantErr(t, r.svc.RedeemJoinLink(context.Background(), hopper, l.Code), people.ErrJoinLinkUnusable)
	}
	// The two that expired can be made again.
	r.makeLink(t, ada)
	r.makeLink(t, ada)
	_, err = r.svc.MakeJoinLink(context.Background(), ada)
	wantErr(t, err, people.ErrNoJoinLinksLeft)
}

// Criterion: Issho records who brought in whom. The record is there for the
// moderator's view (a later ticket); no member call returns it (the rpc package
// proves that).
func TestWhoBroughtInWhomIsRecorded(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	l := r.makeLink(t, ada)
	r.account(t, grace, "grace@example.com")
	if err := r.svc.RedeemJoinLink(context.Background(), grace, l.Code); err != nil {
		t.Fatal(err)
	}

	by, ok, err := r.svc.BroughtInBy(context.Background(), grace)
	if err != nil || !ok || by != ada {
		t.Fatalf("BroughtInBy(grace) = %q, %v, %v; want %q", by, ok, err, ada)
	}
	// The owner came in by the seed, not a link, so nobody brought them.
	if _, ok, err := r.svc.BroughtInBy(context.Background(), ada); err != nil || ok {
		t.Fatalf("BroughtInBy(seeded) = %v, %v; want nobody", ok, err)
	}
	// Someone not admitted was brought by nobody.
	r.account(t, hopper, "hopper@example.com")
	if _, ok, err := r.svc.BroughtInBy(context.Background(), hopper); err != nil || ok {
		t.Fatalf("BroughtInBy(not admitted) = %v, %v; want nobody", ok, err)
	}
}

// A person brought in can bring others, and the chain is kept.
func TestAMemberBroughtInByALinkHasLinksOfTheirOwn(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	r.account(t, grace, "grace@example.com")
	if err := r.svc.RedeemJoinLink(context.Background(), grace, r.makeLink(t, ada).Code); err != nil {
		t.Fatal(err)
	}
	r.account(t, hopper, "hopper@example.com")
	if err := r.svc.RedeemJoinLink(context.Background(), hopper, r.makeLink(t, grace).Code); err != nil {
		t.Fatal(err)
	}
	if by, _, _ := r.svc.BroughtInBy(context.Background(), hopper); by != grace {
		t.Errorf("hopper was brought in by %q, want grace", by)
	}
}

// Criterion: an account that isn't admitted is refused by RequireAdmitted
// (what a signup asks) and an admitted one passes.
func TestRequireAdmittedRefusesAnAccountThatIsNot(t *testing.T) {
	r := newRig(t)
	r.account(t, ada, "ada@example.com")
	wantErr(t, r.svc.RequireAdmitted(context.Background(), ada), people.ErrNotAdmitted)
	wantErr(t, r.svc.RequireAdmitted(context.Background(), grace), people.ErrNotFound)
	if err := people.AdmitByEmail(context.Background(), r.pool, "ada@example.com", r.clk.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.RequireAdmitted(context.Background(), ada); err != nil {
		t.Fatal(err)
	}
}

// Criterion: the owner's account can be admitted by a seed, so the first links
// have a source. It takes the account by its email, and again changes nothing.
func TestTheSeedAdmitsAnAccountByEmailAndRunningItAgainChangesNothing(t *testing.T) {
	r := newRig(t)
	r.account(t, ada, "ada@example.com")

	if err := people.AdmitByEmail(context.Background(), r.pool, "Ada@Example.com", r.clk.Now()); err != nil {
		t.Fatal(err)
	}
	if ok, _ := r.svc.Admitted(context.Background(), ada); !ok {
		t.Fatal("the seed did not admit the account")
	}
	r.clk.Advance(time.Hour)
	if err := people.AdmitByEmail(context.Background(), r.pool, "ada@example.com", r.clk.Now()); err != nil {
		t.Fatalf("seeding twice: %v", err)
	}
	// An account that hasn't signed in yet can't be admitted: the seed says so
	// rather than making one.
	wantErr(t, people.AdmitByEmail(context.Background(), r.pool, "nobody@example.com", r.clk.Now()), people.ErrNotFound)
}

// The list is the member's own: their links, with the state of each, and never
// who used one.
func TestAMemberSeesTheirOwnLinksAndNobodyElses(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	r.admitted(t, grace, "grace@example.com")
	mine := r.makeLink(t, ada)
	r.makeLink(t, grace)

	got, err := r.svc.MyJoinLinks(context.Background(), ada)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Links) != 1 || got.Links[0].ID != mine.ID || got.Links[0].Code != mine.Code {
		t.Fatalf("links = %+v, want only ada's", got.Links)
	}
	if want := mine.CreatedAt.Add(policy.JoinLinkLife); !got.Links[0].ExpiresAt.Equal(want) {
		t.Errorf("expires at %v, want %v", got.Links[0].ExpiresAt, want)
	}
	if got.Remaining != policy.JoinLinksPerMember-1 {
		t.Errorf("remaining = %d", got.Remaining)
	}
}

// Two calls at once can't make a fourth link.
func TestTwoLinksMadeAtOnceCannotPassTheThree(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	r.makeLink(t, ada)
	r.makeLink(t, ada)

	// Hold ada's account row so every call queues on it, then let them go: the
	// calls overlap for certain, and only the lock keeps them to one link.
	ctx := context.Background()
	hold, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = hold.Rollback(ctx) }()
	if _, err := hold.Exec(ctx, `select 1 from app.accounts where id = $1 for update`, ada); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		go func() {
			<-start
			_, err := r.svc.MakeJoinLink(ctx, ada)
			errs <- err
		}()
	}
	close(start)
	testdb.WaitForLockWaiters(t, r.pool, "LockAccount", 6)
	if err := hold.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	made := 0
	for i := 0; i < 6; i++ {
		if err := <-errs; err == nil {
			made++
		} else if !errors.Is(err, people.ErrNoJoinLinksLeft) {
			t.Fatal(err)
		}
	}
	if made != 1 {
		t.Fatalf("%d links were made at once with one left", made)
	}
}

// Two accounts redeeming the same link at once: one is admitted.
func TestTwoRedeemsAtOnceAdmitOne(t *testing.T) {
	r := newRig(t)
	r.admitted(t, ada, "ada@example.com")
	l := r.makeLink(t, ada)
	r.account(t, grace, "grace@example.com")
	r.account(t, hopper, "hopper@example.com")

	// Hold the link's row so both redeems queue on it, then let them go: they
	// overlap for certain, and only UseJoinLink's guard admits one.
	ctx := context.Background()
	hold, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = hold.Rollback(ctx) }()
	if _, err := hold.Exec(ctx, `select 1 from app.join_links where code = $1 for update`, l.Code); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, id := range []string{grace, hopper} {
		go func() {
			<-start
			errs <- r.svc.RedeemJoinLink(ctx, id, l.Code)
		}()
	}
	close(start)
	testdb.WaitForLockWaiters(t, r.pool, "UseJoinLink", 2)
	if err := hold.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var failed int
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			wantErr(t, err, people.ErrJoinLinkUnusable)
			failed++
		}
	}
	if failed != 1 {
		t.Fatalf("%d of 2 simultaneous redeems failed, want 1", failed)
	}
}

// One account redeeming two members' links at once is admitted once: the
// account row is locked (LockAccount), so the second call sees it admitted,
// is refused with ErrAlreadyAdmitted, and leaves its link unused.
func TestOneAccountRedeemingTwoLinksAtOnceIsAdmittedOnce(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	r.admitted(t, ada, "ada@example.com")
	r.admitted(t, grace, "grace@example.com")
	codes := []string{r.makeLink(t, ada).Code, r.makeLink(t, grace).Code}
	r.account(t, hopper, "hopper@example.com")

	// Hold hopper's account row so both redeems queue on it, then let them go:
	// without that, the second could simply start after the first committed.
	hold, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = hold.Rollback(ctx) }()
	if _, err := hold.Exec(ctx, `select 1 from app.accounts where id = $1 for update`, hopper); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make([]error, len(codes))
	done := make(chan struct{})
	for i, code := range codes {
		go func() {
			defer func() { done <- struct{}{} }()
			<-start
			errs[i] = r.svc.RedeemJoinLink(ctx, hopper, code)
		}()
	}
	close(start)
	testdb.WaitForLockWaiters(t, r.pool, "LockAccount", len(codes))
	if err := hold.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for range codes {
		<-done
	}

	admitted, refused := -1, -1
	for i, err := range errs {
		switch {
		case err == nil:
			admitted = i
		case errors.Is(err, people.ErrAlreadyAdmitted):
			refused = i
		default:
			t.Fatalf("redeem %d: %v", i, err)
		}
	}
	if admitted < 0 || refused < 0 {
		t.Fatalf("redeems = %v, want one admitted and one ErrAlreadyAdmitted", errs)
	}
	// The refused call's link is still outstanding, not used.
	owner := []string{ada, grace}[refused]
	links, err := r.svc.MyJoinLinks(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(links.Links) != 1 || links.Links[0].State != people.JoinLinkOutstanding {
		t.Errorf("the refused call's link = %+v, want it unused", links.Links)
	}
}
