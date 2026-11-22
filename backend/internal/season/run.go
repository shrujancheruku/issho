// Package season is the season: a deterministic run of scripted people and
// plans over two weeks, through the real packages on the fake clock, that ends
// by checking the rules the packages keep between them (docs/spec/quality.md,
// "The season"). It grows in each later milestone.
//
// It uses only the packages' exported APIs (tools/boundaries enforces it): it
// reads nothing from their tables. What it knows of the world it was told by
// them, and the invariants are checked on that (World).
package season

import (
	"bytes"
	"container/heap"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/fixtures"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/jobs"
	"github.com/shrujancheruku/issho/backend/internal/logging"
	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/policy"
	"github.com/shrujancheruku/issho/backend/internal/staff"
)

// The fixture city's two plans (fixtures/da-nang-hoi-an.json) and the day they
// open: the season's first day, and the first of its rounds.
const (
	city     = "da-nang-hoi-an"
	lanterns = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0001" // evening
	sunrise  = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0002" // morning
	opensOn  = "2026-01-05"
	rounds   = 2
)

var planParts = map[string]string{lanterns: "evening", sunrise: "morning"}

// seed fixes the draws (who answers, who cancels, which dates a person ticks):
// the same seed is the same season.
const seed = 20260105

// Options changes a run. The zero value is the season CI runs.
type Options struct {
	// Plant, when set, is called once, partway through the run, with the run's
	// logger and the cast: a test logs a member's detail through it to prove
	// the log search sees what the run logs.
	Plant func(l *slog.Logger, cast []Member)
}

// Result is everything a run reports.
type Result struct {
	Cast []Member
	// Outings are the outings the run formed, as they stood at its end, by plan,
	// date and members: no id, so two runs can be compared.
	Outings   []OutingRecord
	Scorecard Scorecard
	// Stages and Refusals count what happened and what the packages refused, by
	// name; a season in which one never happens is not testing it.
	Stages, Refusals map[string]int
	// Checked counts what the invariants looked at.
	Checked map[string]int
	// Violations are the broken invariants, empty when the season is sound.
	Violations []string
	// Log is the run's own log output, and Leaks what in it is a cast member's
	// email or name.
	Log   []byte
	Leaks []Leak
	// FirstSignup and LastSignup bound the signups, on the fake clock.
	FirstSignup, LastSignup time.Time
}

// OutingRecord is an outing at the season's end.
type OutingRecord struct {
	Plan, Date, Part string
	Languages        []string
	State            outings.OutingState
	Seats            []SeatRecord
}

// SeatRecord is a member's seat: who, where it stands and whether they checked in.
type SeatRecord struct {
	Member    int
	State     outings.SeatState
	CheckedIn bool
}

// Scorecard is the season's numbers, the ones a change to a number in policy
// is tried against (docs/spec/quality.md, "The season"). A signup is a
// member's signup to a plan in a round.
type Scorecard struct {
	Signups     int // signups made
	Grouped     int // of them, ones whose person was invited to an outing
	GotAnOuting int // of them, ones whose person went to one: was going at some point
	People      int // members who signed up
	PeopleNone  int // of them, members who never got an outing
}

func (s Scorecard) String() string {
	pct := func(n int) string {
		if s.Signups == 0 {
			return "0%"
		}
		return fmt.Sprintf("%d%%", (n*100+s.Signups/2)/s.Signups)
	}
	return fmt.Sprintf("signups: %d\ngrouped: %d (%s)\ngot an outing: %d (%s)\npeople who signed up: %d\npeople who got none: %d\n",
		s.Signups, s.Grouped, pct(s.Grouped), s.GotAnOuting, pct(s.GotAnOuting), s.People, s.PeopleNone)
}

// lockedBuffer is where the run's logger and River's goroutines write.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) bytes() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]byte(nil), l.b.Bytes()...)
}

