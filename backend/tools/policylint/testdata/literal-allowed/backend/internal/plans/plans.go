package plans

import (
	"time"

	"example.test/backend/internal/policy"
)

// Using the constants is the point.
func tooBig(n int) bool { return n > policy.MaxGroupSize }

func expiry(t time.Time) time.Time { return t.Add(policy.InvitationWindow) }

// A duration that is no rule's, and a string that only looks like a number.
func other(t time.Time) (time.Time, string) { return t.Add(2 * time.Hour), "6" }

// Neither a count of retries nor the group's size; says so, with a reason.
const retries = 6 //policylint:ignore retries of a send, not a group size

//policylint:ignore a page size, not a group size
const pageSize = 3
