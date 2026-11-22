package season_test

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/season"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

// minute is the budget for one run: the ticket's "under a minute" (the whole
// package is a few runs, each of which must fit on its own).
const minute = 60 * time.Second

// lockedBuffer is a writer River's goroutines and the run can both log to.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) Bytes() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]byte(nil), l.b.Bytes()...)
}

// run is one season on its own database. The standard logger (which slog's
// default logger writes through) is captured too, so what the run logs includes
// anything a package printed to it; the returned log is the run's own lines
// and that, together.
func run(t *testing.T, opts season.Options) (season.Result, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), minute)
	defer cancel()
	pool := testdb.NewDatabase(t)
	std := &lockedBuffer{}
	prev := log.Writer()
	log.SetOutput(std)
	defer log.SetOutput(prev)
	res, err := season.Run(ctx, pool, opts)
	if err != nil {
		t.Fatalf("the season: %v", err)
	}
	return res, append(append([]byte(nil), res.Log...), std.Bytes()...)
}

var (
	once    sync.Once
	first   season.Result
	firstLg []byte
)

// theSeason is the one run most tests read; the tests that need another make
// their own.
func theSeason(t *testing.T) (season.Result, []byte) {
	t.Helper()
	once.Do(func() { first, firstLg = run(t, season.Options{}) })
	if first.Outings == nil {
		t.Fatal("the shared season did not run")
	}
	return first, firstLg
}

// Criterion: a cast of about 20 signs up to the two fixture plans over 2 weeks
// on the fake clock, through the packages' exported APIs, from join links to
// feedback.
func TestAScriptedCastGoesFromJoinLinksToFeedbackOverTwoWeeks(t *testing.T) {
	res, _ := theSeason(t)
	if n := len(res.Cast); n < 18 || n > 22 {
		t.Errorf("the cast is %d people, want about 20", n)
	}
	if days := res.LastSignup.Sub(res.FirstSignup); days < 7*24*time.Hour {
		t.Errorf("signups span %s, want two rounds", days)
	}
	// Every stage ran, or the season is only a shell around some of the packages.
	for _, stage := range []string{
		"join link made", "join link redeemed", "profile saved", "signed up", "formed", "yes", "no", "set",
		"cancelled", "checked in", "no show", "feedback saved", "expired",
	} {
		if res.Stages[stage] == 0 {
			t.Errorf("stage %q never happened; stages: %v", stage, res.Stages)
		}
	}
	// The rulings the cast must respect show up as refusals the packages made.
	for _, refusal := range []string{
		"fourth join link", "used join link", "not admitted", "yes at a taken time",
	} {
		if res.Refusals[refusal] == 0 {
			t.Errorf("refusal %q never happened; refusals: %v", refusal, res.Refusals)
		}
	}
}

// Criterion: two runs give the same outings.
func TestTwoRunsGiveTheSameOutings(t *testing.T) {
	a, _ := theSeason(t)
	b, _ := run(t, season.Options{})
	if len(a.Outings) == 0 {
		t.Fatal("the season formed no outings, so there is nothing to compare")
	}
	if !reflect.DeepEqual(a.Outings, b.Outings) {
		t.Errorf("two runs differ:\n%v\n%v", a.Outings, b.Outings)
	}
	if a.Scorecard != b.Scorecard {
		t.Errorf("two runs' scorecards differ: %v and %v", a.Scorecard, b.Scorecard)
	}
}

// Criterion: it ends by checking invariants: every outing that was set had 3 to
// 6 people sharing a language and a time, nobody was double-booked, and nobody
// who isn't admitted has a signup.
func TestTheSeasonEndsWithEveryInvariantHolding(t *testing.T) {
	res, _ := theSeason(t)
	if len(res.Violations) != 0 {
		t.Fatalf("broken invariants:\n%s", strings.Join(res.Violations, "\n"))
	}
	if res.Checked["set outings"] < 4 || res.Checked["double-booking sweeps"] == 0 || res.Checked["not admitted"] == 0 {
		t.Errorf("the invariants checked too little to mean anything: %v", res.Checked)
	}
}

func TestAWorldWithNothingBrokenPasses(t *testing.T) {
	if bad := season.CheckInvariants(season.Broken("")); len(bad) != 0 {
		t.Errorf("a sound world broke invariants: %v", bad)
	}
}

// A season whose invariant check cannot fail proves nothing: break the rule on
// purpose and see it named.
func TestAnInvariantBrokenOnPurposeIsReported(t *testing.T) {
	for _, bug := range []string{"double-book", "unadmitted-signup", "set-too-small", "set-too-big", "no-shared-language", "no-shared-time"} {
		t.Run(bug, func(t *testing.T) {
			res := season.CheckInvariants(season.Broken(bug))
			if len(res) == 0 {
				t.Fatalf("a season with %s passed its invariants", bug)
			}
		})
	}
}