// Run plays the season on the empty database behind pool, which it migrates
// and fills with the fixture city. It returns an error only when a package
// fails when it should not; a rule broken is in Result.Violations.
func Run(ctx context.Context, pool *pgxpool.Pool, opts Options) (Result, error) {
	if err := db.Up(ctx, pool); err != nil {
		return Result{}, fmt.Errorf("season: migrate: %w", err)
	}
	if err := cities.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		return Result{}, fmt.Errorf("season: city: %w", err)
	}
	if err := plans.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		return Result{}, fmt.Errorf("season: plans: %w", err)
	}
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return Result{}, fmt.Errorf("season: city zone: %w", err)
	}
	open, _ := time.Parse(time.DateOnly, opensOn)

	buf := &lockedBuffer{}
	lg := logging.New(buf, "season")
	// 09:00 UTC is 16:00 in the city, on the day the plans open.
	clk := clock.NewFake(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)) //policylint:ignore a scripted number of the season, not that rule
	cs := cities.New(pool)
	ppl := people.New(clk, pool, cs, people.Config{DefaultCity: city})
	signups := plans.NewSignups(clk, pool, cs)
	svc := outings.New(clk, pool, signups, ppl)

	// River's client logs through the same logger as the run, so what it says is
	// searched too. Its queue is paused: its jobs are timed on the real clock,
	// which would race the fake one, so the run does what each job does (Expire)
	// at the fake time it is due.
	workers := jobs.NewWorkers(clk, pool, staff.NewKinds())
	outings.AddWorker(workers, svc)
	client, err := jobs.New(pool, workers, lg)
	if err != nil {
		return Result{}, fmt.Errorf("season: river: %w", err)
	}
	if err := client.Start(ctx); err != nil {
		return Result{}, fmt.Errorf("season: river start: %w", err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = client.Stop(context.WithoutCancel(ctx))
		}
	}()
	if err := client.QueuePause(ctx, river.QueueDefault, nil); err != nil {
		return Result{}, fmt.Errorf("season: river pause: %w", err)
	}

	r := &runner{
		ctx: ctx, loc: loc, open: open, clk: clk, log: lg, ppl: ppl, signups: signups, svc: svc, pool: pool,
		cast: castOf(), rng: rand.New(rand.NewPCG(seed, seed)),
		stages: map[string]int{}, refusals: map[string]int{}, checked: map[string]int{},
		seats: map[seatKey]*seatInfo{}, outingInfo: map[string]*outingInfo{}, signed: map[signedKey]*signedInfo{},
		codes: map[int]string{}, spoken: map[int][]string{},
	}
	for _, m := range r.cast {
		r.spoken[m.Index] = m.Languages
	}
	if err := r.script(opts); err != nil {
		return Result{}, err
	}
	for r.events.Len() > 0 {
		e := heap.Pop(&r.events).(*event)
		clk.Set(e.at)
		if err := e.run(); err != nil {
			return Result{}, fmt.Errorf("season: %s at %s: %w", e.name, e.at.In(loc).Format("2006-01-02 15:04"), err)
		}
	}
	if err := r.observe(); err != nil {
		return Result{}, err
	}
	res, err := r.finish()
	if err != nil {
		return Result{}, err
	}
	if err := client.Stop(ctx); err != nil {
		return Result{}, fmt.Errorf("season: river stop: %w", err)
	}
	stopped = true
	res.Log = buf.bytes()
	res.Leaks = Search(res.Log, r.cast)
	return res, nil
}

// event is something the script does at a fake time. Events at one time run in
// the order they were made.
type event struct {
	at   time.Time
	seq  int
	name string
	run  func() error
}

type eventQueue []*event

func (q eventQueue) Len() int { return len(q) }
func (q eventQueue) Less(i, j int) bool {
	if !q[i].at.Equal(q[j].at) {
		return q[i].at.Before(q[j].at)
	}
	return q[i].seq < q[j].seq
}
func (q eventQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *eventQueue) Push(x any)   { *q = append(*q, x.(*event)) }
func (q *eventQueue) Pop() any {
	old := *q
	e := old[len(old)-1]
	*q = old[:len(old)-1]
	return e
}

type seatKey struct {
	outing string
	member int
}

// seatInfo is what the run decided and saw of one seat.
type seatInfo struct {
	invited, going, scheduled bool
	checkedIn, said           bool // said: told feedback they went
	wasGoing                  bool
	ticked                    bool
}

