// Package bench compares walk proposers (models, efforts, prompts) on a
// fixed set of cases, through the production composer (walks.Composer), so
// the numbers measure what users would get.
//
// A run writes, per (case, variant, rep):
//
//	results.jsonl  one row per attempt that produced a walk or a rejection
//	errors.jsonl   attempts that failed before a verdict (API errors, timeouts)
//	traces/…json   every proposal, what was dropped and why, the final walk
//
// Runs resume: rows already in results.jsonl are skipped. Judge (judge.go)
// compares composed walks pairwise; Report (report.go) summarises.
//
// Only model-written text and Google place ids are written: nothing from
// Google (names, addresses) is stored (Maps terms).
package bench

import (
	"bufio"
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// Case is one walk to compose: an activity in an area, in a month.
type Case struct {
	ID       string   `json:"id"`
	City     string   `json:"city"`     // city pack id
	Area     string   `json:"area"`     // area id
	Activity string   `json:"activity"` // a walking activity's id
	Month    int      `json:"month"`
	Blocks   []string `json:"blocks,omitempty"` // default: the activity's
	// Tags: theme, city, then how well known the area is for the theme
	// (known, mid, quiet, hard). Tags[0] groups the report.
	Tags []string `json:"tags"`
}

//go:embed cases.json
var casesJSON []byte

// Cases is the benchmark's case set.
func Cases() ([]Case, error) {
	var cs []Case
	if err := json.Unmarshal(casesJSON, &cs); err != nil {
		return nil, fmt.Errorf("cases.json: %w", err)
	}
	return cs, nil
}

// Requests turns cases into composer requests from the catalogue and city
// packs (dataDir is the repo's data/ directory). activities overrides the
// catalogue file (e.g. to try rewritten themes); "" = dataDir's.
func Requests(cs []Case, dataDir, activities string) (map[string]walks.Request, error) {
	acts, err := cities.ReadActivities(cmp.Or(activities, filepath.Join(dataDir, "activities.json")))
	if err != nil {
		return nil, err
	}
	byID := map[string]cities.Activity{}
	for _, a := range acts {
		byID[a.ID] = a
	}
	packs := map[string]cities.Pack{}
	out := map[string]walks.Request{}
	for _, c := range cs {
		p, ok := packs[c.City]
		if !ok {
			if p, err = cities.ReadPack(filepath.Join(dataDir, "citypacks", c.City+".json")); err != nil {
				return nil, err
			}
			packs[c.City] = p
		}
		a, ok := byID[c.Activity]
		if !ok || a.WalkTheme == "" {
			return nil, fmt.Errorf("case %s: %q is not a walking activity", c.ID, c.Activity)
		}
		i := slices.IndexFunc(p.Areas, func(ar cities.Area) bool { return ar.ID == c.Area })
		if i < 0 {
			return nil, fmt.Errorf("case %s: no area %q in %s", c.ID, c.Area, c.City)
		}
		if c.Month < 1 || c.Month > 12 {
			return nil, fmt.Errorf("case %s: month %d", c.ID, c.Month)
		}
		ar := p.Areas[i]
		// The city's own name for the activity, if it has one; a case must
		// be one the worker could open (enabled, in season).
		name := a.Name["en"]
		for _, o := range p.ActivityOverrides {
			if o.ActivityID != c.Activity {
				continue
			}
			if o.Disabled {
				return nil, fmt.Errorf("case %s: %s is disabled in %s", c.ID, c.Activity, c.City)
			}
			if len(o.Months) > 0 && !slices.Contains(o.Months, int16(c.Month)) {
				return nil, fmt.Errorf("case %s: %s is out of season in month %d", c.ID, c.Activity, c.Month)
			}
			name = cmp.Or(o.Name["en"], name)
		}
		r := walks.Request{
			City: p.Name["en"], Area: ar.Name["en"], Center: maps.LatLng{Lat: ar.Lat, Lng: ar.Lng},
			Activity: plans.WithoutExamples(name), Theme: a.WalkTheme, Month: time.Month(c.Month),
			Blocks: c.Blocks,
		}
		if len(r.Blocks) == 0 {
			r.Blocks = a.Blocks
		}
		out[c.ID] = r
	}
	return out, nil
}

// Variant is a proposer configuration: "provider:model[:effort]", e.g.
// "claude:claude-opus-5-5:high" (the API), "claudecode:claude-opus-5-5:high"
// (the Claude Code CLI on a subscription) or "gemini:gemini-3.8-flash:MEDIUM".
type Variant struct {
	Name, Provider, Model, Effort string
}

// ParseVariant reads "provider:model[:effort]".
func ParseVariant(s string) (Variant, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 || parts[1] == "" {
		return Variant{}, fmt.Errorf("variant %q: want provider:model[:effort]", s)
	}
	v := Variant{Name: s, Provider: parts[0], Model: parts[1]}
	if len(parts) == 3 {
		v.Effort = parts[2]
	}
	if !slices.Contains([]string{"claude", "claudecode", "gemini"}, v.Provider) {
		return Variant{}, fmt.Errorf("variant %q: provider must be claude, claudecode or gemini", s)
	}
	return v, nil
}

// Stop is one stop of a composed walk as stored: the model's text and
// Google's place id, nothing from Google.
type Stop struct {
	Role    string `json:"role"`
	Name    string `json:"name"` // the model's name
	Kind    string `json:"kind"`
	PlaceID string `json:"place_id"`
	Minutes int    `json:"minutes"`
	Note    string `json:"note"`
	// WalkMin is the walk here from the previous stop (Routes), for our
	// metrics only; it's never shown to a judge.
	WalkMin float64 `json:"walk_min"`
}

// WalkOut is a composed walk.
type WalkOut struct {
	Title     string  `json:"title"`
	Summary   string  `json:"summary"`
	Stops     []Stop  `json:"stops"`
	Minutes   int     `json:"minutes"` // at the stops and walking
	WalkKm    float64 `json:"walk_km"`
	WalkMin   float64 `json:"walk_min"`
	MaxLegMin float64 `json:"max_leg_min"`
}

// Call is one model call.
type Call struct {
	Model        string  `json:"model"` // as the API reported it
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	LatencyMs    int64   `json:"latency_ms"`
	CostUSD      float64 `json:"cost_usd"` // 0 when the model's price is unknown
}

// Result is one (case, variant, rep) that reached a verdict.
type Result struct {
	Case     string   `json:"case"`
	Tags     []string `json:"tags"`
	Variant  string   `json:"variant"`
	Rep      int      `json:"rep"`
	Prompt   string   `json:"prompt"` // walks.PromptVersion, or the override file's name
	Status   string   `json:"status"` // composed | rejected
	Attempts int      `json:"attempts"`
	// Places the model proposed, over all attempts, and why any were
	// dropped (walks.Drop* reasons).
	Proposed int            `json:"proposed"`
	Drops    map[string]int `json:"drops"`
	Problems []string       `json:"problems,omitempty"` // a rejection's last reasons
	Walk     *WalkOut       `json:"walk,omitempty"`
	Calls    []Call         `json:"calls"`
	ModelUSD float64        `json:"model_usd"`
	// Maps calls made checking the walk, and their list price.
	PlaceLookups int     `json:"place_lookups"`
	RouteCalls   int     `json:"route_calls"`
	MapsUSD      float64 `json:"maps_usd"`
	WallMs       int64   `json:"wall_ms"`
}

// ErrorRow is an attempt that failed before a verdict.
type ErrorRow struct {
	Case    string `json:"case"`
	Variant string `json:"variant"`
	Rep     int    `json:"rep"`
	Class   string `json:"class"` // api | timeout | model_mismatch
	Error   string `json:"error"`
	Calls   []Call `json:"calls,omitempty"`
}

// Prices are list prices in USD per million tokens: input, output
// (thinking is billed as output). Unknown models cost 0 and the report
// says so. Check current pricing before relying on totals.
var Prices = map[string][2]float64{
	"claude-fable-5-1":  {10, 50},
	"claude-opus-5-5":   {4, 20},
	"claude-sonnet-5-5": {2, 10},
	"claude-haiku-5-5":  {0.10, 0.50},
}

// Maps list prices per call, before the monthly free allowance: Text Search
// (New) Pro, and Compute Routes Advanced (waypoint optimisation).
const (
	PlaceLookupUSD = 0.032
	RouteCallUSD   = 0.010
)

func cost(model string, in, out int) float64 {
	p := Prices[model]
	return (float64(in)*p[0] + float64(out)*p[1]) / 1e6
}

// Config is a benchmark run.
type Config struct {
	Cases    []Case
	Requests map[string]walks.Request
	Variants []Variant
	Reps     int
	Out      string // directory
	Prompt   string // recorded with each row
	// NewProposer makes a fresh proposer for a variant.
	NewProposer func(Variant) (walks.Proposer, error)
	Places      maps.PlaceFinder
	Routes      maps.WalkRouter
	Rules       walks.Rules
	Concurrency int           // walks composed at once
	Timeout     time.Duration // per walk, all attempts
	Log         *slog.Logger
}

type job struct {
	c   Case
	v   Variant
	rep int
}

func key(c, v string, rep int) string { return fmt.Sprintf("%s|%s|%d", c, v, rep) }

// Run composes every (case, variant, rep) not already in Out's
// results.jsonl and appends what happened. It returns how many ran.
func Run(ctx context.Context, cfg Config) (int, error) {
	if err := os.MkdirAll(filepath.Join(cfg.Out, "traces"), 0o755); err != nil {
		return 0, err
	}
	done, err := doneKeys(filepath.Join(cfg.Out, "results.jsonl"))
	if err != nil {
		return 0, err
	}
	var jobs []job
	for rep := 1; rep <= max(cfg.Reps, 1); rep++ {
		for _, v := range cfg.Variants {
			for _, c := range cfg.Cases {
				if !done[key(c.ID, v.Name, rep)] {
					jobs = append(jobs, job{c, v, rep})
				}
			}
		}
	}
	results, err := newWriter(filepath.Join(cfg.Out, "results.jsonl"))
	if err != nil {
		return 0, err
	}
	defer results.close()
	errs, err := newWriter(filepath.Join(cfg.Out, "errors.jsonl"))
	if err != nil {
		return 0, err
	}
	defer errs.close()

	log := cmp.Or(cfg.Log, slog.Default())
	sem := make(chan struct{}, max(cfg.Concurrency, 1))
	var wg sync.WaitGroup
	var ran, failed atomic.Int32
	for _, j := range jobs {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			res, eRow, trace := runOne(ctx, cfg, j)
			if err := writeTrace(cfg.Out, j, trace); err != nil {
				log.Warn("trace", "err", err)
			}
			if eRow != nil {
				failed.Add(1)
				errs.write(eRow)
				log.Warn("walk failed", "case", j.c.ID, "variant", j.v.Name, "rep", j.rep, "class", eRow.Class, "err", eRow.Error)
				return
			}
			ran.Add(1)
			results.write(res)
			log.Info("walk", "case", j.c.ID, "variant", j.v.Name, "rep", j.rep, "status", res.Status, "attempts", res.Attempts)
		})
	}
	wg.Wait()
	if n := failed.Load(); n > 0 {
		return int(ran.Load()), fmt.Errorf("%d of %d walks failed before a verdict (errors.jsonl); rerun to retry them", n, len(jobs))
	}
	return int(ran.Load()), nil
}

