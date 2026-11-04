package geo

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseBoundaryRejectsBadInput(t *testing.T) {
	_, err := ParseBoundary(`{not json`)
	var se *json.SyntaxError
	if err == nil || !errors.As(err, &se) || !strings.Contains(err.Error(), "not valid GeoJSON") {
		t.Fatalf("invalid JSON: got %v, want a wrapped syntax error", err)
	}

	_, err = ParseBoundary(`{"type":"Polygon","coordinates":[]}`)
	if err == nil || !strings.Contains(err.Error(), `"Polygon", not a GeoJSON MultiPolygon`) || strings.Contains(err.Error(), "%!") {
		t.Fatalf("wrong type: got %v", err)
	}
}