// outingInfo is what the run saw of one outing.
type outingInfo struct {
	plan, date, part string
	seenSet          bool
	expireScheduled  bool
}

type signedKey struct {
	member int
	plan   string
	round  int
}

type signedInfo struct {
	ticks         []plans.Tick
	grouped, went bool
	seated        bool
}

type runner struct {
	ctx     context.Context
	loc     *time.Location
	open    time.Time
	clk     *clock.Fake
	log     *slog.Logger
	pool    *pgxpool.Pool
	ppl     *people.Service
	signups *plans.Signups
	svc     *outings.Service
	cast    []Member
	rng     *rand.Rand

	events eventQueue
	seq    int

	stages, refusals, checked map[string]int
	seats                     map[seatKey]*seatInfo
	outingInfo                map[string]*outingInfo
	signed                    map[signedKey]*signedInfo
	codes                     map[int]string // the join link code each member redeemed
	spoken                    map[int][]string

	world       World
	firstSignup time.Time
	lastSignup  time.Time
}

func (r *runner) at(t time.Time, name string, run func() error) {
	r.seq++
	heap.Push(&r.events, &event{at: t, seq: r.seq, name: name, run: run})
}

// local is a time on the city's clock on January day.
func (r *runner) local(day, hour, min int) time.Time {
	return time.Date(2026, 1, day, hour, min, 0, 0, r.loc)
}

func (r *runner) cityDate(t time.Time) string { return t.In(r.loc).Format(time.DateOnly) }

// roundOf is the number of the round that holds the city date.
func (r *runner) roundOf(date string) int {
	d, _ := time.Parse(time.DateOnly, date)
	return int(d.Sub(r.open) / (24 * time.Hour) / 7)
}

func (r *runner) roundEnd(round int) time.Time { return r.open.AddDate(0, 0, round*7+6) } //policylint:ignore a scripted number of the season, not that rule

func (r *runner) stage(name string) {
	r.stages[name]++
}

// script lays out the whole run's fixed events: who joins when, who signs up
// when, and each day's forming. The rest (answers, cancels, check-ins,
// feedback) are made as the packages' answers call for them.
func (r *runner) script(opts Options) error {
	// Members join in turn on the first evening, each brought in by an
	// earlier member.
	for _, m := range r.cast {
		m := m
		r.at(r.local(5, 16, 5).Add(time.Duration(m.Index)*20*time.Minute), "join", func() error { return r.join(m) }) //policylint:ignore a scripted number of the season, not that rule
	}
	// Signups: each round, each member signs up to their plans on their own day
	// of the first five days, at night.
	for round := 0; round < rounds; round++ {
		for _, m := range r.cast {
			m := m
			day := 5 + round*7 + m.Index%5 //policylint:ignore a scripted number of the season, not that rule
			if round > 0 {
				// The second round's signups come early, so its early forming overlaps the first's outings.
				day = 5 + round*7 + m.Index%2 //policylint:ignore a scripted number of the season, not that rule
			}
			for _, plan := range []string{lanterns, sunrise} {
				plan := plan
				if m.plans&planBit(plan) == 0 {
					continue
				}
				// Not everyone signs up again for the second round.
				if round > 0 && m.Index%6 == 5 { //policylint:ignore a scripted number of the season, not that rule
					continue
				}
				r.at(r.local(day, 23, m.Index), "sign up", func() error { return r.signUp(m, plan) })
			}
		}
	}
	// The forming, each morning from the second day to the last day of the
	// second round: the spec's rules decide whether each plan forms that day.
	for day := 6; day <= 5+rounds*7-1; day++ { //policylint:ignore a scripted number of the season, not that rule
		day := day
		r.at(r.local(day, 8, 0), "form", func() error { return r.formDay(day) })
	}
	r.at(r.local(6, 7, 0), "fourth join link", r.fourthLink) //policylint:ignore a scripted number of the season, not that rule
	if opts.Plant != nil {
		r.at(r.local(10, 12, 0), "plant", func() error { opts.Plant(r.log, r.cast); return nil })
	}
	return nil
}

