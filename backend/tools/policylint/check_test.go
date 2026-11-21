package main

import (
	"strings"
	"testing"
)

// check runs the lint on a fixture: a miniature repo under testdata/ with its
// own docs/spec and backend.
func check(t *testing.T, fixture string) []Finding {
	t.Helper()
	got, err := Check("testdata/" + fixture)
	if err != nil {
		t.Fatalf("Check(%s): %v", fixture, err)
	}
	return got
}

// wantOne fails unless there is exactly one finding, of the kind, whose
// position and message contain each of the substrings.
func wantOne(t *testing.T, got []Finding, kind string, contains ...string) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("want one %s finding, got %d: %v", kind, len(got), got)
	}
	f := got[0]
	if f.Kind != kind {
		t.Errorf("kind = %q, want %q (%v)", f.Kind, kind, f)
	}
	for _, s := range contains {
		if !strings.Contains(f.String(), s) {
			t.Errorf("finding %q does not mention %q", f, s)
		}
	}
}

func TestLinkedConstantsPass(t *testing.T) {
	if got := check(t, "clean"); len(got) != 0 {
		t.Errorf("clean fixture has findings: %v", got)
	}
}

func TestConstantWithNoLinkFails(t *testing.T) {
	wantOne(t, check(t, "unlinked"), KindNoLink, "MaxGroupSize", "policy.go:4")
}

func TestLinkToMissingFileFails(t *testing.T) {
	wantOne(t, check(t, "deadlink"), KindDeadLink, "MaxGroupSize", "docs/spec/gone.md")
}

func TestIntegerLiteralOutsidePolicyFails(t *testing.T) {
	wantOne(t, check(t, "literal-int"), KindLiteral, "plans.go:3", "policy.MaxGroupSize")
}

func TestDurationLiteralOutsidePolicyFails(t *testing.T) {
	got := check(t, "literal-duration")
	if len(got) != 2 {
		t.Fatalf("want two findings, got %d: %v", len(got), got)
	}
	// Both of the round's rules are 7 days, so each literal names both.
	for _, f := range got {
		if f.Kind != KindLiteral {
			t.Errorf("kind = %q, want %q (%v)", f.Kind, KindLiteral, f)
		}
	}
	for i, want := range []string{"policy.InvitationWindow", "policy.RoundLength, policy.FeedbackWindow"} {
		if !strings.Contains(got[i].String(), want) {
			t.Errorf("finding %q does not mention %q", got[i], want)
		}
	}
}

func TestLiteralsThatAreNotPolicyAreAllowed(t *testing.T) {
	if got := check(t, "literal-allowed"); len(got) != 0 {
		t.Errorf("findings: %v", got)
	}
}

// The real repo must pass its own lint; `make policy-lint` runs the same check.
func TestRepoPasses(t *testing.T) {
	if got := check(t, "../../../.."); len(got) != 0 {
		t.Errorf("repo has findings: %v", got)
	}
}

func TestRunExitsNonZeroOnFindings(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		code    int
	}{{"clean", 0}, {"unlinked", 1}, {"literal-int", 1}} {
		var out, errOut strings.Builder
		if got := run([]string{"-root", "testdata/" + tc.fixture}, &out, &errOut); got != tc.code {
			t.Errorf("%s: exit %d, want %d (stderr: %s)", tc.fixture, got, tc.code, errOut.String())
		}
		if tc.code == 1 && !strings.Contains(errOut.String(), "finding(s)") {
			t.Errorf("%s: stderr has no summary: %q", tc.fixture, errOut.String())
		}
	}
}
