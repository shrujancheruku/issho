package maps

import (
	"math"
	"testing"
)

func TestPolyline(t *testing.T) {
	// Google's documented example.
	want := []LatLng{{38.5, -120.2}, {40.7, -120.95}, {43.252, -126.453}}
	const enc = "_p~iF~ps|U_ulLnnqC_mqNvxq`@"
	if got := EncodePolyline(want); got != enc {
		t.Errorf("encoded %q, want %q", got, enc)
	}
	got, err := DecodePolyline(enc)
	if err != nil || len(got) != 3 {
		t.Fatalf("decoded %v, %v", got, err)
	}
	for i := range want {
		if math.Abs(got[i].Lat-want[i].Lat) > 1e-9 || math.Abs(got[i].Lng-want[i].Lng) > 1e-9 {
			t.Errorf("point %d: %v, want %v", i, got[i], want[i])
		}
	}
	for _, bad := range []string{"_p~iF", "_p~iF~ps|U_", "ab\x01"} {
		if _, err := DecodePolyline(bad); err == nil {
			t.Errorf("%q decoded without an error", bad)
		}
	}
}

func TestBoundary(t *testing.T) {
	// A unit-ish square with a separate island to the east.
	b := Boundary{
		{{16, 108}, {16, 108.02}, {16.02, 108.02}, {16.02, 108}},
		{{16, 108.1}, {16, 108.11}, {16.01, 108.11}},
	}
	for p, in := range map[LatLng]bool{
		{16.01, 108.01}: true, {16.002, 108.108}: true, {16.01, 108.05}: false, {15.99, 108.01}: false,
	} {
		if b.Contains(p) != in {
			t.Errorf("Contains(%v) = %v", p, !in)
		}
	}
	if d := b.DistanceM(LatLng{16.01, 108.01}); d != 0 {
		t.Errorf("inside: distance %v", d)
	}
	// 0.01° of latitude south of the bottom edge: about 1.1 km.
	if d := b.DistanceM(LatLng{15.99, 108.01}); math.Abs(d-1106) > 15 {
		t.Errorf("south of the square: %.0f m, want ~1106", d)
	}
	c, r := b.Circle()
	for _, ring := range b {
		for _, p := range ring {
			if DistanceM(c, p) > r+1e-6 {
				t.Errorf("%v outside the covering circle", p)
			}
		}
	}
	rt, err := DecodeBoundary(b.Encode())
	if err != nil || len(rt) != 2 || len(rt[1]) != 3 {
		t.Errorf("round trip: %v, %v", rt, err)
	}
	if _, err := DecodeBoundary([]string{EncodePolyline([]LatLng{{1, 1}, {2, 2}})}); err == nil {
		t.Error("a two-point ring decoded")
	}
}