func planBit(plan string) int {
	if plan == lanterns {
		return 1
	}
	return 2
}

// join is a member's first evening: the account, the join link that admits it
// (or the seed, for the founder), and the profile. The three who are never
// admitted try what the packages must refuse.
func (r *runner) join(m Member) error {
	ctx := r.ctx
	if _, err := r.ppl.Join(ctx, people.Claims{Subject: m.ID, Email: m.Email}, people.Preferences{}); err != nil {
		return fmt.Errorf("join: %w", err)
	}
	r.log.Info("account made", "account", m.ID)
	switch {
	case m.Inviter == -1:
		if err := people.AdmitByEmail(ctx, r.pool, m.Email, r.clk.Now()); err != nil {
			return fmt.Errorf("the seed's admit: %w", err)
		}
		r.stage("admitted by the seed")
	case m.Admitted:
		code, err := r.takeCode(m.Inviter)
		if err != nil {
			return err
		}
		if err := r.ppl.RedeemJoinLink(ctx, m.ID, code); err != nil {
			return fmt.Errorf("redeem: %w", err)
		}
		r.codes[m.Index] = code
		r.stage("join link redeemed")
		r.log.Info("join link redeemed", "account", m.ID)
	default:
		// Not admitted: one tries a code that is nobody's, another the code a
		// friend has already used. Neither gets in.
		code := "no-such-code"
		if m.Index == 18 { //policylint:ignore a scripted number of the season, not that rule
			code = r.codes[16]
		}
		if m.Index != 19 {
			if err := r.ppl.RedeemJoinLink(ctx, m.ID, code); !errors.Is(err, people.ErrJoinLinkUnusable) {
				return fmt.Errorf("a code that admits nobody: got %v, want %v", err, people.ErrJoinLinkUnusable)
			}
			if m.Index == 18 { //policylint:ignore a scripted number of the season, not that rule
				r.refusals["used join link"]++
			} else {
				r.refusals["made-up join link"]++
			}
		}
	}
	if _, err := r.ppl.SaveProfile(ctx, m.ID, people.ProfileInput{
		Name: m.Name, DateOfBirth: "1990-06-15", Gender: m.Gender, Languages: m.Languages,
		LivesInCity: m.Index%3 != 0, AcceptedCodeOfConduct: true, OnlyWomenAndNonBinary: m.WomenOnly, //policylint:ignore a scripted number of the season, not that rule
	}); err != nil {
		return fmt.Errorf("profile: %w", err)
	}
	r.stage("profile saved")
	return nil
}

// takeCode makes the inviter's next join link and returns its code: the
// inviter hands a link to the person they bring in.
func (r *runner) takeCode(inviter int) (string, error) {
	l, err := r.ppl.MakeJoinLink(r.ctx, r.cast[inviter].ID)
	if err != nil {
		return "", fmt.Errorf("make a join link: %w", err)
	}
	r.stage("join link made")
	return l.Code, nil
}

// fourthLink is the founder asking for a fourth link after the three they
// handed out and which were used: a member has three in all, and a used one
// does not return.
func (r *runner) fourthLink() error {
	if _, err := r.ppl.MakeJoinLink(r.ctx, r.cast[0].ID); !errors.Is(err, people.ErrNoJoinLinksLeft) {
		return fmt.Errorf("a fourth join link: got %v, want %v", err, people.ErrNoJoinLinksLeft)
	}
	r.refusals["fourth join link"]++
	return nil
}

