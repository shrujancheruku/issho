package matching

import "math"

// Pick is one person saying, after an outing, that they'd meet someone from
// their group again (private to them).
type Pick struct{ From, To UserID }

// PickPrior is how many returned picks everyone starts with when working
// out how often their picks are returned. It keeps a first unreturned pick
// from counting against someone (the other person may simply not have
// answered).
const PickPrior = 2

// MutualLinks turns meet-again picks into Relations.Mutual: a pair who
// picked each other (at any outings) is linked, with a strength in (0, 1].
// A pick counts for less from someone whose picks are rarely returned, so
// picking everyone, or the same kind of person every time, doesn't buy
// extra time with anyone: the strength is the lower of the two people's
// return rates, (returned + PickPrior) / (made + PickPrior), counting each
// person they picked once however many times they picked them.
func MutualLinks(picks []Pick) map[Pair]float64 {
	made := map[UserID]map[UserID]bool{}
	for _, p := range picks {
		if p.From == p.To {
			continue
		}
		if made[p.From] == nil {
			made[p.From] = map[UserID]bool{}
		}
		made[p.From][p.To] = true
	}
	rate := map[UserID]float64{}
	for from, tos := range made {
		returned := 0
		for to := range tos {
			if made[to][from] {
				returned++
			}
		}
		rate[from] = float64(returned+PickPrior) / float64(len(tos)+PickPrior)
	}
	links := map[Pair]float64{}
	for from, tos := range made {
		for to := range tos {
			if made[to][from] {
				links[MakePair(from, to)] = min(rate[from], rate[to])
			}
		}
	}
	return links
}

// Met is someone a person was in a group with, as far as their picks go: a
// person they met at an outing they gave feedback on, and whether they
// picked them.
type Met struct {
	Gender Gender
	Picked bool
}

// Lean describes picks that lean towards one gender: Picked of Picks went
// to Gender, who were MetShare of the Met people they could pick. PValue is
// how likely that many would be if picks ignored gender.
type Lean struct {
	Gender   Gender  `json:"gender"`
	Picked   int     `json:"picked"`
	Picks    int     `json:"picks"`
	Met      int     `json:"met"`
	MetShare float64 `json:"met_share"`
	PValue   float64 `json:"p_value"`
}

// When picks lean towards one gender enough to ask a moderator to look.
const (
	LeanMinPicks = 6    // too few picks say nothing
	LeanMinShare = 0.8  // at least this share of picks to one gender
	LeanMaxP     = 0.01 // and that unlikely if picks ignored gender
)

// PickLean reports whether someone's meet-again picks lean heavily towards
// one gender compared with who they met, for moderation: picking only
// women from mixed groups suggests someone here for dating, which the
// platonic rules don't allow. It only flags; a moderator decides. Groups
// are mixed by design (Balance), so the comparison is with the people they
// actually met, which also leaves same-gender-only people alone. Only
// "woman" and "man" are compared; nonbinary and unspecified people are too
// few to say anything.
func PickLean(met []Met) (Lean, bool) {
	count := map[Gender][2]int{} // gender → (met, picked)
	picks, total := 0, 0
	for _, m := range met {
		if m.Gender != Woman && m.Gender != Man {
			continue
		}
		c := count[m.Gender]
		c[0]++
		total++
		if m.Picked {
			c[1]++
			picks++
		}
		count[m.Gender] = c
	}
	if picks < LeanMinPicks {
		return Lean{}, false
	}
	for _, g := range []Gender{Woman, Man} {
		c := count[g]
		share := float64(c[1]) / float64(picks)
		expected := float64(c[0]) / float64(total)
		if share < LeanMinShare || share <= expected {
			continue
		}
		p := binomialTail(picks, c[1], expected)
		if p < LeanMaxP {
			return Lean{Gender: g, Picked: c[1], Picks: picks, Met: total, MetShare: expected, PValue: p}, true
		}
	}
	return Lean{}, false
}

// binomialTail is P(X ≥ k) for X ~ Binomial(n, p): how likely at least k of
// n picks would go to one gender if picks ignored gender.
func binomialTail(n, k int, p float64) float64 {
	if p <= 0 {
		return 0
	}
	if p >= 1 {
		return 1
	}
	sum := 0.0
	for i := k; i <= n; i++ {
		lg := lchoose(n, i) + float64(i)*math.Log(p) + float64(n-i)*math.Log1p(-p)
		sum += math.Exp(lg)
	}
	return min(1, sum)
}

func lchoose(n, k int) float64 {
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}
