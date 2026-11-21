package policy

import "time"

// A group is 3 to 6 people (docs/spec/matching.md).
const (
	MinGroupSize = 3 // docs/spec/matching.md
	MaxGroupSize = 6 // docs/spec/matching.md
)

// InvitationWindow is fixed (docs/spec/matching.md).
const InvitationWindow = 48 * time.Hour