// signUp is a member signing up to a plan in the round of the day, as the API
// does it: admitted first, then a profile, then the ticks. Someone not admitted
// is refused and has no signup.
func (r *runner) signUp(m Member, plan string) error {
	ctx := r.ctx
	now := r.clk.Now()
	if err := r.ppl.RequireAdmitted(ctx, m.ID); err != nil {
		if !errors.Is(err, people.ErrNotAdmitted) || m.Admitted {
			return fmt.Errorf("admitted check: %w", err)
		}
		r.refusals["not admitted"]++
		return nil
	}
	if err := r.ppl.RequireProfile(ctx, m.ID); err != nil {
		return fmt.Errorf("profile check: %w", err)
	}
	today := r.cityDate(now)
	round := r.roundOf(today)
	ticks, err := r.pickTicks(m, plan, today, round)
	if err != nil {
		return err
	}
	if _, err := r.signups.Set(ctx, m.ID, plan, ticks); err != nil {
		return fmt.Errorf("signup: %w", err)
	}
	r.stage("signed up")
	r.log.Info("signed up", "account", m.ID, "round", round, "ticks", len(ticks))
	if r.firstSignup.IsZero() {
		r.firstSignup = now
	}
	r.lastSignup = now
	k := signedKey{m.Index, plan, round}
	if r.signed[k] == nil {
		r.signed[k] = &signedInfo{}
	}
	r.signed[k].ticks = ticks
	return nil
}

// pickTicks chooses three to five dates for the plan's one part of the day, from
// the earliest allowed (3 days ahead) to a round beyond the round's last day.
// In the second round a member sometimes ticks a time they are already going
// to, the way a person ticks what suits them without remembering their other
// plans: the rule that they can't be double-booked is the packages'.
func (r *runner) pickTicks(m Member, plan, today string, round int) ([]plans.Tick, error) {
	d, _ := time.Parse(time.DateOnly, today)
	last := r.roundEnd(round).AddDate(0, 0, 7)
	var dates []string
	for day := d.AddDate(0, 0, policy.MinTickLeadDays); !day.After(last); day = day.AddDate(0, 0, 1) {
		dates = append(dates, day.Format(time.DateOnly))
	}
	r.rng.Shuffle(len(dates), func(i, j int) { dates[i], dates[j] = dates[j], dates[i] })
	// The dates a person is free are close together, as a week's are.
	n := 3 + r.rng.IntN(3) //policylint:ignore a scripted number of the season, not that rule
	if len(dates) > 7 {
		dates = dates[:7]
	}
	if n > len(dates) {
		n = len(dates)
	}
	dates = dates[:n]
	if round > 0 {
		seats, err := r.svc.Mine(r.ctx, m.ID)
		if err != nil {
			return nil, fmt.Errorf("mine: %w", err)
		}
		for _, s := range seats {
			held := s.State == outings.SeatGoing || s.State == outings.SeatAccepted || s.State == outings.SeatInvited
			if held && s.PlanID == plan && s.Date >= d.AddDate(0, 0, policy.MinTickLeadDays).Format(time.DateOnly) &&
				!slices.Contains(dates, s.Date) {
				dates[0] = s.Date
			}
		}
	}
	ticks := make([]plans.Tick, len(dates))
	for i, date := range dates {
		ticks[i] = plans.Tick{Date: date, Part: planParts[plan]}
	}
	return ticks, nil
}

// formDay is a morning's forming. For each plan, the spec's rule: the round's
// last day forms from any three or more, and any earlier day only when 5 people
// who can be grouped (one language between them) share a time. The season calls
// the package's Form itself, since forming isn't scheduled yet.
func (r *runner) formDay(day int) error {
	today := r.cityDate(r.clk.Now())
	round := r.roundOf(today)
	for _, plan := range []string{lanterns, sunrise} {
		if today != r.roundEnd(round).Format(time.DateOnly) && !r.fiveShareATime(plan, today, round) {
			continue
		}
		n, err := r.svc.Form(r.ctx, plan)
		if err != nil {
			return fmt.Errorf("form: %w", err)
		}
		r.stages["formed"] += n
		r.log.Info("formed", "day", today, "outings", n)
		if err := r.observe(); err != nil {
			return err
		}
	}
	return nil
}

// fiveShareATime: five members with no seat yet in the round, who speak one
// language between them, tick one date (still 3 days ahead) in the plan.
func (r *runner) fiveShareATime(plan, today string, round int) bool {
	d, _ := time.Parse(time.DateOnly, today)
	from := d.AddDate(0, 0, policy.MinTickLeadDays).Format(time.DateOnly)
	type key struct{ date, lang string }
	count := map[key]int{}
	for k, s := range r.signed {
		if k.plan != plan || k.round != round || s.seated {
			continue
		}
		for _, t := range s.ticks {
			if t.Date < from {
				continue
			}
			for _, l := range r.spoken[k.member] {
				count[key{t.Date, l}]++
			}
		}
	}
	for _, n := range count {
		if n >= 5 { //policylint:ignore a scripted number of the season, not that rule
			return true
		}
	}
	return false
}

