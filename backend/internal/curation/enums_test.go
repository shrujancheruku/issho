package curation

import (
	"testing"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

func TestVenueStatusProtoIsStrict(t *testing.T) {
	for s, want := range map[string]isshov1.VenueStatus{
		"candidate": isshov1.VenueStatus_VENUE_STATUS_CANDIDATE, "approved": isshov1.VenueStatus_VENUE_STATUS_APPROVED, "rejected": isshov1.VenueStatus_VENUE_STATUS_REJECTED,
	} {
		if got, err := VenueStatusProto(s); err != nil || got != want {
			t.Errorf("VenueStatusProto(%q) = %v, %v", s, got, err)
		}
	}
	if _, err := VenueStatusProto("archived"); err == nil {
		t.Error("unknown status was accepted")
	}
}