// counting wraps the Maps clients to count one walk's calls.
type counting struct {
	places        maps.PlaceFinder
	routes        maps.WalkRouter
	finds, routed atomic.Int32
}

func (c *counting) Find(ctx context.Context, r maps.FindRequest) (maps.Found, bool, error) {
	c.finds.Add(1)
	return c.places.Find(ctx, r)
}

func (c *counting) WalkThrough(ctx context.Context, places []string, optimize bool) (maps.WalkRoute, error) {
	c.routed.Add(1)
	return c.routes.WalkThrough(ctx, places, optimize)
}

// traceAttempt is one proposal in a trace file.
type traceAttempt struct {
	N        int            `json:"n"`
	Proposal walks.Proposal `json:"proposal"`
	Drops    []walks.Drop   `json:"drops,omitempty"`
	Problems []string       `json:"problems,omitempty"`
	Error    string         `json:"error,omitempty"`
}

func runOne(ctx context.Context, cfg Config, j job) (*Result, *ErrorRow, []traceAttempt) {
	ctx, cancel := context.WithTimeout(ctx, cmp.Or(cfg.Timeout, 15*time.Minute))
	defer cancel()
	prop, err := cfg.NewProposer(j.v)
	if err != nil {
		return nil, &ErrorRow{Case: j.c.ID, Variant: j.v.Name, Rep: j.rep, Class: "api", Error: err.Error()}, nil
	}
	cnt := &counting{places: cfg.Places, routes: cfg.Routes}
	var mu sync.Mutex
	var attempts []walks.Attempt
	comp := &walks.Composer{
		Proposer: prop, Places: cnt, Routes: cnt, Rules: cfg.Rules, Log: slog.New(slog.DiscardHandler),
		OnAttempt: func(a walks.Attempt) { mu.Lock(); attempts = append(attempts, a); mu.Unlock() },
	}
	start := time.Now()
	walk, err := comp.Compose(ctx, cfg.Requests[j.c.ID])
	wall := time.Since(start)

	var trace []traceAttempt
	var calls []Call
	res := &Result{
		Case: j.c.ID, Tags: j.c.Tags, Variant: j.v.Name, Rep: j.rep, Prompt: cfg.Prompt,
		Drops: map[string]int{}, Attempts: len(attempts), WallMs: wall.Milliseconds(),
		PlaceLookups: int(cnt.finds.Load()), RouteCalls: int(cnt.routed.Load()),
	}
	res.MapsUSD = float64(res.PlaceLookups)*PlaceLookupUSD + float64(res.RouteCalls)*RouteCallUSD
	mismatch := ""
	for _, a := range attempts {
		t := traceAttempt{N: a.N, Proposal: a.Proposal, Drops: a.Drops, Problems: a.Problems}
		if a.Err != nil {
			t.Error = a.Err.Error()
		}
		trace = append(trace, t)
		if a.Err == nil || a.Proposal.Meta.Model != "" {
			m := a.Proposal.Meta
			c := Call{Model: m.Model, InputTokens: m.InputTokens, OutputTokens: m.OutputTokens, LatencyMs: m.Latency.Milliseconds(),
				CostUSD: cmp.Or(m.CostUSD, cost(m.Model, m.InputTokens, m.OutputTokens))}
			calls = append(calls, c)
			res.ModelUSD += c.CostUSD
			// The model that answered must be the one named (no fallback,
			// no silent reroute), allowing for dated snapshot names.
			if m.Model != "" && !strings.HasPrefix(m.Model, j.v.Model) {
				mismatch = m.Model
			}
		}
		res.Proposed += 2 + len(a.Proposal.Stops)
		for _, d := range a.Drops {
			res.Drops[d.Reason]++
		}
	}
	res.Calls = calls

	var rejected *walks.Rejected
	switch {
	case mismatch != "":
		return nil, &ErrorRow{Case: j.c.ID, Variant: j.v.Name, Rep: j.rep, Class: "model_mismatch",
			Error: fmt.Sprintf("asked for %s, answered by %s", j.v.Model, mismatch), Calls: calls}, trace
	case errors.As(err, &rejected):
		res.Status, res.Problems = "rejected", rejected.Reasons
	case errors.Is(err, context.DeadlineExceeded):
		return nil, &ErrorRow{Case: j.c.ID, Variant: j.v.Name, Rep: j.rep, Class: "timeout", Error: err.Error(), Calls: calls}, trace
	case err != nil:
		return nil, &ErrorRow{Case: j.c.ID, Variant: j.v.Name, Rep: j.rep, Class: "api", Error: err.Error(), Calls: calls}, trace
	default:
		res.Status = "composed"
		res.Walk = walkOut(walk, attempts[len(attempts)-1].Proposal)
	}
	return res, nil, trace
}