// observe reads every member's seats through the outings package and reacts to
// what is new: an invitation (the member answers, or doesn't, and the invitations
// end), an outing newly set, a person newly going (they will cancel, not come
// or check in, then answer feedback). It is also the double-booking sweep.
func (r *runner) observe() error {
	type mine struct {
		member int
		seat   outings.Seat
	}
	byOuting := map[string][]mine{}
	var order []string
	for _, m := range r.cast {
		seats, err := r.svc.Mine(r.ctx, m.ID)
		if err != nil {
			return fmt.Errorf("mine: %w", err)
		}
		for _, s := range seats {
			if _, ok := byOuting[s.OutingID]; !ok {
				order = append(order, s.OutingID)
			}
			byOuting[s.OutingID] = append(byOuting[s.OutingID], mine{m.Index, s})
		}
	}
	// By what the outing is, never its id (a random uuid), so the draws below
	// come in the same order in every run.
	for _, g := range byOuting {
		sort.Slice(g, func(i, j int) bool { return g[i].member < g[j].member })
	}
	sig := func(g []mine) string {
		s := ""
		for _, m := range g {
			s += fmt.Sprintf("%02d%s,", m.member, m.seat.State)
		}
		return s
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := byOuting[order[i]], byOuting[order[j]]
		switch {
		case a[0].seat.Date != b[0].seat.Date:
			return a[0].seat.Date < b[0].seat.Date
		case a[0].seat.Part != b[0].seat.Part:
			return a[0].seat.Part < b[0].seat.Part
		case a[0].seat.PlanID != b[0].seat.PlanID:
			return a[0].seat.PlanID < b[0].seat.PlanID
		}
		return sig(a) < sig(b)
	})
	r.checked["double-booking sweeps"]++
	for _, id := range order {
		group := byOuting[id]
		first := group[0].seat
		oi := r.outingInfo[id]
		if oi == nil {
			oi = &outingInfo{plan: first.PlanID, date: first.Date, part: first.Part}
			r.outingInfo[id] = oi
		}
		for _, g := range group {
			k := seatKey{id, g.member}
			si := r.seats[k]
			if si == nil {
				si = &seatInfo{}
				r.seats[k] = si
			}
			if !si.invited {
				si.invited = true
				if err := r.invited(g.member, g.seat, si); err != nil {
					return err
				}
			}
			if g.seat.State == outings.SeatGoing && !si.going {
				si.going = true
				r.world.Going = append(r.world.Going, GoingSeat{Member: g.member, Outing: id, Date: first.Date, Part: first.Part})
				if err := r.plansFor(g.member, g.seat, si); err != nil {
					return err
				}
			}
		}
		if first.OutingState == outings.OutingSet && !oi.seenSet {
			oi.seenSet = true
			r.stage("set")
			so := SetOuting{ID: id, Date: first.Date, Part: first.Part, Languages: first.Languages}
			for _, g := range group {
				if g.seat.State == outings.SeatGoing {
					so.Members = append(so.Members, SetMember{Member: g.member, Spoken: r.spoken[g.member], Ticked: r.seats[seatKey{id, g.member}].ticked})
				}
			}
			r.world.SetOutings = append(r.world.SetOutings, so)
			r.checked["set outings"]++
		}
	}
	return nil
}

