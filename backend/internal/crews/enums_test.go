package crews

import (
	"testing"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

// Every status the database allows has an enum value; an unknown one is an
// error (the admin SPA must never get UNSPECIFIED for a real crew).
func TestStatusProtoIsStrict(t *testing.T) {
	for s, want := range map[string]isshov1.CrewStatus{
		Offered: isshov1.CrewStatus_CREW_STATUS_OFFERED, Active: isshov1.CrewStatus_CREW_STATUS_ACTIVE,
		Lapsed: isshov1.CrewStatus_CREW_STATUS_LAPSED, Closed: isshov1.CrewStatus_CREW_STATUS_CLOSED,
	} {
		if got, err := StatusProto(s); err != nil || got != want {
			t.Errorf("StatusProto(%q) = %v, %v", s, got, err)
		}
	}
	if _, err := StatusProto("paused"); err == nil {
		t.Error("unknown status was accepted")
	}
}
