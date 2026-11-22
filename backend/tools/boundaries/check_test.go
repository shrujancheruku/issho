package main

import (
	"strings"
	"testing"
)

// check runs the lint on a fixture: a miniature backend under testdata/.
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

func TestCleanBackendPasses(t *testing.T) {
	if got := check(t, "clean"); len(got) != 0 {
		t.Errorf("clean fixture has findings: %v", got)
	}
}

func TestImportingAnotherPackagesInternalPartsFails(t *testing.T) {
	wantOne(t, check(t, "internal-import"), KindInternal,
		"backend/internal/plans/plans.go:3", "internal/places/cache")
}

// Each package's sqlc queries generate under it (internal/staff/store), so
// reading another package's tables is an import of its internals, which the same
// rule refuses. The owner's own import, in the fixture's staff, passes.
func TestReadingAnotherPackagesQueriesFails(t *testing.T) {
	wantOne(t, check(t, "store-import"), KindInternal,
		"backend/internal/plans/plans.go:3", "internal/staff/store")
}

func TestDomainPackageImportingARealImplementationFails(t *testing.T) {
	wantOne(t, check(t, "real-impl"), KindOutsideService,
		"backend/internal/plans/plans.go:3", "internal/email/resend")
}

// The Go compiler already refuses an import of someone's `internal` directory;
// this lint is for the layout under internal/, where Go allows everything.
func TestRepoPasses(t *testing.T) {
	got, err := Check("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("the repo breaks its own boundaries: %v", got)
	}
}