// invited is a member's new invitation: noted against their signup, the tick
// the group is formed on is looked for in their signup, and their answer is
// drawn. The outing's unanswered seats are closed when the 48 hours end.
func (r *runner) invited(member int, s outings.Seat, si *seatInfo) error {
	m := r.cast[member]
	mine, err := r.signups.Mine(r.ctx, m.ID)
	if err != nil {
		return fmt.Errorf("signups: %w", err)
	}
	for _, sg := range mine {
		if sg.PlanID == s.PlanID && slices.Contains(sg.Ticks, plans.Tick{Date: s.Date, Part: s.Part}) {
			si.ticked = true
		}
	}
	round := r.roundOf(r.cityDate(s.InvitedAt))
	if sg := r.signed[signedKey{member, s.PlanID, round}]; sg != nil {
		sg.grouped, sg.seated = true, true
	}
	oi := r.outingInfo[s.OutingID]
	if !oi.expireScheduled {
		oi.expireScheduled = true
		id := s.OutingID
		r.at(s.InvitationEndsAt, "expire", func() error {
			if err := r.svc.Expire(r.ctx, id); err != nil {
				return err
			}
			r.stage("expired")
			return r.observe()
		})
	}
	draw := r.rng.Float64()
	delay := time.Hour + time.Duration(r.rng.IntN(38*60))*time.Minute
	var a outings.Answer
	switch {
	case draw < 0.78:
		a = outings.Yes
	case draw < 0.9:
		a = outings.No
	default:
		return nil // never answers
	}
	id := s.OutingID
	r.at(s.InvitedAt.Add(delay), "answer", func() error { return r.answer(member, id, a) })
	return nil
}

func (r *runner) answer(member int, outing string, a outings.Answer) error {
	_, err := r.svc.Answer(r.ctx, r.cast[member].ID, outing, a)
	switch {
	case errors.Is(err, outings.ErrTimeTaken):
		r.refusals["yes at a taken time"]++
	case errors.Is(err, outings.ErrInvitationClosed):
		r.refusals["answer after it closed"]++
	case err != nil:
		return fmt.Errorf("answer: %w", err)
	default:
		r.stage(string(a))
		r.log.Info("answered", "account", r.cast[member].ID, "answer", string(a))
	}
	return r.observe()
}

// plansFor is what a person newly going will do: one in eight cancels the day
// before, one in ten doesn't come, the rest check in; everyone who was going at
// the start answers feedback once it opens.
func (r *runner) plansFor(member int, s outings.Seat, si *seatInfo) error {
	if si.scheduled {
		return nil
	}
	si.scheduled = true
	id := s.OutingID
	acct := r.cast[member].ID
	draw := r.rng.Float64()
	if draw < 0.125 {
		date, _ := time.ParseInLocation(time.DateOnly, s.Date, r.loc)
		at := date.AddDate(0, 0, -1).Add(12 * time.Hour)
		if !at.After(r.clk.Now()) {
			at = r.clk.Now().Add(time.Hour)
		}
		r.at(at, "cancel", func() error {
			if _, err := r.svc.Cancel(r.ctx, acct, id); err != nil {
				return fmt.Errorf("cancel: %w", err)
			}
			r.stage("cancelled")
			return nil
		})
		return nil
	}
	si.wasGoing = true
	noShow := draw < 0.225
	if !noShow {
		at := s.CheckInOpensAt.Add(time.Duration(5+r.rng.IntN(45)) * time.Minute) //policylint:ignore a scripted number of the season, not that rule
		r.at(at, "check in", func() error {
			if _, err := r.svc.CheckIn(r.ctx, acct, id); err != nil {
				return fmt.Errorf("check in: %w", err)
			}
			si.checkedIn = true
			r.stage("checked in")
			return nil
		})
	} else {
		r.at(s.CheckInClosesAt.Add(-time.Minute), "no show", func() error {
			r.stage("no show")
			return nil
		})
	}
	// A no-show says they went one time in three: their word is theirs.
	saysWent := !noShow || r.rng.IntN(3) == 0 //policylint:ignore a scripted number of the season, not that rule
	meetAgain := r.rng.IntN(2) == 0
	at := s.FeedbackOpensAt.Add(time.Hour + time.Duration(r.rng.IntN(40*60))*time.Minute) //policylint:ignore a scripted number of the season, not that rule
	r.at(at, "feedback", func() error {
		form, err := r.svc.Feedback(r.ctx, acct, id)
		if err != nil {
			return fmt.Errorf("feedback: %w", err)
		}
		in := outings.FeedbackInput{Went: saysWent}
		if saysWent && meetAgain && len(form.Others) > 0 {
			in.MeetAgain = []string{form.Others[0].Handle}
		}
		if _, err := r.svc.SaveFeedback(r.ctx, acct, id, in); err != nil {
			return fmt.Errorf("save feedback: %w", err)
		}
		r.stage("feedback saved")
		si.said = saysWent
		got, err := r.svc.Attended(r.ctx, acct, id)
		if err != nil {
			return fmt.Errorf("attended: %w", err)
		}
		r.world.Attendance = append(r.world.Attendance, AttendanceFact{Member: member, Want: saysWent || si.checkedIn, Got: got})
		return nil
	})
	return nil
}

