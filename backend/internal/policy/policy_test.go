package policy

import (
	"testing"
	"time"
)

// The expected values are the spec's own words (docs/spec/core-loop.md,
// matching.md, outings.md), written out independently of the constants.
func TestThinLoopNumbers(t *testing.T) {
	if MinGroupSize != 3 || MaxGroupSize != 6 {
		t.Errorf("group size = %d to %d, spec says 3 to 6", MinGroupSize, MaxGroupSize)
	}
	if PreferredGroupSizeLow != 4 || PreferredGroupSizeHigh != 5 {
		t.Errorf("preferred size = %d or %d, spec says 4 or 5", PreferredGroupSizeLow, PreferredGroupSizeHigh)
	}
	if InvitationWindow != 48*time.Hour {
		t.Errorf("invitation window = %v, spec says 48 hours", InvitationWindow)
	}
	if RoundLength != 7*24*time.Hour {
		t.Errorf("round length = %v, spec says 7 days", RoundLength)
	}
	if CheckInOpensBeforeStart != 30*time.Minute {
		t.Errorf("check-in opens %v before the start, spec says 30 minutes", CheckInOpensBeforeStart)
	}
	if FeedbackWindow != 7*24*time.Hour {
		t.Errorf("feedback window = %v, spec says 7 days", FeedbackWindow)
	}
}

func TestPreferredSizesLieWithinTheGroupRange(t *testing.T) {
	if !(MinGroupSize <= PreferredGroupSizeLow && PreferredGroupSizeLow <= PreferredGroupSizeHigh && PreferredGroupSizeHigh <= MaxGroupSize) {
		t.Errorf("want %d <= %d <= %d <= %d", MinGroupSize, PreferredGroupSizeLow, PreferredGroupSizeHigh, MaxGroupSize)
	}
}
