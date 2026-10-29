package maps

import (
	"errors"
	"math"
	"strings"
)

// Google's encoded polyline format (5 decimal places, about 1 m): compact
// enough to keep an area's boundary on one line of a city pack, and the
// format Routes already returns. See
// https://developers.google.com/maps/documentation/utilities/polylinealgorithm

// EncodePolyline encodes points as a polyline.
func EncodePolyline(points []LatLng) string {
	var b strings.Builder
	var lastLat, lastLng int64
	for _, p := range points {
		lat, lng := int64(math.Round(p.Lat*1e5)), int64(math.Round(p.Lng*1e5))
		encodeValue(&b, lat-lastLat)
		encodeValue(&b, lng-lastLng)
		lastLat, lastLng = lat, lng
	}
	return b.String()
}

func encodeValue(b *strings.Builder, v int64) {
	u := uint64(v) << 1
	if v < 0 {
		u = ^u
	}
	for u >= 0x20 {
		b.WriteByte(byte(0x20|(u&0x1f)) + 63)
		u >>= 5
	}
	b.WriteByte(byte(u) + 63)
}

// DecodePolyline decodes a polyline.
func DecodePolyline(s string) ([]LatLng, error) {
	var out []LatLng
	var lat, lng int64
	for i := 0; i < len(s); {
		var d [2]int64
		for k := range d {
			var u uint64
			for shift := uint(0); ; shift += 5 {
				if i >= len(s) || shift > 60 {
					return nil, errors.New("truncated or invalid polyline")
				}
				c := uint64(s[i]) - 63
				i++
				if c > 0x3f {
					return nil, errors.New("invalid character in polyline")
				}
				u |= (c & 0x1f) << shift
				if c < 0x20 {
					break
				}
			}
			d[k] = int64(u >> 1)
			if u&1 != 0 {
				d[k] = ^d[k]
			}
		}
		lat, lng = lat+d[0], lng+d[1]
		out = append(out, LatLng{Lat: float64(lat) / 1e5, Lng: float64(lng) / 1e5})
	}
	return out, nil
}
