package plans

import (
	"math"
	"testing"
)

func TestReliability(t *testing.T) {
	for _, c := range []struct {
		attended, confirmed int
		want                float64
	}{
		{0, 0, 1},       // a newcomer
		{1, 1, 1},       // turned up
		{0, 1, 2.0 / 3}, // one no-show in their first outing
		{3, 4, 5.0 / 6},
		{0, 10, 2.0 / 12},
	} {
		if got := reliability(c.attended, c.confirmed); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("reliability(%d, %d) = %v, want %v", c.attended, c.confirmed, got, c.want)
		}
	}
}
