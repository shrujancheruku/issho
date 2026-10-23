package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps/google"
	"github.com/shrujancheruku/issho/backend/internal/walks"
	"github.com/shrujancheruku/issho/backend/internal/walks/bench"
	walksclaude "github.com/shrujancheruku/issho/backend/internal/walks/claude"
	walksgemini "github.com/shrujancheruku/issho/backend/internal/walks/gemini"
)

// Proposers compared by default: the production model, a stronger Gemini,
// and the current Claude models at the effort each is meant for here.
// Claude runs through Claude Code on the developer's subscription
// (claudecode:); use claude: variants for the API.
const defaultVariants = "gemini:gemini-3.8-flash:MEDIUM,gemini:gemini-3.1-pro-preview:MEDIUM," +
	"claudecode:claude-fable-5-1:high,claudecode:claude-opus-5-5:high,claudecode:claude-sonnet-5-5:high,claudecode:claude-haiku-5-5:medium"

// One judge from each family, so neither family only judges itself.
const defaultJudges = "claudecode:claude-opus-5-5:high,gemini:gemini-3.1-pro-preview:HIGH"

// walkBench compares walk proposers (internal/walks/bench):
//
//	tool walk-bench run    [-variants …] [-reps 2] [-sample 12] [-cases id,…] [-prompt file] [-activities file] [-dry-run] -out dir
//	tool walk-bench judge  [-judges …] -out dir
//	tool walk-bench review [-n 40] -out dir   (GOOGLE_MAPS_EMBED_API_KEY embeds each route)
//	tool walk-bench report [-human review_answers.json] -out dir
func walkBench(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tool walk-bench run|judge|review|report [flags]")
	}
	fs := flag.NewFlagSet("walk-bench "+args[0], flag.ExitOnError)
	out := fs.String("out", "", "run directory (results, judgments, report)")
	data := fs.String("data", "../data", "the repo's data directory")
	activities := fs.String("activities", "", "activity catalogue to read themes from (default: data's)")
	variants := fs.String("variants", defaultVariants, "comma-separated provider:model[:effort]")
	judges := fs.String("judges", defaultJudges, "comma-separated provider:model[:effort] judges")
	reps := fs.Int("reps", 2, "walks per case and variant")
	sample := fs.Int("sample", 0, "run this many cases, spread over themes (0 = all)")
	caseIDs := fs.String("cases", "", "comma-separated case ids (default: all)")
	promptFile := fs.String("prompt", "", "system prompt to use instead of walks.SystemPrompt")
	concurrency := fs.Int("concurrency", 4, "walks or judgments at once (a subscription's rate limits are lower than the API's)")
	dryRun := fs.Bool("dry-run", false, "list what would run and its estimated cost")
	n := fs.Int("n", 40, "review: pairs on the sheet")
	human := fs.String("human", "", "report: the review sheet's downloaded answers")
	_ = fs.Parse(args[1:])
	if *out == "" {
		return errors.New("-out is required")
	}
	ctx := context.Background()

	cases, err := bench.Cases()
	if err != nil {
		return err
	}
	cases, err = pickCases(cases, *caseIDs, *sample)
	if err != nil {
		return err
	}
	reqs, err := bench.Requests(cases, *data, *activities)
	if err != nil {
		return err
	}
	settings := map[string]string{}
	for id, r := range reqs {
		settings[id] = walks.UserPrompt(r, nil)
	}

	switch args[0] {
	case "run":
		vs, err := parseVariants(*variants)
		if err != nil {
			return err
		}
		jobs := len(cases) * len(vs) * *reps
		if *dryRun {
			fmt.Printf("%d cases × %d variants × %d reps = %d walks (estimates at list price)\n", len(cases), len(vs), *reps, jobs)
			var paid, covered float64
			for _, v := range vs {
				model, mapsUSD := estimateWalk(v.Model)
				walksN := float64(len(cases) * *reps)
				note := ""
				if v.Provider == "claudecode" {
					covered += walksN * model
					paid += walksN * mapsUSD
					note = " (model on your subscription)"
				} else {
					paid += walksN * (model + mapsUSD)
				}
				fmt.Printf("  %-45s model ~$%.2f, Maps ~$%.2f%s\n", v.Name, walksN*model, walksN*mapsUSD, note)
			}
			pairs := len(cases) * len(vs) * (len(vs) - 1) / 2
			jPaid, jCovered := estimateJudgments(*judges, pairs*2)
			fmt.Printf("judging (later): up to %d pairs × 2 orders: ~$%.2f billed, ~$%.2f on your subscription\n", pairs, jPaid, jCovered)
			fmt.Printf("total billed ~$%.2f; API-price value of subscription calls ~$%.2f\n", paid+jPaid, covered+jCovered)
			return nil
		}
		prompt, system := walks.PromptVersion, ""
		if *promptFile != "" {
			b, err := os.ReadFile(*promptFile)
			if err != nil {
				return err
			}
			prompt, system = *promptFile, string(b)
		}
		mapsKey, err := mapsKey()
		if err != nil {
			return err
		}
		g := google.New(mapsKey)
		newProposer, err := proposers(system)
		if err != nil {
			return err
		}
		ran, err := bench.Run(ctx, bench.Config{
			Cases: cases, Requests: reqs, Variants: vs, Reps: *reps, Out: *out, Prompt: prompt,
			NewProposer: newProposer, Places: g, Routes: g, Rules: walks.DefaultRules(),
			Concurrency: *concurrency, Timeout: 20 * time.Minute,
		})
		slog.Info("walk bench", "walks", ran, "of", jobs)
		return err
	case "judge":
		js, err := judgeLLMs(*judges)
		if err != nil {
			return err
		}
		done, err := bench.Judge(ctx, bench.JudgeConfig{Dir: *out, Cases: cases, Settings: settings, Judges: js, Concurrency: *concurrency})
		slog.Info("walk bench judged", "judgments", done)
		return err
	case "review":
		// The Maps Embed API (free) shows each route on the page; without a
		// key with it enabled, the page links to Google Maps instead.
		return bench.Review(*out, settings, *n, uint64(time.Now().UnixNano()), os.Getenv("GOOGLE_MAPS_EMBED_API_KEY"))
	case "report":
		md, err := bench.Summarize(*out, *human)
		if err != nil {
			return err
		}
		if err := os.WriteFile(*out+"/report.md", []byte(md), 0o644); err != nil {
			return err
		}
		fmt.Print(md)
		return nil
	}
	return fmt.Errorf("unknown walk-bench command %q", args[0])
}

