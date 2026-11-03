package plans

import (
	"testing"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

// The admin protos carry enums, not strings: every value the database
// allows maps, and anything else is an error rather than UNSPECIFIED.
func TestAdminEnumsAreStrict(t *testing.T) {
	for s, want := range map[string]isshov1.TimeBlock{
		"morning": isshov1.TimeBlock_TIME_BLOCK_MORNING, "afternoon": isshov1.TimeBlock_TIME_BLOCK_AFTERNOON, "evening": isshov1.TimeBlock_TIME_BLOCK_EVENING,
	} {
		if got, err := BlockProto(s); err != nil || got != want {
			t.Errorf("BlockProto(%q) = %v, %v", s, got, err)
		}
	}
	for s, want := range map[string]isshov1.EventStatus{
		"proposed": isshov1.EventStatus_EVENT_STATUS_PROPOSED, "scheduled": isshov1.EventStatus_EVENT_STATUS_SCHEDULED,
		"done": isshov1.EventStatus_EVENT_STATUS_DONE, "cancelled": isshov1.EventStatus_EVENT_STATUS_CANCELLED,
	} {
		if got, err := EventStatusProto(s); err != nil || got != want {
			t.Errorf("EventStatusProto(%q) = %v, %v", s, got, err)
		}
	}
	for s, want := range map[string]isshov1.GroupStatus{
		"inviting": isshov1.GroupStatus_GROUP_STATUS_INVITING, "confirmed": isshov1.GroupStatus_GROUP_STATUS_CONFIRMED, "cancelled": isshov1.GroupStatus_GROUP_STATUS_CANCELLED,
	} {
		if got, err := GroupStatusProto(s); err != nil || got != want {
			t.Errorf("GroupStatusProto(%q) = %v, %v", s, got, err)
		}
	}
	for s, want := range map[string]isshov1.InviteResponse{
		"invited": isshov1.InviteResponse_INVITE_RESPONSE_INVITED, "confirmed": isshov1.InviteResponse_INVITE_RESPONSE_CONFIRMED,
		"declined": isshov1.InviteResponse_INVITE_RESPONSE_DECLINED, "dropped": isshov1.InviteResponse_INVITE_RESPONSE_DROPPED,
	} {
		if got, err := ResponseProto(s); err != nil || got != want {
			t.Errorf("ResponseProto(%q) = %v, %v", s, got, err)
		}
	}
	for name, err := range map[string]error{
		"block":    first(BlockProto("day")),
		"event":    first(EventStatusProto("postponed")),
		"group":    first(GroupStatusProto("")),
		"response": first(ResponseProto("maybe")),
	} {
		if err == nil {
			t.Errorf("unknown %s value was accepted", name)
		}
	}
}

func first[T any](_ T, err error) error { return err }
