package maps

import (
	"fmt"
	"math"
)

// Boundary is an area's outline: one or more outer rings, since an area can
// include islands. Holes are not kept: administrative areas rarely have them.
type Boundary [][]LatLng

// DecodeBoundary decodes one polyline per ring, as city packs store them.
func DecodeBoundary(rings []string) (Boundary, error) {
	var b Boundary
	for i, s := range rings {
		r, err := DecodePolyline(s)
		if err != nil {
			return nil, fmt.Errorf("ring %d: %w", i, err)
		}
		if len(r) < 3 {
			return nil, fmt.Errorf("ring %d: %d points, need at least 3", i, len(r))
		}
		b = append(b, r)
	}
	return b, nil
}

// Encode is the inverse of DecodeBoundary.
func (b Boundary) Encode() []string {
	out := make([]string, len(b))
	for i, r := range b {
		out[i] = EncodePolyline(r)
	}
	return out
}

// Contains reports whether p is inside any ring (even-odd rule). Planar
// lat/lng is fine at the scale of a city's districts.
func (b Boundary) Contains(p LatLng) bool {
	for _, r := range b {
		in := false
		for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
			a, c := r[i], r[j]
			if (a.Lat > p.Lat) != (c.Lat > p.Lat) &&
				p.Lng < (c.Lng-a.Lng)*(p.Lat-a.Lat)/(c.Lat-a.Lat)+a.Lng {
				in = !in
			}
		}
		if in {
			return true
		}
	}
	return false
}

// DistanceM is how far p is from the area: 0 inside, otherwise the distance
// to the nearest edge, in metres.
func (b Boundary) DistanceM(p LatLng) float64 {
	if b.Contains(p) {
		return 0
	}
	// Local flat projection around p, in metres: accurate to well under 1%
	// over the few tens of kilometres a city spans.
	kx := 111320 * math.Cos(p.Lat*math.Pi/180)
	const ky = 110574
	xy := func(q LatLng) (float64, float64) { return (q.Lng - p.Lng) * kx, (q.Lat - p.Lat) * ky }
	best := math.Inf(1)
	for _, r := range b {
		for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
			ax, ay := xy(r[j])
			bx, by := xy(r[i])
			dx, dy := bx-ax, by-ay
			t := 0.0
			if l := dx*dx + dy*dy; l > 0 {
				t = max(0, min(1, -(ax*dx+ay*dy)/l))
			}
			best = min(best, math.Hypot(ax+t*dx, ay+t*dy))
		}
	}
	return best
}

// Circle returns a circle covering the whole boundary: the centre of its
// bounding box and the distance to its farthest vertex.
func (b Boundary) Circle() (LatLng, float64) {
	minLat, minLng, maxLat, maxLng := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, r := range b {
		for _, q := range r {
			minLat, maxLat = min(minLat, q.Lat), max(maxLat, q.Lat)
			minLng, maxLng = min(minLng, q.Lng), max(maxLng, q.Lng)
		}
	}
	c := LatLng{Lat: (minLat + maxLat) / 2, Lng: (minLng + maxLng) / 2}
	radius := 0.0
	for _, r := range b {
		for _, q := range r {
			radius = max(radius, DistanceM(c, q))
		}
	}
	return c, radius
}
