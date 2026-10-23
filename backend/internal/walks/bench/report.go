package bench

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Summarize reads a run's results, errors, judgments and (if given) the
// human review answers, and returns a Markdown report.
//
// Quality is reported two ways, because they answer different questions:
// how often a variant gives a usable walk at all (composed rate), and how
// good its walks are when both sides of a pair have one (judge win rate).
// Every rate carries a 95% interval; differences inside the intervals are
// not evidence.
func Summarize(dir, humanAnswers string) (string, error) {
	results, err := ReadResults(filepath.Join(dir, "results.jsonl"))
	if err != nil {
		return "", err
	}
	errRows, err := readLines[ErrorRow](filepath.Join(dir, "errors.jsonl"))
	if err != nil {
		return "", err
	}
	judgments, err := readLines[Judgment](filepath.Join(dir, "judgments.jsonl"))
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", errors.New("no results yet")
	}
	var variants []string
	byVariant := map[string][]Result{}
	for _, r := range results {
		if byVariant[r.Variant] == nil {
			variants = append(variants, r.Variant)
		}
		byVariant[r.Variant] = append(byVariant[r.Variant], r)
	}
	errCount := map[string]int{}
	for _, e := range errRows {
		errCount[e.Variant]++
		// A variant whose every walk failed still gets a row.
		if !slices.Contains(variants, e.Variant) {
			variants = append(variants, e.Variant)
		}
	}
	slices.Sort(variants)

	var b strings.Builder
	cases := map[string]bool{}
	reps := 0
	for _, r := range results {
		cases[r.Case] = true
		reps = max(reps, r.Rep)
	}
	fmt.Fprintf(&b, "# Walk benchmark\n\n%d cases × %d variants × up to %d reps; prompt %s. ", len(cases), len(variants), reps, results[0].Prompt)
	fmt.Fprintf(&b, "Noise floor on a rate over n = cases × reps: about ±%.0f points at 95%%.\n\n", 100/math.Sqrt(float64(len(cases)*reps)))

	b.WriteString("## Usable walks and cost\n\n")
	b.WriteString("| Variant | Composed | First try | Places usable | Failed attempts | Min | Km | Longest leg | Model $/walk | Maps $/walk | Model s (p50/p90) |\n|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, v := range variants {
		rs := byVariant[v]
		var composed, first, proposed, dropped int
		var minutes, km, leg, modelUSD, mapsUSD float64
		var lat []float64
		for _, r := range rs {
			proposed += r.Proposed
			for _, n := range r.Drops {
				dropped += n
			}
			modelUSD += r.ModelUSD
			mapsUSD += r.MapsUSD
			var ms int64
			for _, c := range r.Calls {
				ms += c.LatencyMs
			}
			lat = append(lat, float64(ms)/1000)
			if r.Status == "composed" {
				composed++
				if r.Attempts == 1 {
					first++
				}
				minutes += float64(r.Walk.Minutes)
				km += r.Walk.WalkKm
				leg += r.Walk.MaxLegMin
			}
		}
		lo, hi := wilson(composed, len(rs))
		per := func(x float64) string {
			if composed == 0 {
				return "–"
			}
			return fmt.Sprintf("%.0f", x/float64(composed))
		}
		price := "n/a"
		if modelUSD > 0 && composed > 0 {
			price = fmt.Sprintf("%.3f", modelUSD/float64(composed))
		}
		fmt.Fprintf(&b, "| %s | %d/%d %.0f%% (%.0f–%.0f) | %.0f%% | %.0f%% | %d | %s | %.1f | %s | %s | %.2f | %.0f / %.0f |\n",
			v, composed, len(rs), pct(composed, len(rs)), lo*100, hi*100, pct(first, len(rs)), 100-pct(dropped, proposed), errCount[v],
			per(minutes), km/math.Max(float64(composed), 1), per(leg), price, mapsUSD/math.Max(float64(composed), 1),
			quantile(lat, 0.5), quantile(lat, 0.9))
	}
	b.WriteString("\nComposed: walks that passed every check (95% Wilson interval). First try: composed without a retry. Places usable: proposed places that existed, were open, in the area and the right kind (higher = fewer invented or wrong places). Model $ is unknown (n/a) for models without a price in bench.Prices; for claudecode variants it's what Claude Code reports the calls would cost on the API, not what a subscription is billed, and includes Claude Code's own context. Maps $ is list price before the free allowance. Failed attempts (errors.jsonl: API errors, timeouts, a different model answering) are retried on a rerun and never count as rejections.\n\n")

	b.WriteString("### Why places were dropped\n\n| Variant | " + strings.Join(dropReasons, " | ") + " |\n|---|" + strings.Repeat("---|", len(dropReasons)) + "\n")
	for _, v := range variants {
		counts := map[string]int{}
		proposed := 0
		for _, r := range byVariant[v] {
			proposed += r.Proposed
			for k, n := range r.Drops {
				counts[k] += n
			}
		}
		cells := make([]string, len(dropReasons))
		for i, k := range dropReasons {
			cells[i] = fmt.Sprintf("%.1f%%", pct(counts[k], proposed))
		}
		fmt.Fprintf(&b, "| %s | %s |\n", v, strings.Join(cells, " | "))
	}

	for i, name := range []string{"theme", "city", "familiarity"} {
		fmt.Fprintf(&b, "\n### Composed rate by %s\n\n", name)
		writeByTag(&b, results, variants, i)
	}

	if len(judgments) > 0 {
		writeJudgments(&b, judgments, variants)
	}
	if humanAnswers != "" {
		if err := writeHuman(&b, dir, humanAnswers, judgments, variants); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

var dropReasons = []string{"not_found", "wrong_kind", "far", "duplicate", "closed", "outlier"}

func writeByTag(b *strings.Builder, results []Result, variants []string, tag int) {
	values := map[string]bool{}
	for _, r := range results {
		if tag < len(r.Tags) {
			values[r.Tags[tag]] = true
		}
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	b.WriteString("| Variant | " + strings.Join(keys, " | ") + " |\n|---|" + strings.Repeat("---|", len(keys)) + "\n")
	for _, v := range variants {
		cells := make([]string, len(keys))
		for i, k := range keys {
			n, c := 0, 0
			for _, r := range results {
				if r.Variant == v && tag < len(r.Tags) && r.Tags[tag] == k {
					n++
					if r.Status == "composed" {
						c++
					}
				}
			}
			cells[i] = fmt.Sprintf("%d/%d", c, n)
		}
		fmt.Fprintf(b, "| %s | %s |\n", v, strings.Join(cells, " | "))
	}
}

// pairVerdict is a judge's verdict on one pair: both orders combined, so a
// verdict that flips with the order (position bias) counts as a tie.
type pairVerdict struct {
	caseID, x, y, judge string
	winners             map[string]string
	consistent          bool    // both orders agreed on overall
	dubiousX, dubiousY  float64 // doubted claims, mean over orders
}

func combine(js []Judgment) []pairVerdict {
	type k struct{ c, x, y, j string }
	groups := map[k][]Judgment{}
	var order []k
	for _, j := range js {
		key := k{j.Case, j.X, j.Y, j.Judge}
		if groups[key] == nil {
			order = append(order, key)
		}
		groups[key] = append(groups[key], j)
	}
	var out []pairVerdict
	for _, key := range order {
		g := groups[key]
		pv := pairVerdict{caseID: key.c, x: key.x, y: key.y, judge: key.j, winners: map[string]string{}, consistent: true}
		for _, crit := range Criteria {
			w := g[0].Winners[crit]
			for _, j := range g[1:] {
				if j.Winners[crit] != w {
					w = "tie"
					if crit == "overall" {
						pv.consistent = false
					}
				}
			}
			pv.winners[crit] = w
		}
		for _, j := range g {
			pv.dubiousX += float64(len(j.DubiousX)) / float64(len(g))
			pv.dubiousY += float64(len(j.DubiousY)) / float64(len(g))
		}
		out = append(out, pv)
	}
	return out
}

// score is a variant's points in a pair: 1 win, 0.5 tie, 0 loss.
func score(winner, v string) float64 {
	switch winner {
	case v:
		return 1
	case "tie":
		return 0.5
	}
	return 0
}

func writeJudgments(b *strings.Builder, js []Judgment, variants []string) {
	pvs := combine(js)
	judges := map[string]bool{}
	for _, p := range pvs {
		judges[p.judge] = true
	}
	b.WriteString("\n## Quality when both composed (blind pairwise judges)\n\n")
	b.WriteString("Win rate against the other variants on the same cases (win 1, tie 0.5), all judges together, both orders combined; 95% bootstrap interval over cases. Dubious: claims per walk a judge doubted.\n\n")
	b.WriteString("| Variant | " + strings.Join(Criteria, " | ") + " | Dubious |\n|---|" + strings.Repeat("---|", len(Criteria)+1) + "\n")
	for _, v := range variants {
		cells := make([]string, len(Criteria))
		for i, crit := range Criteria {
			cells[i] = winRate(pvs, v, crit)
		}
		dub, n := 0.0, 0
		for _, p := range pvs {
			switch v {
			case p.x:
				dub, n = dub+p.dubiousX, n+1
			case p.y:
				dub, n = dub+p.dubiousY, n+1
			}
		}
		d := "–"
		if n > 0 {
			d = fmt.Sprintf("%.2f", dub/float64(n))
		}
		fmt.Fprintf(b, "| %s | %s | %s |\n", v, strings.Join(cells, " | "), d)
	}

	b.WriteString("\n### Head to head (overall)\n\nRow's win rate against column.\n\n| | " + strings.Join(variants, " | ") + " |\n|---|" + strings.Repeat("---|", len(variants)) + "\n")
	for _, v := range variants {
		cells := make([]string, len(variants))
		for i, w := range variants {
			if v == w {
				cells[i] = "–"
				continue
			}
			var sum float64
			n := 0
			for _, p := range pvs {
				if (p.x == v && p.y == w) || (p.x == w && p.y == v) {
					sum += score(p.winners["overall"], v)
					n++
				}
			}
			if n == 0 {
				cells[i] = "–"
			} else {
				cells[i] = fmt.Sprintf("%.0f%% (%d)", 100*sum/float64(n), n)
			}
		}
		fmt.Fprintf(b, "| %s | %s |\n", v, strings.Join(cells, " | "))
	}

	names := make([]string, 0, len(judges))
	for j := range judges {
		names = append(names, j)
	}
	slices.Sort(names)
	// Each judge on its own: a ranking both judges share is robust; where
	// they split along family lines, the human review decides.
	b.WriteString("\n### Overall win rate by judge\n\n| Variant | " + strings.Join(names, " | ") + " |\n|---|" + strings.Repeat("---|", len(names)) + "\n")
	for _, v := range variants {
		cells := make([]string, len(names))
		for i, j := range names {
			var sum float64
			n := 0
			for _, p := range pvs {
				if p.judge == j && (p.x == v || p.y == v) {
					sum += score(p.winners["overall"], v)
					n++
				}
			}
			cells[i] = "–"
			if n > 0 {
				cells[i] = fmt.Sprintf("%.0f%% (%d)", 100*sum/float64(n), n)
			}
		}
		fmt.Fprintf(b, "| %s | %s |\n", v, strings.Join(cells, " | "))
	}

	b.WriteString("\n### Judges\n\n| Judge | Pairs | Same verdict in both orders | Own family's walks win rate |\n|---|---|---|---|\n")
	for _, j := range names {
		n, consistent := 0, 0
		var own, ownN float64
		family := strings.SplitN(j, ":", 2)[0]
		for _, p := range pvs {
			if p.judge != j {
				continue
			}
			n++
			if p.consistent {
				consistent++
			}
			xf, yf := strings.HasPrefix(p.x, family+":"), strings.HasPrefix(p.y, family+":")
			if xf != yf { // a pair across families: does the judge favour its own?
				mine := p.x
				if yf {
					mine = p.y
				}
				own += score(p.winners["overall"], mine)
				ownN++
			}
		}
		selfPref := "–"
		if ownN > 0 {
			selfPref = fmt.Sprintf("%.0f%% (%d)", 100*own/ownN, int(ownN))
		}
		fmt.Fprintf(b, "| %s | %d | %.0f%% | %s |\n", j, n, pct(consistent, n), selfPref)
	}
	if len(names) > 1 {
		agree, n := 0, 0
		byPair := map[string]map[string]string{}
		for _, p := range pvs {
			k := p.caseID + "|" + p.x + "|" + p.y
			if byPair[k] == nil {
				byPair[k] = map[string]string{}
			}
			byPair[k][p.judge] = p.winners["overall"]
		}
		for _, m := range byPair {
			if len(m) < 2 {
				continue
			}
			n++
			first := m[names[0]]
			same := true
			for _, w := range m {
				if w != first {
					same = false
				}
			}
			if same {
				agree++
			}
		}
		fmt.Fprintf(b, "\nJudges agree on the overall verdict for %.0f%% of %d pairs. A judge favouring its own family's walks much more than the others do suggests self-preference: weigh the other judge and the human review more.\n", pct(agree, n), n)
	}
}

// winRate is a variant's mean score on a criterion with a bootstrap
// interval over cases.
func winRate(pvs []pairVerdict, v, crit string) string {
	byCase := map[string][]float64{}
	for _, p := range pvs {
		if p.x == v || p.y == v {
			byCase[p.caseID] = append(byCase[p.caseID], score(p.winners[crit], v))
		}
	}
	if len(byCase) == 0 {
		return "–"
	}
	cases := make([]string, 0, len(byCase))
	for c := range byCase {
		cases = append(cases, c)
	}
	slices.Sort(cases)
	mean := func(pick func(int) string) float64 {
		var sum float64
		n := 0
		for i := range cases {
			for _, s := range byCase[pick(i)] {
				sum += s
				n++
			}
		}
		return sum / float64(n)
	}
	est := mean(func(i int) string { return cases[i] })
	rng := rand.New(rand.NewPCG(1, 2))
	boots := make([]float64, 1000)
	for i := range boots {
		boots[i] = mean(func(int) string { return cases[rng.IntN(len(cases))] })
	}
	return fmt.Sprintf("%.0f%% (%.0f–%.0f)", est*100, quantile(boots, 0.025)*100, quantile(boots, 0.975)*100)
}

func writeHuman(b *strings.Builder, dir, answersPath string, js []Judgment, variants []string) error {
	keyRaw, err := os.ReadFile(filepath.Join(dir, "review_key.json"))
	if err != nil {
		return fmt.Errorf("human answers need the review key: %w", err)
	}
	var key []ReviewPair
	if err := json.Unmarshal(keyRaw, &key); err != nil {
		return err
	}
	raw, err := os.ReadFile(answersPath)
	if err != nil {
		return err
	}
	var answers map[string]struct {
		Overall string `json:"overall"`
		Wrong   bool   `json:"wrong"`
	}
	if err := json.Unmarshal(raw, &answers); err != nil {
		return err
	}
	pvs := combine(js)
	b.WriteString("\n## Human review\n\n")
	sums, counts := map[string]float64{}, map[string]int{}
	agree, compared := map[string]int{}, map[string]int{}
	wrong := 0
	for _, p := range key {
		a, ok := answers[p.ID]
		if !ok {
			continue
		}
		if a.Wrong {
			wrong++
		}
		winner := "tie"
		switch a.Overall {
		case "A":
			winner = p.A
		case "B":
			winner = p.B
		}
		for _, v := range []string{p.A, p.B} {
			sums[v] += score(winner, v)
			counts[v]++
		}
		for _, pv := range pvs {
			if pv.caseID == p.Case && ((pv.x == p.A && pv.y == p.B) || (pv.x == p.B && pv.y == p.A)) {
				compared[pv.judge]++
				if pv.winners["overall"] == winner {
					agree[pv.judge]++
				}
			}
		}
	}
	fmt.Fprintf(b, "%d pairs answered; a wrong place or fact flagged in %d.\n\n| Variant | Human win rate |\n|---|---|\n", len(answers), wrong)
	for _, v := range variants {
		if counts[v] > 0 {
			fmt.Fprintf(b, "| %s | %.0f%% (%d) |\n", v, 100*sums[v]/float64(counts[v]), counts[v])
		}
	}
	b.WriteString("\n| Judge | Agrees with the reviewer |\n|---|---|\n")
	for j, n := range compared {
		fmt.Fprintf(b, "| %s | %.0f%% of %d |\n", j, pct(agree[j], n), n)
	}
	b.WriteString("\nTrust a judge's taste scores only once it agrees with the reviewer on most clear-cut pairs (aim for 80%+ over 30+ pairs; ties make this lower).\n")
	return nil
}

func pct(a, n int) float64 {
	if n == 0 {
		return 0
	}
	return 100 * float64(a) / float64(n)
}

// wilson is the 95% Wilson score interval for k of n.
func wilson(k, n int) (float64, float64) {
	if n == 0 {
		return 0, 0
	}
	const z = 1.96
	p, nf := float64(k)/float64(n), float64(n)
	d := 1 + z*z/nf
	c := (p + z*z/(2*nf)) / d
	h := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf)) / d
	return math.Max(0, c-h), math.Min(1, c+h)
}

func quantile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	return s[int(math.Round(q*float64(len(s)-1)))]
}
