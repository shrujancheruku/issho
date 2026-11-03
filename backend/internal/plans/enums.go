package plans

import (
	"fmt"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// Strict DB-string → enum conversions for the admin protos (ReviewService,
// ModerationService). Unlike the lenient maps the app-facing handlers index
// directly, a value the enum lacks is an error (an Internal one): a new DB
// value must be added to the enum, not shown as UNSPECIFIED.

// BlockProto converts a time block ("morning", "afternoon" or "evening").
func BlockProto(s string) (isshov1.TimeBlock, error) {
	if b, ok := blocksPB[matching.Block(s)]; ok && b != isshov1.TimeBlock_TIME_BLOCK_UNSPECIFIED {
		return b, nil
	}
	return 0, fmt.Errorf("unknown time block %q", s)
}

// EventStatusProto converts an event's status.
func EventStatusProto(s string) (isshov1.EventStatus, error) {
	if v, ok := eventStatusPB[s]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("unknown event status %q", s)
}

// GroupStatusProto converts a group's status.
func GroupStatusProto(s string) (isshov1.GroupStatus, error) {
	if v, ok := groupStatusPB[s]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("unknown group status %q", s)
}

// ResponseProto converts an invitation's answer.
func ResponseProto(s string) (isshov1.InviteResponse, error) {
	if v, ok := responsePB[s]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("unknown invite response %q", s)
}