// Criterion: a command prints the share of signups that got an outing and the
// people who got none.
func TestTheScorecardSaysWhatShareGotAnOutingAndWhoGotNone(t *testing.T) {
	res, _ := theSeason(t)
	s := res.Scorecard
	if s.Signups == 0 || s.GotAnOuting == 0 || s.GotAnOuting > s.Grouped || s.Grouped > s.Signups {
		t.Errorf("scorecard does not add up: %+v", s)
	}
	out := s.String()
	for _, want := range []string{"signups", "got an outing", "%", "people who got none"} {
		if !strings.Contains(out, want) {
			t.Errorf("scorecard %q lacks %q", out, want)
		}
	}
}

// Criterion: the whole run's log output holds none of the cast's emails or
// names.
func TestTheLogHoldsNoEmailOrNameOfTheCast(t *testing.T) {
	res, lg := theSeason(t)
	if !bytes.Contains(lg, []byte(`"cmd":"season"`)) || !bytes.Contains(lg, []byte("client")) || bytes.Count(lg, []byte("\n")) < 50 {
		t.Fatalf("the captured log is too thin to search (%d bytes): River's and the run's lines are both expected", len(lg))
	}
	if leaks := season.Search(lg, res.Cast); len(leaks) != 0 {
		t.Errorf("the log holds the cast's own details: %v", leaks)
	}
}

// Criterion: planting one in a log line makes the test fail, and for that
// reason: the search names the planted member and which of their details, and
// nothing else.
func TestAPlantedNameInTheRunsLogIsFound(t *testing.T) {
	res, lg := theSeason(t)
	m := res.Cast[3]
	plants := map[string]struct {
		line string
		kind season.LeakKind
	}{
		"email":      {`{"msg":"x","who":"` + m.Email + `"}`, season.LeakEmail},
		"full name":  {`{"msg":"x","who":"` + m.Name + `"}`, season.LeakName},
		"first name": {`{"msg":"x","who":"` + strings.ToUpper(m.FirstName) + `"}`, season.LeakFirstName},
	}
	for name, p := range plants {
		t.Run(name, func(t *testing.T) {
			leaks := season.Search(append(append([]byte(nil), lg...), p.line...), res.Cast)
			if len(leaks) == 0 {
				t.Fatalf("a planted %s was not found", name)
			}
			for _, l := range leaks {
				if l.Member != m.Index {
					t.Errorf("found a leak of member %d, the planted one is %d: %v", l.Member, m.Index, l)
				}
			}
			if !hasKind(leaks, p.kind) {
				t.Errorf("leaks %v do not include the planted kind %v", leaks, p.kind)
			}
		})
	}
}

func hasKind(leaks []season.Leak, k season.LeakKind) bool {
	for _, l := range leaks {
		if l.Kind == k {
			return true
		}
	}
	return false
}

// The same, through a whole run: a line logged by the run's own logger, partway
// through, is in what the run reports as leaked. This is the path that would
// catch a package logging a person.
func TestANameLoggedDuringTheRunMakesTheRunReportTheLeak(t *testing.T) {
	res, _ := run(t, season.Options{Plant: func(l *slog.Logger, cast []season.Member) {
		l.Info("planted", "who", cast[5].Name)
	}})
	// A full name holds the first name, so both are found; nothing else is, and
	// nobody else.
	if len(res.Leaks) != 2 || !hasKind(res.Leaks, season.LeakName) || !hasKind(res.Leaks, season.LeakFirstName) {
		t.Fatalf("leaks = %v, want the planted name of member 5", res.Leaks)
	}
	for _, l := range res.Leaks {
		if l.Member != 5 {
			t.Errorf("leak %v is not the planted member's", l)
		}
	}
	if len(res.Violations) != 0 {
		t.Errorf("the plant broke an invariant too: %v", res.Violations)
	}
}

// The cast is invented: no real people, no real domains.
func TestTheCastIsInventedPeopleOnExampleDotCom(t *testing.T) {
	res, _ := theSeason(t)
	seen := map[string]bool{}
	for _, m := range res.Cast {
		if !strings.HasSuffix(m.Email, "@example.com") {
			t.Errorf("member %d's email %q is not on example.com", m.Index, m.Email)
		}
		if seen[m.Email] || seen[m.Name] {
			t.Errorf("member %d repeats an email or a name", m.Index)
		}
		seen[m.Email], seen[m.Name] = true, true
	}
}
