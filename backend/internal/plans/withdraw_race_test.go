package plans

import (
	"context"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationWithdrawWaitsForThePlanLock: a withdrawal arriving while a
// scheduling pass holds the plan's lock waits for it, so it can't interleave
// with the pass (which has already read the signup it would delete). Without
// the lock the withdrawal finishes at once.
func TestIntegrationWithdrawWaitsForThePlanLock(t *testing.T) {
	f := newFixture(t, "zz-plans-withdraw", "UTC", 1)
	ctx := context.Background()
	w := Window{Start: time.Date(2030, 1, 7, 0, 0, 0, 0, time.UTC), Days: 14, Deadline: time.Date(2030, 1, 20, 12, 0, 0, 0, time.UTC)}
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "en"}, Audience: 6, Score: 1}, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	user := f.users[0]
	if err := SaveSignup(ctx, f.pool, planID, matching.Signup{User: matching.UserID(user.String())}); err != nil {
		t.Fatal(err)
	}

	tx, err := f.pool.Begin(ctx) // the scheduling pass
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op once committed; frees the lock if the test fails
	if _, err := dbgen.New(tx).LockOpenPlan(ctx, planID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- WithdrawSignup(ctx, f.pool, planID, user) }()
	select {
	case err := <-done:
		t.Fatalf("a withdrawal finished while the plan was locked: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	var n int
	if err := f.pool.QueryRow(ctx, "select count(*) from app.plan_signups where plan_id = $1", planID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("signups while locked = %d, %v; want 1", n, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, "select count(*) from app.plan_signups where plan_id = $1", planID).Scan(&n); err != nil || n != 0 {
		t.Errorf("signups after the lock was released = %d, %v; want 0", n, err)
	}
}
