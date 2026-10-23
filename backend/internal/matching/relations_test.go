package matching

import (
	"math"
	"testing"
)

func TestMutualLinks(t *testing.T) {
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	tests := []struct {
		name  string
		picks []Pick
		want  map[Pair]float64
	}{
		{"none", nil, map[Pair]float64{}},
		{"one-sided picks don't link", []Pick{{"a", "b"}, {"c", "a"}}, map[Pair]float64{}},
		{"mutual, nothing else picked", []Pick{{"a", "b"}, {"b", "a"}}, map[Pair]float64{MakePair("a", "b"): 1}},
		{"picking twice counts once", []Pick{{"a", "b"}, {"a", "b"}, {"b", "a"}}, map[Pair]float64{MakePair("a", "b"): 1}},
		{"self-picks ignored", []Pick{{"a", "a"}, {"a", "b"}, {"b", "a"}}, map[Pair]float64{MakePair("a", "b"): 1}},
		{
			// a picked five people and only b picked back: (1+2)/(5+2).
			"rarely returned picks count less",
			[]Pick{{"a", "b"}, {"b", "a"}, {"a", "c"}, {"a", "d"}, {"a", "e"}, {"a", "f"}},
			map[Pair]float64{MakePair("a", "b"): 3.0 / 7},
		},
		{
			// b's own rate is 1; the pair takes the lower of the two.
			"the lower rate wins",
			[]Pick{{"a", "b"}, {"b", "a"}, {"b", "c"}, {"c", "b"}, {"a", "x"}},
			map[Pair]float64{MakePair("a", "b"): 3.0 / 4, MakePair("b", "c"): 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MutualLinks(tt.picks)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for p, w := range tt.want {
				if !near(got[p], w) {
					t.Errorf("%v = %.4f, want %.4f", p, got[p], w)
				}
			}
		})
	}
}

func TestPickLean(t *testing.T) {
	// met builds n people of each gender, the first k of them picked.
	met := func(women, pickedWomen, men, pickedMen int) []Met {
		var out []Met
		for i := range women {
			out = append(out, Met{Gender: Woman, Picked: i < pickedWomen})
		}
		for i := range men {
			out = append(out, Met{Gender: Man, Picked: i < pickedMen})
		}
		return out
	}
	tests := []struct {
		name   string
		met    []Met
		flag   bool
		gender Gender
	}{
		{"too few picks", met(10, 5, 10, 0), false, ""},
		{"only women picked from mixed groups", met(12, 8, 12, 0), true, Woman},
		{"only men picked from mixed groups", met(12, 0, 12, 7), true, Man},
		{"balanced picks", met(12, 4, 12, 4), false, ""},
		{"mostly one gender, but not unlikely by chance", met(12, 5, 12, 2), false, ""},
		// Someone who met almost only women (same-gender groups) picks women.
		{"met mostly that gender", met(20, 8, 1, 0), false, ""},
		{"nonbinary and unspecified people aren't compared", append(met(0, 0, 0, 0),
			Met{Nonbinary, true}, Met{Nonbinary, true}, Met{Nonbinary, true},
			Met{Unspecified, true}, Met{Unspecified, true}, Met{Unspecified, true}, Met{Man, false}), false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lean, flag := PickLean(tt.met)
			if flag != tt.flag || lean.Gender != tt.gender {
				t.Errorf("PickLean = %+v, %v; want %v %q", lean, flag, tt.flag, tt.gender)
			}
			if flag && (lean.PValue >= LeanMaxP || lean.Picked < LeanMinPicks) {
				t.Errorf("flagged with %+v", lean)
			}
		})
	}
}

func TestBinomialTail(t *testing.T) {
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	// 3 fair coin flips: P(X ≥ 2) = 4/8.
	if got := binomialTail(3, 2, 0.5); !near(got, 0.5) {
		t.Errorf("binomialTail(3, 2, 0.5) = %v", got)
	}
	if got := binomialTail(5, 0, 0.3); !near(got, 1) {
		t.Errorf("P(X ≥ 0) = %v, want 1", got)
	}
	if binomialTail(4, 1, 0) != 0 || binomialTail(4, 4, 1) != 1 {
		t.Error("edge probabilities")
	}
}
