package main

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func lines(t *testing.T, root string, exempt ...string) []string {
	t.Helper()
	found, err := check(root, exempt)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(found)
	return found
}

func TestFlagsEveryReadOfTheSystemClock(t *testing.T) {
	got := lines(t, "testdata/bad")
	want := []string{ // sorted as strings, so line 9 follows line 16
		"testdata/bad/bad.go:11: time.Since reads the system clock",
		"testdata/bad/bad.go:13: time.Until reads the system clock",
		"testdata/bad/bad.go:16: time.Now reads the system clock",
		"testdata/bad/bad.go:9: time.Now reads the system clock",
		"testdata/bad/bad_test.go:9: time.Now reads the system clock",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got:\n%v\nwant:\n%v", got, want)
	}
}

func TestPassesCodeThatDoesNotReadTheClock(t *testing.T) {
	if got := lines(t, "testdata/ok"); len(got) != 0 {
		t.Fatalf("want no findings, got %v", got)
	}
}

func TestSkipsTheClocksOwnDirectory(t *testing.T) {
	if got := lines(t, "testdata/exempt"); len(got) != 1 {
		t.Fatalf("without the exemption want 1 finding, got %v", got)
	}
	if got := lines(t, "testdata/exempt", filepath.Join("testdata", "exempt")); len(got) != 0 {
		t.Fatalf("with the exemption want none, got %v", got)
	}
}

// Walking a tree never descends into a testdata directory, so the fixtures
// above don't fail the real run.
func TestSkipsTestdataDirectories(t *testing.T) {
	if got := lines(t, "."); len(got) != 0 {
		t.Fatalf("want no findings under the tool's own directory, got %v", got)
	}
}