// walkOut records a composed walk with the model's names and kinds for its
// stops (matched by note, which the composer copies verbatim).
func walkOut(w walks.Walk, p walks.Proposal) *WalkOut {
	byNote := map[string]walks.Place{}
	for _, pl := range slices.Concat([]walks.Place{p.Meet}, p.Stops, []walks.Place{p.Finish}) {
		byNote[strings.TrimSpace(pl.Note)] = pl
	}
	out := &WalkOut{Title: w.Title, Summary: w.Summary, Minutes: w.Minutes(),
		WalkKm: float64(w.Route.Meters) / 1000, WalkMin: w.Route.Duration.Minutes()}
	for _, s := range w.Stops {
		pl := byNote[s.Note]
		out.Stops = append(out.Stops, Stop{Role: string(s.Role), Name: pl.Name, Kind: pl.Kind, PlaceID: s.PlaceID,
			Minutes: s.Minutes, Note: s.Note, WalkMin: math.Round(s.Walk.Minutes()*10) / 10})
		out.MaxLegMin = max(out.MaxLegMin, s.Walk.Minutes())
	}
	return out
}

func writeTrace(dir string, j job, t []traceAttempt) error {
	name := strings.NewReplacer(":", "_", "/", "_").Replace(j.v.Name)
	p := filepath.Join(dir, "traces", name, fmt.Sprintf("%s_rep%d.json", j.c.ID, j.rep))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

// writer appends JSON lines safely from many goroutines.
type writer struct {
	mu sync.Mutex
	f  *os.File
}

func newWriter(path string) (*writer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &writer{f: f}, nil
}

func (w *writer) write(v any) {
	b, _ := json.Marshal(v)
	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = w.f.Write(append(b, '\n'))
}

func (w *writer) close() { _ = w.f.Close() }

func doneKeys(path string) (map[string]bool, error) {
	rows, err := ReadResults(path)
	done := map[string]bool{}
	for _, r := range rows {
		done[key(r.Case, r.Variant, r.Rep)] = true
	}
	return done, err
}

// ReadResults reads a results.jsonl ("" or a missing file = none).
func ReadResults(path string) ([]Result, error) {
	return readLines[Result](path)
}

func readLines[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		if len(strings.TrimSpace(sc.Text())) == 0 {
			continue
		}
		var v T
		if err := json.Unmarshal(sc.Bytes(), &v); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, v)
	}
	return out, sc.Err()
}