// finish reads the end state: the outings as they stand, the signups held, the
// scorecard, and the invariants on all of it.
func (r *runner) finish() (Result, error) {
	res := Result{Cast: r.cast, Stages: r.stages, Refusals: r.refusals, Checked: r.checked, FirstSignup: r.firstSignup, LastSignup: r.lastSignup}
	byOuting := map[string]*OutingRecord{}
	for _, m := range r.cast {
		seats, err := r.svc.Mine(r.ctx, m.ID)
		if err != nil {
			return Result{}, fmt.Errorf("mine: %w", err)
		}
		for _, s := range seats {
			o := byOuting[s.OutingID]
			if o == nil {
				name := "lanterns"
				if s.PlanID == sunrise {
					name = "sunrise"
				}
				o = &OutingRecord{Plan: name, Date: s.Date, Part: s.Part, Languages: s.Languages, State: s.OutingState}
				byOuting[s.OutingID] = o
			}
			o.Seats = append(o.Seats, SeatRecord{Member: m.Index, State: s.State, CheckedIn: s.CheckedIn})
			if s.State == outings.SeatCancelled {
				for i := range r.world.Going {
					if g := &r.world.Going[i]; g.Member == m.Index && g.Outing == s.OutingID {
						g.Cancelled = true
					}
				}
			}
			if s.State == outings.SeatGoing || s.State == outings.SeatCancelled {
				round := r.roundOf(r.cityDate(s.InvitedAt))
				if sg := r.signed[signedKey{m.Index, s.PlanID, round}]; sg != nil {
					sg.went = true
				}
			}
		}
		held, err := r.signups.Mine(r.ctx, m.ID)
		if err != nil {
			return Result{}, fmt.Errorf("signups: %w", err)
		}
		admitted, err := r.ppl.Admitted(r.ctx, m.ID)
		if err != nil {
			return Result{}, fmt.Errorf("admitted: %w", err)
		}
		if !admitted {
			r.checked["not admitted"]++
		}
		if len(held) > 0 {
			r.world.Holders = append(r.world.Holders, Holder{Member: m.Index, Admitted: admitted})
		}
	}
	for _, o := range byOuting {
		res.Outings = append(res.Outings, *o)
	}
	for _, o := range res.Outings {
		sort.Slice(o.Seats, func(i, j int) bool { return o.Seats[i].Member < o.Seats[j].Member })
	}
	sig := func(o OutingRecord) string {
		s := ""
		for _, st := range o.Seats {
			s += fmt.Sprintf("%02d%s,", st.Member, st.State)
		}
		return s
	}
	sort.Slice(res.Outings, func(i, j int) bool {
		a, b := res.Outings[i], res.Outings[j]
		switch {
		case a.Date != b.Date:
			return a.Date < b.Date
		case a.Part != b.Part:
			return a.Part < b.Part
		case a.Plan != b.Plan:
			return a.Plan < b.Plan
		}
		return sig(a) < sig(b)
	})
	people := map[int]bool{}
	wentPeople := map[int]bool{}
	for k, s := range r.signed {
		res.Scorecard.Signups++
		people[k.member] = true
		if s.grouped {
			res.Scorecard.Grouped++
		}
		if s.went {
			res.Scorecard.GotAnOuting++
			wentPeople[k.member] = true
		}
	}
	res.Scorecard.People = len(people)
	res.Scorecard.PeopleNone = len(people) - len(wentPeople)
	res.Violations = CheckInvariants(r.world)
	return res, nil
}
