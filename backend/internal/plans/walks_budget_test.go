package plans

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// With less than walks.MinBudget left on the job's context, Suggest starts
// no walk: the proposer is never called, nothing is rested or failed, and
// the walks are counted as deferred for the next run. With enough time it
// composes as usual.
func TestIntegrationSuggestDefersWalksWithoutBudget(t *testing.T) {
	f := newFixture(t, "zz-plans-walkbudget", "Asia/Tokyo", 8)
	bg := context.Background()
	for _, id := range f.users {
		f.addProfile(t, id, "ja")
	}
	for _, s := range []string{
		`update app.activities set walk_theme = 'temples', name = '{"en":"Temples and shrines"}' where id = '` + f.act + `'`,
		`update app.areas set centroid = 'POINT(139.58 35.703)', name = '{"en":"Kichijoji"}' where id = '` + f.area + `'`,
	} {
		if _, err := f.pool.Exec(bg, s); err != nil {
			t.Fatal(err)
		}
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	prop, router := &walkProposer{}, &walkRouter{}
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: log, WalkRoutes: router,
		Walks: &walks.Composer{Proposer: prop, Places: findAll{}, Routes: router, Rules: walks.DefaultRules(), Log: log}}
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)

	short, cancel := context.WithTimeout(bg, walks.MinBudget-time.Minute)
	defer cancel()
	rep, err := r.Suggest(short, f.city, now)
	if err != nil || rep.Opened != 0 || rep.Failed != 0 || rep.Deferred == 0 || prop.calls != 0 {
		t.Fatalf("short budget: %+v after %d proposals, %v", rep, prop.calls, err)
	}

	// Deferred is not a failure: the area isn't rested, so with time the
	// same pass opens the walk at once.
	long, cancel2 := context.WithTimeout(bg, walks.MinBudget+10*time.Minute)
	defer cancel2()
	if rep, err := r.Suggest(long, f.city, now); err != nil || rep.Opened != 1 || rep.Deferred != 0 || prop.calls == 0 {
		t.Fatalf("enough budget: %+v after %d proposals, %v", rep, prop.calls, err)
	}
}

// slowWalkProposer is a model that takes a moment, so the budget can run out
// between a rejected attempt and its retry.
type slowWalkProposer struct {
	*walkProposer
	delay time.Duration
}

func (s slowWalkProposer) Propose(ctx context.Context, req walks.Request, fb []string) (walks.Proposal, error) {
	time.Sleep(s.delay)
	return s.walkProposer.Propose(ctx, req, fb)
}

// Out of time between attempts is a deferral: counted as Deferred, not
// Failed, nothing rests, no further proposal is asked for, and the pass
// returns normally so meeting places and routes still run.
func TestIntegrationSuggestDefersWhenBudgetRunsOutBetweenAttempts(t *testing.T) {
	f := newFixture(t, "zz-plans-walkbudget2", "Asia/Tokyo", 8)
	bg := context.Background()
	for _, id := range f.users {
		f.addProfile(t, id, "ja")
	}
	for _, s := range []string{
		`update app.activities set walk_theme = 'temples', name = '{"en":"Temples and shrines"}' where id = '` + f.act + `'`,
		`update app.areas set centroid = 'POINT(139.58 35.703)', name = '{"en":"Kichijoji"}' where id = '` + f.area + `'`,
	} {
		if _, err := f.pool.Exec(bg, s); err != nil {
			t.Fatal(err)
		}
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	prop, router := &walkProposer{bad: true}, &walkRouter{}
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: log, WalkRoutes: router,
		Walks: &walks.Composer{Proposer: slowWalkProposer{prop, 1500 * time.Millisecond}, Places: findAll{}, Routes: router, Rules: walks.DefaultRules(), Log: log}}
	ctx, cancel := context.WithTimeout(bg, walks.MinBudget+time.Second)
	defer cancel()
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	rep, err := r.Suggest(ctx, f.city, now)
	if err != nil || rep.Deferred != 1 || rep.Failed != 0 || rep.Opened != 0 || prop.calls != 1 {
		t.Fatalf("%+v after %d proposals, %v", rep, prop.calls, err)
	}
	// No rest was recorded: with time, the area is tried again at once.
	prop.bad = false
	if rep, err := r.Suggest(bg, f.city, now); err != nil || rep.Opened != 1 {
		t.Fatalf("after the deferral: %+v, %v", rep, err)
	}
}