// pickCases keeps the named cases, or a sample spread evenly over themes
// (fixed seed, so a sample is the same cases every time).
func pickCases(cases []bench.Case, ids string, sample int) ([]bench.Case, error) {
	if ids != "" {
		var out []bench.Case
		for _, id := range strings.Split(ids, ",") {
			i := slices.IndexFunc(cases, func(c bench.Case) bool { return c.ID == id })
			if i < 0 {
				return nil, fmt.Errorf("no case %q", id)
			}
			out = append(out, cases[i])
		}
		return out, nil
	}
	if sample <= 0 || sample >= len(cases) {
		return cases, nil
	}
	byTheme := map[string][]bench.Case{}
	var themes []string
	for _, c := range cases {
		if byTheme[c.Tags[0]] == nil {
			themes = append(themes, c.Tags[0])
		}
		byTheme[c.Tags[0]] = append(byTheme[c.Tags[0]], c)
	}
	rng := rand.New(rand.NewPCG(7, 7))
	for _, t := range themes {
		rng.Shuffle(len(byTheme[t]), func(i, j int) { byTheme[t][i], byTheme[t][j] = byTheme[t][j], byTheme[t][i] })
	}
	var out []bench.Case
	for i := 0; len(out) < sample; i++ {
		for _, t := range themes {
			if i < len(byTheme[t]) && len(out) < sample {
				out = append(out, byTheme[t][i])
			}
		}
	}
	return out, nil
}

