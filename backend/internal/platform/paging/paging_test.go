package paging

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

func TestSize(t *testing.T) {
	for in, want := range map[int32]int32{0: 50, 1: 1, 200: 200, 201: 200, 1000: 200} {
		got, err := Size(in)
		if err != nil || got != want {
			t.Errorf("Size(%d) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := Size(-1); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("Size(-1) = %v, want InvalidArgument", err)
	}
}

func TestTokenRoundTrip(t *testing.T) {
	at := time.Date(2026, 11, 3, 10, 0, 0, 123456000, time.UTC)
	id := uuid.New()
	scope := Scope("ListReports", "open", "")
	k, err := Open(Encode(scope, Time(at), UUID(id), Bool(true), Int(-7), "x"), scope, 5)
	if err != nil || !k.Present {
		t.Fatalf("Open: %v", err)
	}
	if got := k.Time(); !got.Equal(at) {
		t.Errorf("time %v, want %v", got, at)
	}
	if got := k.UUID(); got != id {
		t.Errorf("uuid %v, want %v", got, id)
	}
	if !k.Bool() || k.Int() != -7 || k.String() != "x" || k.Err() != nil {
		t.Errorf("keys wrong, err %v", k.Err())
	}
}

func TestFirstPage(t *testing.T) {
	k, err := Open("", "any", 2)
	if err != nil || k.Present || !k.Time().IsZero() || k.UUID() != uuid.Nil || k.Err() != nil {
		t.Errorf("first page: %+v, %v", k, err)
	}
}

func TestOpenRefuses(t *testing.T) {
	scope := Scope("ListReports", "open")
	good := Encode(scope, "a", "b")
	for name, c := range map[string]struct {
		tok, scope string
		n          int
	}{
		"not base64":      {"%%%", scope, 2},
		"not json":        {"bm90IGpzb24", scope, 2},
		"another list":    {good, Scope("ListFlags", "open"), 2},
		"another filter":  {good, Scope("ListReports", "actioned"), 2},
		"wrong key count": {good, scope, 3},
	} {
		if _, err := Open(c.tok, c.scope, c.n); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v, want InvalidArgument", name, err)
		}
	}
}

func TestBadKeyIsInvalid(t *testing.T) {
	scope := Scope("L")
	k, err := Open(Encode(scope, "nope", "nope"), scope, 2)
	if err != nil {
		t.Fatal(err)
	}
	k.Time()
	k.UUID()
	if connect.CodeOf(k.Err()) != connect.CodeInvalidArgument {
		t.Errorf("Err = %v, want InvalidArgument", k.Err())
	}
}

func TestTrim(t *testing.T) {
	if p, more := Trim([]int{1, 2, 3}, 2); len(p) != 2 || !more {
		t.Errorf("Trim over: %v %v", p, more)
	}
	if p, more := Trim([]int{1, 2}, 2); len(p) != 2 || more {
		t.Errorf("Trim exact: %v %v", p, more)
	}
}