func parseVariants(s string) ([]bench.Variant, error) {
	var out []bench.Variant
	for _, part := range strings.Split(s, ",") {
		v, err := bench.ParseVariant(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// proposers makes proposers from the keys in the environment. Each
// provider's key is only required when a variant uses it.
func proposers(system string) (func(bench.Variant) (walks.Proposer, error), error) {
	return func(v bench.Variant) (walks.Proposer, error) {
		switch v.Provider {
		case "claudecode":
			return &bench.ClaudeCode{Model: v.Model, Effort: v.Effort, System: system}, nil
		case "claude":
			key := os.Getenv("ANTHROPIC_API_KEY")
			if key == "" {
				return nil, errors.New("ANTHROPIC_API_KEY is required for claude variants (or use claudecode: with a Claude subscription)")
			}
			p := walksclaude.New(key, v.Model, v.Effort)
			p.System = system
			return p, nil
		default:
			key := os.Getenv("GOOGLE_GEMINI_API_KEY")
			if key == "" {
				return nil, errors.New("GOOGLE_GEMINI_API_KEY is required for gemini variants")
			}
			p := walksgemini.NewModel(key, v.Model)
			if v.Effort != "" {
				p.Thinking = v.Effort
			}
			p.System = system
			return p, nil
		}
	}, nil
}

func judgeLLMs(s string) ([]bench.LLM, error) {
	vs, err := parseVariants(s)
	if err != nil {
		return nil, err
	}
	var out []bench.LLM
	for _, v := range vs {
		switch v.Provider {
		case "claudecode":
			out = append(out, &bench.ClaudeCode{Model: v.Model, Effort: v.Effort})
		case "claude":
			key := os.Getenv("ANTHROPIC_API_KEY")
			if key == "" {
				return nil, errors.New("ANTHROPIC_API_KEY is required for a claude judge (or use claudecode:)")
			}
			out = append(out, bench.NewClaudeLLM(key, v.Model, v.Effort))
		default:
			key := os.Getenv("GOOGLE_GEMINI_API_KEY")
			if key == "" {
				return nil, errors.New("GOOGLE_GEMINI_API_KEY is required for a gemini judge")
			}
			out = append(out, bench.NewGeminiLLM(key, v.Model, v.Effort))
		}
	}
	return out, nil
}

// estimateWalk is a rough cost per walk at list price, for the model and
// for Maps: about 1.3 proposals of ~1,500 input and ~6,000 output tokens
// (thinking included), and about 8 lookups and 1.3 routes a proposal.
// Models without a known price are guessed at $0.03 a proposal.
func estimateWalk(model string) (modelUSD, mapsUSD float64) {
	p, ok := bench.Prices[model]
	perProposal := 0.03
	if ok {
		perProposal = (1500*p[0] + 6000*p[1]) / 1e6
	}
	return 1.3 * perProposal, 1.3 * (8*bench.PlaceLookupUSD + bench.RouteCallUSD)
}

// estimateJudgments is a rough cost of n judgments by each judge (~2,500
// tokens in, ~3,000 out), split into billed and subscription.
func estimateJudgments(judges string, n int) (billed, covered float64) {
	for _, j := range strings.Split(judges, ",") {
		parts := strings.Split(j, ":")
		if len(parts) < 2 {
			continue
		}
		c := 0.02
		if p, ok := bench.Prices[parts[1]]; ok {
			c = (2500*p[0] + 3000*p[1]) / 1e6
		}
		if parts[0] == "claudecode" {
			covered += float64(n) * c
		} else {
			billed += float64(n) * c
		}
	}
	return billed, covered
}
